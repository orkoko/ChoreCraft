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

	"ChoreCraft/internal/config"
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

	testRouter = setupRouter(testDbPool, &config.Config{GeminiAPIKey: "dummy_key", JWTSecret: "test-secret-key-1234567890", VAPIDPublicKey: "", VAPIDPrivateKey: "", VAPIDContact: ""})
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
	var tokenA string
	for _, c := range rrLoginA.Result().Cookies() {
		if c.Name == "token" {
			tokenA = c.Value
		}
	}

	loginReqUserA := model.LoginRequest{Username: "user_a", Password: "password123"}
	loginBodyUserA, _ := json.Marshal(loginReqUserA)
	reqLoginUserA, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginBodyUserA))
	rrLoginUserA := performRequest(t, reqLoginUserA, http.StatusOK)
	var userALogin model.LoginResponse
	json.NewDecoder(rrLoginUserA.Body).Decode(&userALogin)
	var tokenUserA string
	for _, c := range rrLoginUserA.Result().Cookies() {
		if c.Name == "token" {
			tokenUserA = c.Value
		}
	}

	// Create task
	taskA := model.CreateTaskRequest{Title: "Mow the lawn", Type: "individual", PointsReward: 100}
	taskABody, _ := json.Marshal(taskA)
	createTaskURL := fmt.Sprintf("/api/choregroups/%s/tasks", adminALogin.ChoreGroupID)
	reqCreate, _ := http.NewRequest("POST", createTaskURL, bytes.NewBuffer(taskABody))
	reqCreate.Header.Set("Authorization", "Bearer "+tokenA)
	rrCreate := performRequest(t, reqCreate, http.StatusCreated)
	var createdTask model.Task
	json.NewDecoder(rrCreate.Body).Decode(&createdTask)

	// Submit task
	submitURL := fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", userALogin.ChoreGroupID, createdTask.ID)
	reqSubmit, _ := http.NewRequest("POST", submitURL, nil)
	reqSubmit.Header.Set("Authorization", "Bearer "+tokenUserA)
	performRequest(t, reqSubmit, http.StatusCreated)

	// Approve submission
	approveURL := fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", adminALogin.ChoreGroupID, createdTask.ID)
	approvePayload := model.UpdateSubmissionRequest{Action: "approve"}
	approveBody, _ := json.Marshal(approvePayload)
	reqApprove, _ := http.NewRequest("PUT", approveURL, bytes.NewBuffer(approveBody))
	reqApprove.Header.Set("Authorization", "Bearer "+tokenA)
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
	var tokenAdmin string
	for _, c := range rrLoginAdmin.Result().Cookies() {
		if c.Name == "token" {
			tokenAdmin = c.Value
		}
	}

	loginGalReq := model.LoginRequest{Username: "gal", Password: "pw"}
	loginGalBody, _ := json.Marshal(loginGalReq)
	reqLoginGal, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginGalBody))
	rrLoginGal := performRequest(t, reqLoginGal, http.StatusOK)
	var gal model.LoginResponse
	json.NewDecoder(rrLoginGal.Body).Decode(&gal)
	var tokenGal string
	for _, c := range rrLoginGal.Result().Cookies() {
		if c.Name == "token" {
			tokenGal = c.Value
		}
	}

	loginRonReq := model.LoginRequest{Username: "ron", Password: "pw"}
	loginRonBody, _ := json.Marshal(loginRonReq)
	reqLoginRon, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginRonBody))
	rrLoginRon := performRequest(t, reqLoginRon, http.StatusOK)
	var ron model.LoginResponse
	json.NewDecoder(rrLoginRon.Body).Decode(&ron)
	var tokenRon string
	for _, c := range rrLoginRon.Result().Cookies() {
		if c.Name == "token" {
			tokenRon = c.Value
		}
	}

	// 2. Admin creates a public task and a private task assigned to Gal
	publicTask := model.CreateTaskRequest{Title: "Wash the dishes", Type: "cooperative", PointsReward: 20}
	privateTask := model.CreateTaskRequest{Title: "Clean Gal's room", Type: "individual", PointsReward: 50, AssignedToUserID: &gal.UserID}

	createURL := fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID)

	publicTaskBody, _ := json.Marshal(publicTask)
	reqCreatePublic, _ := http.NewRequest("POST", createURL, bytes.NewBuffer(publicTaskBody))
	reqCreatePublic.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqCreatePublic, http.StatusCreated)

	privateTaskBody, _ := json.Marshal(privateTask)
	reqCreatePrivate, _ := http.NewRequest("POST", createURL, bytes.NewBuffer(privateTaskBody))
	reqCreatePrivate.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqCreatePrivate, http.StatusCreated)

	// 3. Test what Admin sees
	t.Run("Admin sees all tasks", func(t *testing.T) {
		listTasksURL := fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID)
		req, _ := http.NewRequest("GET", listTasksURL, nil)
		req.Header.Set("Authorization", "Bearer "+tokenAdmin)
		rr := performRequest(t, req, http.StatusOK)
		var tasks []model.Task
		json.NewDecoder(rr.Body).Decode(&tasks)
		if len(tasks) != 2 {
			t.Errorf("Expected Admin to see 2 tasks, but got %d", len(tasks))
		}
	})

	// 4. Test what Gal sees
	t.Run("Gal sees public and personal tasks", func(t *testing.T) {
		listTasksURL := fmt.Sprintf("/api/choregroups/%s/tasks", gal.ChoreGroupID)
		req, _ := http.NewRequest("GET", listTasksURL, nil)
		req.Header.Set("Authorization", "Bearer "+tokenGal)
		rr := performRequest(t, req, http.StatusOK)
		var tasks []model.Task
		json.NewDecoder(rr.Body).Decode(&tasks)
		if len(tasks) != 2 {
			t.Errorf("Expected Gal to see 2 tasks, but got %d", len(tasks))
		}
	})

	// 5. Test what Ron sees
	t.Run("Ron sees only public tasks", func(t *testing.T) {
		listTasksURL := fmt.Sprintf("/api/choregroups/%s/tasks", ron.ChoreGroupID)
		req, _ := http.NewRequest("GET", listTasksURL, nil)
		req.Header.Set("Authorization", "Bearer "+tokenRon)
		rr := performRequest(t, req, http.StatusOK)
		var tasks []model.Task
		json.NewDecoder(rr.Body).Decode(&tasks)
		if len(tasks) != 1 {
			t.Errorf("Expected Ron to see 1 task, but got %d", len(tasks))
		}
	})
}

