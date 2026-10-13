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

func TestDailyBonusAndLuckyWheel(t *testing.T) {
	clearAllTables(t)

	// 1. Setup household, admin and kid
	signup := model.SignUpRequest{ChoreGroupName: "Bonus House", Username: "bonus_admin", Password: "pw"}
	signupBody, _ := json.Marshal(signup)
	reqSignup, _ := http.NewRequest("POST", "/api/signup", bytes.NewBuffer(signupBody))
	performRequest(t, reqSignup, http.StatusCreated)

	loginAdminReq := model.LoginRequest{Username: "bonus_admin", Password: "pw"}
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

	addUserReq := model.AddUserRequest{ChoreGroupName: "Bonus House", Username: "bonus_kid", Password: "pw", UserRole: "user"}
	addUserBody, _ := json.Marshal(addUserReq)
	reqAddUser, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(addUserBody))
	performRequest(t, reqAddUser, http.StatusCreated)

	loginKidReq := model.LoginRequest{Username: "bonus_kid", Password: "pw"}
	loginKidBody, _ := json.Marshal(loginKidReq)
	reqLoginKid, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginKidBody))
	rrLoginKid := performRequest(t, reqLoginKid, http.StatusOK)
	var kid model.LoginResponse
	json.NewDecoder(rrLoginKid.Body).Decode(&kid)
	var tokenKid string
	for _, c := range rrLoginKid.Result().Cookies() {
		if c.Name == "token" {
			tokenKid = c.Value
		}
	}

	// 2. Kid login should indicate welcome spin not yet claimed
	if kid.HasClaimedWelcomeSpin {
		t.Errorf("Expected new kid to not have claimed welcome spin yet")
	}

	// 3. Check GET /daily-bonus
	dailyBonusURL := fmt.Sprintf("/api/choregroups/%s/daily-bonus", admin.ChoreGroupID)
	reqDB, _ := http.NewRequest("GET", dailyBonusURL, nil)
	reqDB.Header.Set("Authorization", "Bearer "+tokenKid)
	rrDB := performRequest(t, reqDB, http.StatusOK)
	var dbStatus model.DailyBonusStatusResponse
	json.NewDecoder(rrDB.Body).Decode(&dbStatus)

	if dbStatus.TasksCompleted != 0 {
		t.Errorf("Expected 0 tasks completed, got %d", dbStatus.TasksCompleted)
	}
	if dbStatus.SpinsAvailable != 0 {
		t.Errorf("Expected 0 spins available on login, got %d", dbStatus.SpinsAvailable)
	}
	if dbStatus.TargetWheel != 1 {
		t.Errorf("Expected TargetWheel to be 1, got %d", dbStatus.TargetWheel)
	}

	// 4. Admin tries to spin the wheel -> forbidden
	wheelSpinURL := fmt.Sprintf("/api/choregroups/%s/wheel/spin", admin.ChoreGroupID)
	reqAdminSpin, _ := http.NewRequest("POST", wheelSpinURL, nil)
	reqAdminSpin.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqAdminSpin, http.StatusForbidden)

	// 5. Kid tries to spin before completing any tasks -> 400 Bad Request (no spins available)
	reqKidSpinEarly, _ := http.NewRequest("POST", wheelSpinURL, nil)
	reqKidSpinEarly.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqKidSpinEarly, http.StatusBadRequest)

	// 6. Complete 1st task to reach milestone 1 (awards daily wheel spin)
	taskReq1 := model.CreateTaskRequest{Title: "Chore 1", Type: "individual", PointsReward: 5}
	taskBody1, _ := json.Marshal(taskReq1)
	reqCreate1, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID), bytes.NewBuffer(taskBody1))
	reqCreate1.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrCreate1 := performRequest(t, reqCreate1, http.StatusCreated)
	var tObj1 model.Task
	json.NewDecoder(rrCreate1.Body).Decode(&tObj1)

	// Submit task 1
	reqSub1, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", admin.ChoreGroupID, tObj1.ID), nil)
	reqSub1.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqSub1, http.StatusCreated)

	// Approve task 1
	approveReq1 := model.UpdateSubmissionRequest{Action: "approve"}
	approveBody1, _ := json.Marshal(approveReq1)
	reqApprove1, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", admin.ChoreGroupID, tObj1.ID), bytes.NewBuffer(approveBody1))
	reqApprove1.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqApprove1, http.StatusNoContent)

	// Check daily bonus after 1st task: 1 task completed, 1 spin available!
	reqDB1, _ := http.NewRequest("GET", dailyBonusURL, nil)
	reqDB1.Header.Set("Authorization", "Bearer "+tokenKid)
	rrDB1 := performRequest(t, reqDB1, http.StatusOK)
	var dbStatus1 model.DailyBonusStatusResponse
	json.NewDecoder(rrDB1.Body).Decode(&dbStatus1)

	if dbStatus1.TasksCompleted != 1 {
		t.Errorf("Expected 1 task completed, got %d", dbStatus1.TasksCompleted)
	}
	if dbStatus1.SpinsAvailable != 1 {
		t.Errorf("Expected 1 spin available after 1st task, got %d", dbStatus1.SpinsAvailable)
	}

	// 7. Kid spins the wheel using the earned spin
	reqKidSpin, _ := http.NewRequest("POST", wheelSpinURL, nil)
	reqKidSpin.Header.Set("Authorization", "Bearer "+tokenKid)
	rrKidSpin := performRequest(t, reqKidSpin, http.StatusOK)
	var spinRes model.WheelSpinResponse
	json.NewDecoder(rrKidSpin.Body).Decode(&spinRes)

	if spinRes.SliceIndex < 0 || spinRes.SliceIndex > 7 {
		t.Errorf("Expected slice index between 0 and 7, got %d", spinRes.SliceIndex)
	}
	if spinRes.SpinsRemaining != 0 {
		t.Errorf("Expected 0 spins remaining after spinning, got %d", spinRes.SpinsRemaining)
	}

	// 8. Kid tries to spin again -> 400 Bad Request (no spins left)
	reqKidSpin2, _ := http.NewRequest("POST", wheelSpinURL, nil)
	reqKidSpin2.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqKidSpin2, http.StatusBadRequest)

	// 9. Complete tasks 2 and 3
	for i := 2; i <= 3; i++ {
		taskReq := model.CreateTaskRequest{Title: fmt.Sprintf("Chore %d", i), Type: "individual", PointsReward: 5}
		taskBody, _ := json.Marshal(taskReq)
		reqCreate, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID), bytes.NewBuffer(taskBody))
		reqCreate.Header.Set("Authorization", "Bearer "+tokenAdmin)
		rrCreate := performRequest(t, reqCreate, http.StatusCreated)
		var tObj model.Task
		json.NewDecoder(rrCreate.Body).Decode(&tObj)

		// Submit
		reqSub, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", admin.ChoreGroupID, tObj.ID), nil)
		reqSub.Header.Set("Authorization", "Bearer "+tokenKid)
		performRequest(t, reqSub, http.StatusCreated)

		// Approve
		approveReq := model.UpdateSubmissionRequest{Action: "approve"}
		approveBody, _ := json.Marshal(approveReq)
		reqApprove, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", admin.ChoreGroupID, tObj.ID), bytes.NewBuffer(approveBody))
		reqApprove.Header.Set("Authorization", "Bearer "+tokenAdmin)
		performRequest(t, reqApprove, http.StatusNoContent)
	}

	// Check daily bonus after 3 tasks: spins available is 0 (first spin was already used, milestone 3 is not a spin milestone)
	reqDB3, _ := http.NewRequest("GET", dailyBonusURL, nil)
	reqDB3.Header.Set("Authorization", "Bearer "+tokenKid)
	rrDB3 := performRequest(t, reqDB3, http.StatusOK)
	var dbStatus3 model.DailyBonusStatusResponse
	json.NewDecoder(rrDB3.Body).Decode(&dbStatus3)

	if dbStatus3.TasksCompleted != 3 {
		t.Errorf("Expected 3 tasks completed, got %d", dbStatus3.TasksCompleted)
	}
	if dbStatus3.SpinsAvailable != 0 {
		t.Errorf("Expected 0 spins available after 3 tasks (already consumed at 1), got %d", dbStatus3.SpinsAvailable)
	}

	// 9. Complete task 4 to unlock milestone 4 (2nd Lucky Wheel spin)
	taskReq4 := model.CreateTaskRequest{Title: "Chore 4", Type: "individual", PointsReward: 5}
	taskBody4, _ := json.Marshal(taskReq4)
	reqCreate4, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID), bytes.NewBuffer(taskBody4))
	reqCreate4.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrCreate4 := performRequest(t, reqCreate4, http.StatusCreated)
	var tObj4 model.Task
	json.NewDecoder(rrCreate4.Body).Decode(&tObj4)

	// Submit & Approve task 4
	reqSub4, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", admin.ChoreGroupID, tObj4.ID), nil)
	reqSub4.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqSub4, http.StatusCreated)

	approveReq4 := model.UpdateSubmissionRequest{Action: "approve"}
	approveBody4, _ := json.Marshal(approveReq4)
	reqApprove4, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", admin.ChoreGroupID, tObj4.ID), bytes.NewBuffer(approveBody4))
	reqApprove4.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqApprove4, http.StatusNoContent)

	// Check daily bonus after 4 tasks -> milestone 4 awards a 2nd Lucky Wheel spin!
	reqDB4, _ := http.NewRequest("GET", dailyBonusURL, nil)
	reqDB4.Header.Set("Authorization", "Bearer "+tokenKid)
	rrDB4 := performRequest(t, reqDB4, http.StatusOK)
	var dbStatus4 model.DailyBonusStatusResponse
	json.NewDecoder(rrDB4.Body).Decode(&dbStatus4)

	if dbStatus4.TasksCompleted != 4 {
		t.Errorf("Expected 4 tasks completed, got %d", dbStatus4.TasksCompleted)
	}
	if dbStatus4.SpinsAvailable != 1 {
		t.Errorf("Expected 1 spin available after 4 tasks (milestone 4 spin), got %d", dbStatus4.SpinsAvailable)
	}

	// 10. Kid spins the wheel using the milestone 4 earned spin
	reqKidSpin4, _ := http.NewRequest("POST", wheelSpinURL, nil)
	reqKidSpin4.Header.Set("Authorization", "Bearer "+tokenKid)
	rrKidSpin4 := performRequest(t, reqKidSpin4, http.StatusOK)
	var spinRes4 model.WheelSpinResponse
	json.NewDecoder(rrKidSpin4.Body).Decode(&spinRes4)
	if spinRes4.SpinsRemaining != 0 {
		t.Errorf("Expected 0 spins remaining after spinning milestone 4 spin, got %d", spinRes4.SpinsRemaining)
	}

	// Complete task 5
	taskReq5 := model.CreateTaskRequest{Title: "Chore 5", Type: "individual", PointsReward: 5}
	taskBody5, _ := json.Marshal(taskReq5)
	reqCreate5, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID), bytes.NewBuffer(taskBody5))
	reqCreate5.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrCreate5 := performRequest(t, reqCreate5, http.StatusCreated)
	var tObj5 model.Task
	json.NewDecoder(rrCreate5.Body).Decode(&tObj5)

	reqSub5, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", admin.ChoreGroupID, tObj5.ID), nil)
	reqSub5.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqSub5, http.StatusCreated)

	approveReq5 := model.UpdateSubmissionRequest{Action: "approve"}
	approveBody5, _ := json.Marshal(approveReq5)
	reqApprove5, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", admin.ChoreGroupID, tObj5.ID), bytes.NewBuffer(approveBody5))
	reqApprove5.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqApprove5, http.StatusNoContent)

	// 10.5 Test that a mandatory task counts towards daily bonus
	mandReq := model.CreateTaskRequest{Title: "Clean Dining Table (Mandatory Test)", Type: "individual", IsMandatory: true}
	mandBody, _ := json.Marshal(mandReq)
	reqCreateMand, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID), bytes.NewBuffer(mandBody))
	reqCreateMand.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrCreateMand := performRequest(t, reqCreateMand, http.StatusCreated)
	var tObjMand model.Task
	json.NewDecoder(rrCreateMand.Body).Decode(&tObjMand)

	reqSubMand, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", admin.ChoreGroupID, tObjMand.ID), nil)
	reqSubMand.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqSubMand, http.StatusCreated)

	approveReqMand := model.UpdateSubmissionRequest{Action: "approve"}
	approveBodyMand, _ := json.Marshal(approveReqMand)
	reqApproveMand, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", admin.ChoreGroupID, tObjMand.ID), bytes.NewBuffer(approveBodyMand))
	reqApproveMand.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqApproveMand, http.StatusNoContent)

	// Check daily bonus after mandatory task
	reqDBMand, _ := http.NewRequest("GET", dailyBonusURL, nil)
	reqDBMand.Header.Set("Authorization", "Bearer "+tokenKid)
	rrDBMand := performRequest(t, reqDBMand, http.StatusOK)
	var dbStatusMand model.DailyBonusStatusResponse
	json.NewDecoder(rrDBMand.Body).Decode(&dbStatusMand)

	if dbStatusMand.TasksCompleted != 6 {
		t.Errorf("Expected tasks_completed to be 6 (mandatory counts for progress bar), got %d", dbStatusMand.TasksCompleted)
	}

	// 11. Create 6th chore with 10 coins reward. When approved, awards regular 10 coins.
	taskReq6 := model.CreateTaskRequest{Title: "Chore 6", Type: "individual", PointsReward: 10}
	taskBody6, _ := json.Marshal(taskReq6)
	reqCreate6, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID), bytes.NewBuffer(taskBody6))
	reqCreate6.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrCreate6 := performRequest(t, reqCreate6, http.StatusCreated)
	var tObj6 model.Task
	json.NewDecoder(rrCreate6.Body).Decode(&tObj6)

	// Submit task 6
	reqSub6, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", admin.ChoreGroupID, tObj6.ID), nil)
	reqSub6.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqSub6, http.StatusCreated)

	// Approve task 6
	approveReq6 := model.UpdateSubmissionRequest{Action: "approve"}
	approveBody6, _ := json.Marshal(approveReq6)
	reqApprove6, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", admin.ChoreGroupID, tObj6.ID), bytes.NewBuffer(approveBody6))
	reqApprove6.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqApprove6, http.StatusNoContent)

	// 12. Simulate Day 2: Daily spin is NOT available on login (tasks_completed = 0)
	_, execErr := testDbPool.Exec(context.Background(),
		"UPDATE user_daily_progress SET date = CURRENT_DATE - INTERVAL '1 day' WHERE user_id = $1",
		kid.UserID,
	)
	if execErr != nil {
		t.Fatalf("Failed to simulate previous day: %v", execErr)
	}

	reqDay2, _ := http.NewRequest("GET", dailyBonusURL, nil)
	reqDay2.Header.Set("Authorization", "Bearer "+tokenKid)
	rrDay2 := performRequest(t, reqDay2, http.StatusOK)
	var dbStatusDay2 model.DailyBonusStatusResponse
	json.NewDecoder(rrDay2.Body).Decode(&dbStatusDay2)

	if dbStatusDay2.TasksCompleted != 0 {
		t.Errorf("Expected 0 tasks completed on Day 2, got %d", dbStatusDay2.TasksCompleted)
	}
	if dbStatusDay2.SpinsAvailable != 0 {
		t.Errorf("Expected 0 spins available on login on Day 2, got %d", dbStatusDay2.SpinsAvailable)
	}
	if !dbStatusDay2.HasClaimedWelcomeSpin {
		t.Errorf("Expected HasClaimedWelcomeSpin to be true on Day 2")
	}

	// Spinning on Day 2 without completing the 1st task fails (400 Bad Request)
	reqKidSpinDay2Fail, _ := http.NewRequest("POST", wheelSpinURL, nil)
	reqKidSpinDay2Fail.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqKidSpinDay2Fail, http.StatusBadRequest)

	// Complete 1 chore on Day 2 to earn the Day 2 daily wheel spin
	taskDay2Req := model.CreateTaskRequest{Title: "Day 2 Chore 1", Type: "individual", PointsReward: 5}
	taskDay2Body, _ := json.Marshal(taskDay2Req)
	reqDay2Create, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID), bytes.NewBuffer(taskDay2Body))
	reqDay2Create.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrDay2Create := performRequest(t, reqDay2Create, http.StatusCreated)
	var taskDay2Obj model.Task
	json.NewDecoder(rrDay2Create.Body).Decode(&taskDay2Obj)

	reqDay2Sub, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", admin.ChoreGroupID, taskDay2Obj.ID), nil)
	reqDay2Sub.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqDay2Sub, http.StatusCreated)

	approveDay2Body, _ := json.Marshal(model.UpdateSubmissionRequest{Action: "approve"})
	reqDay2Approve, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", admin.ChoreGroupID, taskDay2Obj.ID), bytes.NewBuffer(approveDay2Body))
	reqDay2Approve.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqDay2Approve, http.StatusNoContent)

	// Now Day 2 has 1 spin available!
	reqDay2AfterTask, _ := http.NewRequest("GET", dailyBonusURL, nil)
	reqDay2AfterTask.Header.Set("Authorization", "Bearer "+tokenKid)
	rrDay2AfterTask := performRequest(t, reqDay2AfterTask, http.StatusOK)
	var dbStatusDay2After model.DailyBonusStatusResponse
	json.NewDecoder(rrDay2AfterTask.Body).Decode(&dbStatusDay2After)
	if dbStatusDay2After.SpinsAvailable != 1 {
		t.Errorf("Expected 1 spin available on Day 2 after completing 1st chore, got %d", dbStatusDay2After.SpinsAvailable)
	}

	// Kid can spin the Day 2 earned spin
	reqKidSpinDay2, _ := http.NewRequest("POST", wheelSpinURL, nil)
	reqKidSpinDay2.Header.Set("Authorization", "Bearer "+tokenKid)
	rrKidSpinDay2 := performRequest(t, reqKidSpinDay2, http.StatusOK)
	var spinResDay2 model.WheelSpinResponse
	json.NewDecoder(rrKidSpinDay2.Body).Decode(&spinResDay2)
	if spinResDay2.SpinsRemaining != 0 {
		t.Errorf("Expected 0 spins remaining after Day 2 spin, got %d", spinResDay2.SpinsRemaining)
	}

	// Spinning again on Day 2 without completing 4 chores fails
	reqKidSpinDay2Second, _ := http.NewRequest("POST", wheelSpinURL, nil)
	reqKidSpinDay2Second.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqKidSpinDay2Second, http.StatusBadRequest)
}

