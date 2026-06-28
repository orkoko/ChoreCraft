package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"ChoreCraft/internal/model"
)

var testRouter *chi.Mux
var testDbPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("docker.io/postgres:16-alpine"),
		postgres.WithDatabase("chorecraft_test"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("could not start postgres container: %v", err)
	}
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("could not get connection string: %v", err)
	}

	testDbPool, err = pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatalf("could not connect to test database: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := executeSchema(testDbPool, logger); err != nil {
		log.Fatalf("could not execute schema for test database: %v", err)
	}

	testRouter = setupRouter(testDbPool)
	exitCode := m.Run()
	os.Exit(exitCode)
}

func performRequest(t *testing.T, req *http.Request, expectedStatus int) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)
	if rr.Code != expectedStatus {
		t.Fatalf("Expected status %d, got %d. Body: %s", expectedStatus, rr.Code, rr.Body.String())
	}
	return rr
}

func clearAllTables(t *testing.T) {
	t.Helper()
	if _, err := testDbPool.Exec(context.Background(), `TRUNCATE TABLE choregroups RESTART IDENTITY CASCADE;`); err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}
}

func TestAuthAndFullFlow(t *testing.T) {
	clearAllTables(t)

	// Setup users and choregroup
	signupA := model.SignUpRequest{ChoreGroupName: "Group A", Username: "admin_a", Password: "password123"}
	signupABody, _ := json.Marshal(signupA)
	reqA, _ := http.NewRequest("POST", "/api/signup", bytes.NewBuffer(signupABody))
	performRequest(t, reqA, http.StatusCreated)

	addUserReq := model.AddUserRequest{ChoreGroupName: "Group A", Username: "user_a", Password: "password123", UserRole: "user"}
	addUserBody, _ := json.Marshal(addUserReq)
	reqAddUser, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(addUserBody))
	performRequest(t, reqAddUser, http.StatusCreated)

	// Login
	loginReqA := model.LoginRequest{Username: "admin_a", Password: "password123"}
	loginBodyA, _ := json.Marshal(loginReqA)
	reqLoginA, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginBodyA))
	rrLoginA := performRequest(t, reqLoginA, http.StatusOK)
	var adminALogin model.LoginResponse
	json.NewDecoder(rrLoginA.Body).Decode(&adminALogin)

	loginReqUserA := model.LoginRequest{Username: "user_a", Password: "password123"}
	loginBodyUserA, _ := json.Marshal(loginReqUserA)
	reqLoginUserA, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginBodyUserA))
	rrLoginUserA := performRequest(t, reqLoginUserA, http.StatusOK)
	var userALogin model.LoginResponse
	json.NewDecoder(rrLoginUserA.Body).Decode(&userALogin)

	// Create task
	taskA := model.CreateTaskRequest{Title: "Mow the lawn", Type: "individual", PointsReward: 100}
	taskABody, _ := json.Marshal(taskA)
	createTaskURL := fmt.Sprintf("/api/choregroups/%s/tasks", adminALogin.ChoreGroupID)
	reqCreate, _ := http.NewRequest("POST", createTaskURL, bytes.NewBuffer(taskABody))
	reqCreate.Header.Set("X-User-ID", adminALogin.UserID.String())
	rrCreate := performRequest(t, reqCreate, http.StatusCreated)
	var createdTask model.Task
	json.NewDecoder(rrCreate.Body).Decode(&createdTask)

	// Submit task
	submitURL := fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", userALogin.ChoreGroupID, createdTask.ID)
	reqSubmit, _ := http.NewRequest("POST", submitURL, nil)
	reqSubmit.Header.Set("X-User-ID", userALogin.UserID.String())
	performRequest(t, reqSubmit, http.StatusCreated)

	// Approve submission
	approveURL := fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", adminALogin.ChoreGroupID, createdTask.ID)
	approvePayload := model.UpdateSubmissionRequest{Action: "approve"}
	approveBody, _ := json.Marshal(approvePayload)
	reqApprove, _ := http.NewRequest("PUT", approveURL, bytes.NewBuffer(approveBody))
	reqApprove.Header.Set("X-User-ID", adminALogin.UserID.String())
	performRequest(t, reqApprove, http.StatusNoContent)
}