func TestAdminEditAndDeleteTask(t *testing.T) {
	clearAllTables(t)
	_, _ = testDbPool.Exec(context.Background(), "INSERT INTO icon_mappings (keyword, emoji) VALUES ($1, $2) ON CONFLICT (keyword) DO NOTHING", "updated", "📋")

	// 1. Setup admin and user
	signup := model.SignUpRequest{ChoreGroupName: "Edit-Delete Group", Username: "edit_admin", Password: "pw"}
	signupBody, _ := json.Marshal(signup)
	reqSignup, _ := http.NewRequest("POST", "/api/signup", bytes.NewBuffer(signupBody))
	performRequest(t, reqSignup, http.StatusCreated)

	loginAdminReq := model.LoginRequest{Username: "edit_admin", Password: "pw"}
	loginAdminBody, _ := json.Marshal(loginAdminReq)
	reqLoginAdmin, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginAdminBody))
	rrLoginAdmin := performRequest(t, reqLoginAdmin, http.StatusOK)
	var admin model.LoginResponse
	json.NewDecoder(rrLoginAdmin.Body).Decode(&admin)
	var tokenAdmin string
	for _, c := range rrLoginAdmin.Result().Cookies() {
		if c.Name == "token" {
			tokenAdmin = c.Value
		}
	}

	// 2. Admin creates a task
	task := model.CreateTaskRequest{Title: "Initial Task", Type: "individual", PointsReward: 10}
	taskBody, _ := json.Marshal(task)
	createURL := fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID)
	reqCreate, _ := http.NewRequest("POST", createURL, bytes.NewBuffer(taskBody))
	reqCreate.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrCreate := performRequest(t, reqCreate, http.StatusCreated)
	var createdTask model.Task
	json.NewDecoder(rrCreate.Body).Decode(&createdTask)

	// 3. Admin edits the task
	updateReq := model.UpdateTaskRequest{Title: "Updated Task Title", Type: "cooperative", PointsReward: 99}
	updateBody, _ := json.Marshal(updateReq)
	updateURL := fmt.Sprintf("/api/choregroups/%s/tasks/%s", admin.ChoreGroupID, createdTask.ID)
	reqUpdate, _ := http.NewRequest("PUT", updateURL, bytes.NewBuffer(updateBody))
	reqUpdate.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqUpdate, http.StatusNoContent)

	// 4. Verify the task was updated
	listTasksURL := fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID)
	reqList, _ := http.NewRequest("GET", listTasksURL, nil)
	reqList.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrList := performRequest(t, reqList, http.StatusOK)
	var tasks []model.Task
	json.NewDecoder(rrList.Body).Decode(&tasks)
	if len(tasks) != 1 || tasks[0].Title != "📋 Updated Task Title" || tasks[0].PointsReward != 99 {
		t.Fatalf("Task was not updated correctly. Found: %+v", tasks)
	}

	// 5. Admin deletes the task
	deleteURL := fmt.Sprintf("/api/choregroups/%s/tasks/%s", admin.ChoreGroupID, createdTask.ID)
	reqDelete, _ := http.NewRequest("DELETE", deleteURL, nil)
	reqDelete.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqDelete, http.StatusNoContent)

	// 6. Verify the task was deleted
	rrListAfterDelete := performRequest(t, reqList, http.StatusOK)
	var tasksAfterDelete []model.Task
	json.NewDecoder(rrListAfterDelete.Body).Decode(&tasksAfterDelete)
	if len(tasksAfterDelete) != 0 {
		t.Errorf("Expected task to be deleted, but found %d tasks", len(tasksAfterDelete))
	}
}