func TestUserProfileAvatar(t *testing.T) {
	// 1. Sign up admin
	adminSignup := model.SignUpRequest{ChoreGroupName: "AvatarFamily", Username: "avatar_admin", Password: "Password123!"}
	adminBody, _ := json.Marshal(adminSignup)
	reqAdminSignup, _ := http.NewRequest("POST", "/api/signup", bytes.NewBuffer(adminBody))
	performRequest(t, reqAdminSignup, http.StatusCreated)

	// Admin login
	adminLogin := model.LoginRequest{Username: "avatar_admin", Password: "Password123!"}
	adminLoginBody, _ := json.Marshal(adminLogin)
	reqAdminLogin, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(adminLoginBody))
	rrAdminLogin := performRequest(t, reqAdminLogin, http.StatusOK)
	var admin model.LoginResponse
	json.NewDecoder(rrAdminLogin.Body).Decode(&admin)
	var tokenAdmin string
	for _, c := range rrAdminLogin.Result().Cookies() {
		if c.Name == "token" {
			tokenAdmin = c.Value
		}
	}

	// 2. Add kid user
	kidAdd := model.AddUserRequest{ChoreGroupName: "AvatarFamily", Username: "avatar_kid", Password: "Password123!", UserRole: "user"}
	kidBody, _ := json.Marshal(kidAdd)
	reqKidAdd, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(kidBody))
	performRequest(t, reqKidAdd, http.StatusCreated)

	// Kid login
	kidLogin := model.LoginRequest{Username: "avatar_kid", Password: "Password123!"}
	kidLoginBody, _ := json.Marshal(kidLogin)
	reqKidLogin, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(kidLoginBody))
	rrKidLogin := performRequest(t, reqKidLogin, http.StatusOK)
	var kid model.LoginResponse
	json.NewDecoder(rrKidLogin.Body).Decode(&kid)
	var tokenKid string
	for _, c := range rrKidLogin.Result().Cookies() {
		if c.Name == "token" {
			tokenKid = c.Value
		}
	}

	// Verify initially neither has avatar
	if admin.ProfileImageURL != nil {
		t.Errorf("Expected admin avatar to be nil initially")
	}
	if kid.ProfileImageURL != nil {
		t.Errorf("Expected kid avatar to be nil initially")
	}

	// 3. Kid updates their own avatar to "🚀"
	avatarReq1 := model.UpdateAvatarRequest{AvatarURL: "🚀"}
	avatarBody1, _ := json.Marshal(avatarReq1)
	reqKidAvatar, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/members/%s/avatar", admin.ChoreGroupID, kid.UserID), bytes.NewBuffer(avatarBody1))
	reqKidAvatar.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqKidAvatar, http.StatusOK)

	// Verify via GET /members
	reqMembers, _ := http.NewRequest("GET", fmt.Sprintf("/api/choregroups/%s/members", admin.ChoreGroupID), nil)
	reqMembers.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrMembers := performRequest(t, reqMembers, http.StatusOK)
	var members []model.User
	json.NewDecoder(rrMembers.Body).Decode(&members)
	for _, m := range members {
		if m.ID == kid.UserID {
			if m.ProfileImageURL == nil || *m.ProfileImageURL != "🚀" {
				t.Errorf("Expected kid avatar to be '🚀', got %v", m.ProfileImageURL)
			}
		}
	}

	// 4. Admin updates kid's avatar to an uploaded photo URL "/uploads/kid_photo.png"
	avatarReq2 := model.UpdateAvatarRequest{AvatarURL: "/uploads/kid_photo.png"}
	avatarBody2, _ := json.Marshal(avatarReq2)
	reqAdminUpdateKid, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/members/%s/avatar", admin.ChoreGroupID, kid.UserID), bytes.NewBuffer(avatarBody2))
	reqAdminUpdateKid.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqAdminUpdateKid, http.StatusOK)

	// Verify kid has new uploaded avatar
	rrMembers2 := performRequest(t, reqMembers, http.StatusOK)
	var members2 []model.User
	json.NewDecoder(rrMembers2.Body).Decode(&members2)
	for _, m := range members2 {
		if m.ID == kid.UserID {
			if m.ProfileImageURL == nil || *m.ProfileImageURL != "/uploads/kid_photo.png" {
				t.Errorf("Expected kid avatar to be '/uploads/kid_photo.png', got %v", m.ProfileImageURL)
			}
		}
	}

	// 5. Kid attempts to update admin's avatar -> should be 403 Forbidden
	avatarReq3 := model.UpdateAvatarRequest{AvatarURL: "🤡"}
	avatarBody3, _ := json.Marshal(avatarReq3)
	reqKidHackAdmin, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/members/%s/avatar", admin.ChoreGroupID, admin.UserID), bytes.NewBuffer(avatarBody3))
	reqKidHackAdmin.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqKidHackAdmin, http.StatusForbidden)

	// 6. Admin updates own avatar to "🦁"
	avatarReq4 := model.UpdateAvatarRequest{AvatarURL: "🦁"}
	avatarBody4, _ := json.Marshal(avatarReq4)
	reqAdminSelf, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/members/%s/avatar", admin.ChoreGroupID, admin.UserID), bytes.NewBuffer(avatarBody4))
	reqAdminSelf.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqAdminSelf, http.StatusOK)

	// 7. Verify login returns updated ProfileImageURL
	reqAdminLogin2, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(adminLoginBody))
	rrAdminLogin2 := performRequest(t, reqAdminLogin2, http.StatusOK)
	var admin2 model.LoginResponse
	json.NewDecoder(rrAdminLogin2.Body).Decode(&admin2)
	if admin2.ProfileImageURL == nil || *admin2.ProfileImageURL != "🦁" {
		t.Errorf("Expected login response to include updated profile image URL '🦁', got %v", admin2.ProfileImageURL)
	}
}