func TestAssignedTaskVisibility(t *testing.T) {
	clearAllTables(t)

	// 1. Create choregroup, admin, and two users
	signup := model.SignUpRequest{ChoreGroupName: "The Assignments", Username: "assign_admin", Password: "pw"}
	signupBody, _ := json.Marshal(signup)
	reqSignup, _ := http.NewRequest("POST", "/api/signup", bytes.NewBuffer(signupBody))
	performRequest(t, reqSignup, http.StatusCreated)

	addUserGalReq := model.AddUserRequest{ChoreGroupName: "The Assignments", Username: "gal", Password: "pw", UserRole: "user"}
	addUserBodyGal, _ := json.Marshal(addUserGalReq)
	reqAddGal, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(addUserBodyGal))
	performRequest(t, reqAddGal, http.StatusCreated)

	addUserRonReq := model.AddUserRequest{ChoreGroupName: "The Assignments", Username: "ron", Password: "pw", UserRole: "user"}
	addUserBodyRon, _ := json.Marshal(addUserRonReq)
	reqAddRon, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(addUserBodyRon))
	performRequest(t, reqAddRon, http.StatusCreated)

	// Login all users
	loginAdminReq := model.LoginRequest{Username: "assign_admin", Password: "pw"}
	loginAdminBody, _ := json.Marshal(loginAdminReq)
	reqLoginAdmin, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginAdminBody))
	rrLoginAdmin := performRequest(t, reqLoginAdmin, http.StatusOK)
	var admin model.LoginResponse
	json.NewDecoder(rrLoginAdmin.Body).Decode(&admin)

	loginGalReq := model.LoginRequest{Username: "gal", Password: "pw"}
	loginGalBody, _ := json.Marshal(loginGalReq)
	reqLoginGal, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginGalBody))
	rrLoginGal := performRequest(t, reqLoginGal, http.StatusOK)
	var gal model.LoginResponse
	json.NewDecoder(rrLoginGal.Body).Decode(&gal)

	loginRonReq := model.LoginRequest{Username: "ron", Password: "pw"}
	loginRonBody, _ := json.Marshal(loginRonReq)
	reqLoginRon, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginRonBody))
	rrLoginRon := performRequest(t, reqLoginRon, http.StatusOK)
	var ron model.LoginResponse
	json.NewDecoder(rrLoginRon.Body).Decode(&ron)

	// 2. Admin creates a public task and a private task assigned to Gal
	publicTask := model.CreateTaskRequest{Title: "Wash the dishes", Type: "cooperative", PointsReward: 20}
	privateTask := model.CreateTaskRequest{Title: "Clean Gal's room", Type: "individual", PointsReward: 50, AssignedToUserID: &gal.UserID}

	createURL := fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID)

	publicTaskBody, _ := json.Marshal(publicTask)
	reqCreatePublic, _ := http.NewRequest("POST", createURL, bytes.NewBuffer(publicTaskBody))
	reqCreatePublic.Header.Set("X-User-ID", admin.UserID.String())
	performRequest(t, reqCreatePublic, http.StatusCreated)

	privateTaskBody, _ := json.Marshal(privateTask)
	reqCreatePrivate, _ := http.NewRequest("POST", createURL, bytes.NewBuffer(privateTaskBody))
	reqCreatePrivate.Header.Set("X-User-ID", admin.UserID.String())
	performRequest(t, reqCreatePrivate, http.StatusCreated)

	// 3. Test what Gal sees
	t.Run("Gal sees public and personal tasks", func(t *testing.T) {
		listTasksURL := fmt.Sprintf("/api/choregroups/%s/tasks", gal.ChoreGroupID)
		req, _ := http.NewRequest("GET", listTasksURL, nil)
		req.Header.Set("X-User-ID", gal.UserID.String())
		rr := performRequest(t, req, http.StatusOK)
		var tasks []model.Task
		json.NewDecoder(rr.Body).Decode(&tasks)
		if len(tasks) != 2 {
			t.Errorf("Expected Gal to see 2 tasks, but got %d", len(tasks))
		}
	})

	// 4. Test what Ron sees
	t.Run("Ron sees only public tasks", func(t *testing.T) {
		listTasksURL := fmt.Sprintf("/api/choregroups/%s/tasks", ron.ChoreGroupID)
		req, _ := http.NewRequest("GET", listTasksURL, nil)
		req.Header.Set("X-User-ID", ron.UserID.String())
		rr := performRequest(t, req, http.StatusOK)
		var tasks []model.Task
		json.NewDecoder(rr.Body).Decode(&tasks)
		if len(tasks) != 1 {
			t.Errorf("Expected Ron to see 1 task, but got %d", len(tasks))
		}
	})
}