func TestCooperativeTasksAndStatistics(t *testing.T) {
	clearAllTables(t)

	// 1. Setup admin and two users
	signup := model.SignUpRequest{ChoreGroupName: "Co-op Group", Username: "coop_admin", Password: "pw"}
	signupBody, _ := json.Marshal(signup)
	reqSignup, _ := http.NewRequest("POST", "/api/signup", bytes.NewBuffer(signupBody))
	performRequest(t, reqSignup, http.StatusCreated)

	loginAdminReq := model.LoginRequest{Username: "coop_admin", Password: "pw"}
	loginAdminBody, _ := json.Marshal(loginAdminReq)
	reqLoginAdmin, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginAdminBody))
	rrLoginAdmin := performRequest(t, reqLoginAdmin, http.StatusOK)
	var admin model.LoginResponse
	json.NewDecoder(rrLoginAdmin.Body).Decode(&admin)
	var tokenAdmin string
	for _, c := range rrLoginAdmin.Result().Cookies() {
		if c.Name == "token" {
			tokenAdmin = c.Value
		}
	}

	addUser1Req := model.AddUserRequest{ChoreGroupName: "Co-op Group", Username: "user1", Password: "pw", UserRole: "user"}
	addUser1Body, _ := json.Marshal(addUser1Req)
	reqAddUser1, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(addUser1Body))
	performRequest(t, reqAddUser1, http.StatusCreated)

	loginUser1Req := model.LoginRequest{Username: "user1", Password: "pw"}
	loginUser1Body, _ := json.Marshal(loginUser1Req)
	reqLoginUser1, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginUser1Body))
	rrLoginUser1 := performRequest(t, reqLoginUser1, http.StatusOK)
	var user1 model.LoginResponse
	json.NewDecoder(rrLoginUser1.Body).Decode(&user1)
	var tokenUser1 string
	for _, c := range rrLoginUser1.Result().Cookies() {
		if c.Name == "token" {
			tokenUser1 = c.Value
		}
	}

	// 2. Admin creates a cooperative task
	coopTask := model.CreateTaskRequest{Title: "Clean the garage", Type: "cooperative", PointsReward: 250}
	coopTaskBody, _ := json.Marshal(coopTask)
	createURL := fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID)
	reqCreate, _ := http.NewRequest("POST", createURL, bytes.NewBuffer(coopTaskBody))
	reqCreate.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrCreate := performRequest(t, reqCreate, http.StatusCreated)
	var createdCoopTask model.Task
	json.NewDecoder(rrCreate.Body).Decode(&createdCoopTask)

	// 3. User1 submits the task
	submitURL := fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", user1.ChoreGroupID, createdCoopTask.ID)
	reqSubmit, _ := http.NewRequest("POST", submitURL, nil)
	reqSubmit.Header.Set("Authorization", "Bearer "+tokenUser1)
	performRequest(t, reqSubmit, http.StatusCreated)

	// 4. Admin approves the submission
	approveURL := fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", admin.ChoreGroupID, createdCoopTask.ID)
	approvePayload := model.UpdateSubmissionRequest{Action: "approve"}
	approveBody, _ := json.Marshal(approvePayload)
	reqApprove, _ := http.NewRequest("PUT", approveURL, bytes.NewBuffer(approveBody))
	reqApprove.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqApprove, http.StatusNoContent)

	// 5. Verify the statistics
	statsURL := fmt.Sprintf("/api/choregroups/%s/statistics", admin.ChoreGroupID)
	reqStats, _ := http.NewRequest("GET", statsURL, nil)
	reqStats.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrStats := performRequest(t, reqStats, http.StatusOK)
	var stats model.StatisticsResponse
	json.NewDecoder(rrStats.Body).Decode(&stats)

	if stats.CooperativePoints != 250 {
		t.Errorf("Expected cooperative points to be 250, but got %d", stats.CooperativePoints)
	}

	// Check that no individual points were awarded
	for _, u := range stats.Users {
		if u.Points != 0 {
			t.Errorf("Expected user %s to have 0 points, but got %d", u.Username, u.Points)
		}
	}
}