func TestActivityHistoryUserIsolation(t *testing.T) {
	// 1. Sign up admin
	adminSignup := model.SignUpRequest{ChoreGroupName: "HistoryFamily", Username: "hist_admin", Password: "Password123!"}
	adminBody, _ := json.Marshal(adminSignup)
	reqAdminSignup, _ := http.NewRequest("POST", "/api/signup", bytes.NewBuffer(adminBody))
	performRequest(t, reqAdminSignup, http.StatusCreated)

	adminLogin := model.LoginRequest{Username: "hist_admin", Password: "Password123!"}
	adminLoginBody, _ := json.Marshal(adminLogin)
	reqAdminLogin, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(adminLoginBody))
	rrAdminLogin := performRequest(t, reqAdminLogin, http.StatusOK)
	var admin model.LoginResponse
	json.NewDecoder(rrAdminLogin.Body).Decode(&admin)
	var tokenAdmin string
	for _, c := range rrAdminLogin.Result().Cookies() {
		if c.Name == "token" {
			tokenAdmin = c.Value
		}
	}

	// 2. Add Kid A and Kid B
	addKidAReq := model.AddUserRequest{ChoreGroupName: "HistoryFamily", Username: "kid_a", Password: "pw", UserRole: "user"}
	addKidABody, _ := json.Marshal(addKidAReq)
	reqAddKidA, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(addKidABody))
	performRequest(t, reqAddKidA, http.StatusCreated)

	addKidBReq := model.AddUserRequest{ChoreGroupName: "HistoryFamily", Username: "kid_b", Password: "pw", UserRole: "user"}
	addKidBBody, _ := json.Marshal(addKidBReq)
	reqAddKidB, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(addKidBBody))
	performRequest(t, reqAddKidB, http.StatusCreated)

	// Login Kid A
	loginKidABody, _ := json.Marshal(model.LoginRequest{Username: "kid_a", Password: "pw"})
	reqLoginKidA, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginKidABody))
	rrLoginKidA := performRequest(t, reqLoginKidA, http.StatusOK)
	var kidA model.LoginResponse
	json.NewDecoder(rrLoginKidA.Body).Decode(&kidA)
	var tokenKidA string
	for _, c := range rrLoginKidA.Result().Cookies() {
		if c.Name == "token" {
			tokenKidA = c.Value
		}
	}

	// Login Kid B
	loginKidBBody, _ := json.Marshal(model.LoginRequest{Username: "kid_b", Password: "pw"})
	reqLoginKidB, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginKidBBody))
	rrLoginKidB := performRequest(t, reqLoginKidB, http.StatusOK)
	var kidB model.LoginResponse
	json.NewDecoder(rrLoginKidB.Body).Decode(&kidB)
	var tokenKidB string
	for _, c := range rrLoginKidB.Result().Cookies() {
		if c.Name == "token" {
			tokenKidB = c.Value
		}
	}

	// 3. Kid A completes a task
	taskReqA := model.CreateTaskRequest{Title: "Kid A Chore", Type: "individual", PointsReward: 5}
	taskBodyA, _ := json.Marshal(taskReqA)
	reqCreateA, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID), bytes.NewBuffer(taskBodyA))
	reqCreateA.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrCreateA := performRequest(t, reqCreateA, http.StatusCreated)
	var taskA model.Task
	json.NewDecoder(rrCreateA.Body).Decode(&taskA)

	reqSubA, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", admin.ChoreGroupID, taskA.ID), nil)
	reqSubA.Header.Set("Authorization", "Bearer "+tokenKidA)
	performRequest(t, reqSubA, http.StatusCreated)

	approveBody, _ := json.Marshal(model.UpdateSubmissionRequest{Action: "approve"})
	reqApproveA, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", admin.ChoreGroupID, taskA.ID), bytes.NewBuffer(approveBody))
	reqApproveA.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqApproveA, http.StatusNoContent)

	// 4. Kid B completes a task
	taskReqB := model.CreateTaskRequest{Title: "Kid B Chore", Type: "individual", PointsReward: 5}
	taskBodyB, _ := json.Marshal(taskReqB)
	reqCreateB, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID), bytes.NewBuffer(taskBodyB))
	reqCreateB.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrCreateB := performRequest(t, reqCreateB, http.StatusCreated)
	var taskB model.Task
	json.NewDecoder(rrCreateB.Body).Decode(&taskB)

	reqSubB, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", admin.ChoreGroupID, taskB.ID), nil)
	reqSubB.Header.Set("Authorization", "Bearer "+tokenKidB)
	performRequest(t, reqSubB, http.StatusCreated)

	reqApproveB, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", admin.ChoreGroupID, taskB.ID), bytes.NewBuffer(approveBody))
	reqApproveB.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqApproveB, http.StatusNoContent)

	// 5. Admin deletes a task
	taskReqDel := model.CreateTaskRequest{Title: "Admin Deleted Chore", Type: "individual", PointsReward: 2}
	taskBodyDel, _ := json.Marshal(taskReqDel)
	reqCreateDel, _ := http.NewRequest("POST", fmt.Sprintf("/api/choregroups/%s/tasks", admin.ChoreGroupID), bytes.NewBuffer(taskBodyDel))
	reqCreateDel.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrCreateDel := performRequest(t, reqCreateDel, http.StatusCreated)
	var taskDel model.Task
	json.NewDecoder(rrCreateDel.Body).Decode(&taskDel)

	reqDel, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/choregroups/%s/tasks/%s", admin.ChoreGroupID, taskDel.ID), nil)
	reqDel.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqDel, http.StatusNoContent)

	// 6. Verify Kid A's GET /history/recent sees ONLY Kid A's actions
	reqHistRecentA, _ := http.NewRequest("GET", fmt.Sprintf("/api/choregroups/%s/history/recent", admin.ChoreGroupID), nil)
	reqHistRecentA.Header.Set("Authorization", "Bearer "+tokenKidA)
	rrHistRecentA := performRequest(t, reqHistRecentA, http.StatusOK)
	var eventsRecentA []model.ActivityEvent
	json.NewDecoder(rrHistRecentA.Body).Decode(&eventsRecentA)

	if len(eventsRecentA) != 1 {
		t.Fatalf("Expected Kid A to see exactly 1 recent event, got %d", len(eventsRecentA))
	}
	if eventsRecentA[0].UserUsername != "kid_a" {
		t.Errorf("Expected Kid A's recent event to be by 'kid_a', got '%s'", eventsRecentA[0].UserUsername)
	}

	// 7. Verify Kid A's GET /history (Show more..) sees ONLY Kid A's actions
	reqHistFullA, _ := http.NewRequest("GET", fmt.Sprintf("/api/choregroups/%s/history", admin.ChoreGroupID), nil)
	reqHistFullA.Header.Set("Authorization", "Bearer "+tokenKidA)
	rrHistFullA := performRequest(t, reqHistFullA, http.StatusOK)
	var histFullA model.HistoryResponse
	json.NewDecoder(rrHistFullA.Body).Decode(&histFullA)

	if histFullA.TotalCount != 1 {
		t.Fatalf("Expected Kid A to have total count 1 in full history, got %d", histFullA.TotalCount)
	}
	for _, e := range histFullA.Events {
		if e.UserUsername != "kid_a" {
			t.Errorf("Expected Kid A's full history to contain only 'kid_a' events, found '%s'", e.UserUsername)
		}
	}

	// 8. Verify Kid B's GET /history/recent sees ONLY Kid B's actions
	reqHistRecentB, _ := http.NewRequest("GET", fmt.Sprintf("/api/choregroups/%s/history/recent", admin.ChoreGroupID), nil)
	reqHistRecentB.Header.Set("Authorization", "Bearer "+tokenKidB)
	rrHistRecentB := performRequest(t, reqHistRecentB, http.StatusOK)
	var eventsRecentB []model.ActivityEvent
	json.NewDecoder(rrHistRecentB.Body).Decode(&eventsRecentB)

	if len(eventsRecentB) != 1 {
		t.Fatalf("Expected Kid B to see exactly 1 recent event, got %d", len(eventsRecentB))
	}
	if eventsRecentB[0].UserUsername != "kid_b" {
		t.Errorf("Expected Kid B's recent event to be by 'kid_b', got '%s'", eventsRecentB[0].UserUsername)
	}

	// 9. Verify Kid B's GET /history (Show more..) sees ONLY Kid B's actions
	reqHistFullB, _ := http.NewRequest("GET", fmt.Sprintf("/api/choregroups/%s/history", admin.ChoreGroupID), nil)
	reqHistFullB.Header.Set("Authorization", "Bearer "+tokenKidB)
	rrHistFullB := performRequest(t, reqHistFullB, http.StatusOK)
	var histFullB model.HistoryResponse
	json.NewDecoder(rrHistFullB.Body).Decode(&histFullB)

	if histFullB.TotalCount != 1 {
		t.Fatalf("Expected Kid B to have total count 1 in full history, got %d", histFullB.TotalCount)
	}
	for _, e := range histFullB.Events {
		if e.UserUsername != "kid_b" {
			t.Errorf("Expected Kid B's full history to contain only 'kid_b' events, found '%s'", e.UserUsername)
		}
	}

	// 10. Verify Admin's GET /history/recent sees all household events (3: Kid A, Kid B, Admin task deleted)
	reqHistRecentAdmin, _ := http.NewRequest("GET", fmt.Sprintf("/api/choregroups/%s/history/recent", admin.ChoreGroupID), nil)
	reqHistRecentAdmin.Header.Set("Authorization", "Bearer "+tokenAdmin)
	rrHistRecentAdmin := performRequest(t, reqHistRecentAdmin, http.StatusOK)
	var eventsRecentAdmin []model.ActivityEvent
	json.NewDecoder(rrHistRecentAdmin.Body).Decode(&eventsRecentAdmin)

	if len(eventsRecentAdmin) != 3 {
		t.Fatalf("Expected Admin to see 3 recent events, got %d", len(eventsRecentAdmin))
	}
}