func TestSubmissionFiltering(t *testing.T) {
	clearAllTables(t)

	// 1. Setup admin and user
	signup := model.SignUpRequest{ChoreGroupName: "Filter Group", Username: "filter_admin", Password: "pw"}
	signupBody, _ := json.Marshal(signup)
	reqSignup, _ := http.NewRequest("POST", "/api/signup", bytes.NewBuffer(signupBody))
	performRequest(t, reqSignup, http.StatusCreated)

	loginAdminReq := model.LoginRequest{Username: "filter_admin", Password: "pw"}
	loginAdminBody, _ := json.Marshal(loginAdminReq)
	reqLoginAdmin, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginAdminBody))
	rrLoginAdmin := performRequest(t, reqLoginAdmin, http.StatusOK)
	var admin model.LoginResponse
	json.NewDecoder(rrLoginAdmin.Body).Decode(&admin)
	var tokenAdmin string
	for _, c := range rrLoginAdmin.Result().Cookies() {
		if c.Name == "token" {
			tokenAdmin = c.Value
		}
	}

	addUserReq := model.AddUserRequest{ChoreGroupName: "Filter Group", Username: "filter_user", Password: "pw", UserRole: "user"}
	addUserBody, _ := json.Marshal(addUserReq)
	reqAddUser, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(addUserBody))
	performRequest(t, reqAddUser, http.StatusCreated)

	loginUserReq := model.LoginRequest{Username: "filter_user", Password: "pw"}
	loginUserBody, _ := json.Marshal(loginUserReq)
	reqLoginUser, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginUserBody))
	rrLoginUser := performRequest(t, reqLoginUser, http.StatusOK)
	var user model.LoginResponse
	json.NewDecoder(rrLoginUser.Body).Decode(&user)
	var tokenUser string
	for _, c := range rrLoginUser.Result().Cookies() {
		if c.Name == "token" {
			tokenUser = c.Value
		}
	}

	// 2. Create and submit a task
	task := model.CreateTaskRequest{Title: "Filter Task", Type: "individual", PointsReward: 10}
	taskBody, _ := json.Marshal(task)
	createURL := fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID)
	reqCreate, _ := http.NewRequest("POST", createURL, bytes.NewBuffer(taskBody))
	reqCreate.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrCreate := performRequest(t, reqCreate, http.StatusCreated)
	var createdTask model.Task
	json.NewDecoder(rrCreate.Body).Decode(&createdTask)

	submitURL := fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", user.ChoreGroupID, createdTask.ID)
	reqSubmit, _ := http.NewRequest("POST", submitURL, nil)
	reqSubmit.Header.Set("Authorization", "Bearer "+tokenUser)
	performRequest(t, reqSubmit, http.StatusCreated)

	// 3. Verify default (pending)
	submissionsURL := fmt.Sprintf("/api/choregroups/%s/submissions", admin.ChoreGroupID)
	reqDefault, _ := http.NewRequest("GET", submissionsURL, nil)
	reqDefault.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrDefault := performRequest(t, reqDefault, http.StatusOK)
	var pendingSubs []model.TaskSubmission
	json.NewDecoder(rrDefault.Body).Decode(&pendingSubs)
	if len(pendingSubs) != 1 {
		t.Fatalf("Expected 1 pending submission, got %d", len(pendingSubs))
	}

	// 4. Admin rejects the submission
	rejectURL := fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", admin.ChoreGroupID, createdTask.ID)
	rejectPayload := model.UpdateSubmissionRequest{Action: "reject"}
	rejectBody, _ := json.Marshal(rejectPayload)
	reqReject, _ := http.NewRequest("PUT", rejectURL, bytes.NewBuffer(rejectBody))
	reqReject.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqReject, http.StatusNoContent)

	// 5. Verify default again (should be none)
	rrDefaultAfterReject := performRequest(t, reqDefault, http.StatusOK)
	var noPendingSubs []model.TaskSubmission
	json.NewDecoder(rrDefaultAfterReject.Body).Decode(&noPendingSubs)
	if len(noPendingSubs) != 0 {
		t.Errorf("Expected 0 pending submissions after rejection, got %d", len(noPendingSubs))
	}

	// 6. Verify can fetch rejected
	reqRejected, _ := http.NewRequest("GET", submissionsURL+"?status=rejected", nil)
	reqRejected.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrRejected := performRequest(t, reqRejected, http.StatusOK)
	var rejectedSubs []model.TaskSubmission
	json.NewDecoder(rrRejected.Body).Decode(&rejectedSubs)
	if len(rejectedSubs) != 1 {
		t.Errorf("Expected 1 rejected submission, got %d", len(rejectedSubs))
	}
}