func TestAdminCosmeticAccess(t *testing.T) {
	clearAllTables(t)

	// 1. Register Admin
	signupAdmin := model.SignUpRequest{ChoreGroupName: "Cosmetic Family", Username: "cosmetic_admin", Password: "password123"}
	signupBody, _ := json.Marshal(signupAdmin)
	reqAdminSignup, _ := http.NewRequest("POST", "/api/signup", bytes.NewBuffer(signupBody))
	performRequest(t, reqAdminSignup, http.StatusCreated)

	// Login Admin
	loginReqAdmin := model.LoginRequest{Username: "cosmetic_admin", Password: "password123"}
	loginBodyAdmin, _ := json.Marshal(loginReqAdmin)
	reqLoginAdmin, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginBodyAdmin))
	rrLoginAdmin := performRequest(t, reqLoginAdmin, http.StatusOK)
	var adminLogin model.LoginResponse
	json.NewDecoder(rrLoginAdmin.Body).Decode(&adminLogin)

	var tokenAdmin string
	for _, c := range rrLoginAdmin.Result().Cookies() {
		if c.Name == "token" {
			tokenAdmin = c.Value
		}
	}

	// Verify Admin has all 8 avatar frames and 8 backgrounds
	if len(adminLogin.UnlockedAvatarFrames) != 8 {
		t.Fatalf("Expected admin to have 8 unlocked avatar frames, got %d", len(adminLogin.UnlockedAvatarFrames))
	}
	if len(adminLogin.UnlockedBackgrounds) != 8 {
		t.Fatalf("Expected admin to have 8 unlocked backgrounds, got %d", len(adminLogin.UnlockedBackgrounds))
	}

	// 2. Admin equips surprise avatar frame (royal_crown)
	frameReq := model.UpdateAvatarFrameRequest{AvatarFrame: "royal_crown"}
	frameBody, _ := json.Marshal(frameReq)
	reqUpdateFrame, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/members/%s/avatar-frame", adminLogin.ChoreGroupID, adminLogin.UserID), bytes.NewBuffer(frameBody))
	reqUpdateFrame.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqUpdateFrame, http.StatusOK)

	// Admin equips surprise points background (fire_dragon)
	bgReq := model.UpdatePointsBackgroundRequest{Background: "fire_dragon"}
	bgBody, _ := json.Marshal(bgReq)
	reqUpdateBg, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/members/%s/points-background", adminLogin.ChoreGroupID, adminLogin.UserID), bytes.NewBuffer(bgBody))
	reqUpdateBg.Header.Set("Authorization", "Bearer "+tokenAdmin)
	performRequest(t, reqUpdateBg, http.StatusOK)

	// 3. Register Kid (role: user)
	addUserReq := model.AddUserRequest{ChoreGroupName: "Cosmetic Family", Username: "cosmetic_kid", Password: "password123", UserRole: "user"}
	addUserBody, _ := json.Marshal(addUserReq)
	reqAddKid, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(addUserBody))
	performRequest(t, reqAddKid, http.StatusCreated)

	// Login Kid
	loginReqKid := model.LoginRequest{Username: "cosmetic_kid", Password: "password123"}
	loginBodyKid, _ := json.Marshal(loginReqKid)
	reqLoginKid, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(loginBodyKid))
	rrLoginKid := performRequest(t, reqLoginKid, http.StatusOK)
	var kidLogin model.LoginResponse
	json.NewDecoder(rrLoginKid.Body).Decode(&kidLogin)

	var tokenKid string
	for _, c := range rrLoginKid.Result().Cookies() {
		if c.Name == "token" {
			tokenKid = c.Value
		}
	}

	// Verify Kid has default cosmetics
	if len(kidLogin.UnlockedAvatarFrames) != 1 || kidLogin.UnlockedAvatarFrames[0] != "none" {
		t.Fatalf("Expected kid to only have 'none' frame, got %v", kidLogin.UnlockedAvatarFrames)
	}
	if len(kidLogin.UnlockedBackgrounds) != 3 {
		t.Fatalf("Expected kid to only have 3 default backgrounds, got %d", len(kidLogin.UnlockedBackgrounds))
	}

	// Kid attempts to equip surprise frame without unlocking it -> should fail
	reqKidFrame, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/members/%s/avatar-frame", kidLogin.ChoreGroupID, kidLogin.UserID), bytes.NewBuffer(frameBody))
	reqKidFrame.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqKidFrame, http.StatusBadRequest)

	// Kid attempts to equip surprise background without unlocking it -> should fail
	reqKidBg, _ := http.NewRequest("PUT", fmt.Sprintf("/api/choregroups/%s/members/%s/points-background", kidLogin.ChoreGroupID, kidLogin.UserID), bytes.NewBuffer(bgBody))
	reqKidBg.Header.Set("Authorization", "Bearer "+tokenKid)
	performRequest(t, reqKidBg, http.StatusBadRequest)
}


