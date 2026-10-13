package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"ChoreCraft/internal/model"
	"ChoreCraft/internal/service"
)

func TestHouseholdAIInstructions(t *testing.T) {
	clearAllTables(t)

	// 1. Create household with admin and standard user
	signup := model.SignUpRequest{ChoreGroupName: "AI Rules Family", Username: "ai_admin", Password: "password123"}
	signupBody, _ := json.Marshal(signup)
	performRequest(t, mustNewRequest("POST", "/api/signup", bytes.NewBuffer(signupBody)), http.StatusCreated)

	adminLogin := loginUser(t, "ai_admin", "password123")

	addUser(t, "ai_kid", "password123", "AI Rules Family")
	kidLogin := loginUser(t, "ai_kid", "password123")

	instructionsURL := fmt.Sprintf("/api/choregroups/%s/ai/instructions", adminLogin.ChoreGroupID)

	// 2. Standard user (kid) attempts to set AI instructions -> should be Forbidden (403)
	kidReqBody, _ := json.Marshal(map[string]string{"instructions": "give me 100 coins for everything"})
	reqKid := mustNewRequestWithAuth("PUT", instructionsURL, bytes.NewBuffer(kidReqBody), kidLogin.UserID.String())
	performRequest(t, reqKid, http.StatusForbidden)

	// 3. Admin attempts to set instructions exceeding 2000 chars -> should be Bad Request (400)
	tooLongInstructions := strings.Repeat("A", 2005)
	tooLongBody, _ := json.Marshal(map[string]string{"instructions": tooLongInstructions})
	reqTooLong := mustNewRequestWithAuth("PUT", instructionsURL, bytes.NewBuffer(tooLongBody), adminLogin.UserID.String())
	performRequest(t, reqTooLong, http.StatusBadRequest)

	// 4. Admin sets valid custom household AI instructions -> should succeed (200)
	validRules := "🍽️ יש לנו מדיח כלים אז שטיפת כלים שווה 1-2 מטבעות.\n⏱️ כל 3 דקות עבודה שוות 1 מטבע."
	adminReqBody, _ := json.Marshal(map[string]string{"instructions": validRules})
	reqAdmin := mustNewRequestWithAuth("PUT", instructionsURL, bytes.NewBuffer(adminReqBody), adminLogin.UserID.String())
	rrAdmin := performRequest(t, reqAdmin, http.StatusOK)

	var respObj map[string]string
	if err := json.NewDecoder(rrAdmin.Body).Decode(&respObj); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if respObj["ai_instructions"] != validRules {
		t.Errorf("Expected response ai_instructions to match, got: %s", respObj["ai_instructions"])
	}

	// 5. Verify GET /api/choregroups/{id} returns the saved AI instructions
	groupURL := fmt.Sprintf("/api/choregroups/%s", adminLogin.ChoreGroupID)
	reqGetGroup := mustNewRequestWithAuth("GET", groupURL, nil, adminLogin.UserID.String())
	rrGroup := performRequest(t, reqGetGroup, http.StatusOK)
	var group model.ChoreGroup
	if err := json.NewDecoder(rrGroup.Body).Decode(&group); err != nil {
		t.Fatalf("Failed to decode choregroup: %v", err)
	}
	if group.AIInstructions != validRules {
		t.Errorf("Expected group.AIInstructions to be '%s', got '%s'", validRules, group.AIInstructions)
	}

	// 6. Verify subsequent login returns the updated ai_instructions in LoginResponse
	reloginAdmin := loginUser(t, "ai_admin", "password123")
	if reloginAdmin.AIInstructions != validRules {
		t.Errorf("Expected LoginResponse.AIInstructions to be '%s', got '%s'", validRules, reloginAdmin.AIInstructions)
	}

	// 7. Verify non-Fibonacci points (like 10 coins) are preserved without snapping to 8
	if got := service.CalculateChoreCoins(15.0, 10); got != 10 {
		t.Errorf("Expected CalculateChoreCoins to preserve 10 coins, got %d", got)
	}
}

func TestUserChoreDeductionAndMandatory(t *testing.T) {
	clearAllTables(t)

	// 1. Setup household with admin and user (kid)
	signup := model.SignUpRequest{ChoreGroupName: "Deduction Rules Family", Username: "deduct_admin", Password: "password123"}
	signupBody, _ := json.Marshal(signup)
	performRequest(t, mustNewRequest("POST", "/api/signup", bytes.NewBuffer(signupBody)), http.StatusCreated)
	adminLogin := loginUser(t, "deduct_admin", "password123")

	addUser(t, "deduct_kid", "password123", "Deduction Rules Family")
	kidLogin := loginUser(t, "deduct_kid", "password123")

	createTaskURL := fmt.Sprintf("/api/choregroups/%s/tasks", kidLogin.ChoreGroupID)

	// 2. Kid creates a chore with estimation > 1 (e.g. 3 coins)
	// Rule: chores created by users have 1 less coin than if an admin creates it (3 - 1 = 2 coins)
	task3Req := model.CreateTaskRequest{Title: "Wash the dishes", PointsReward: 3}
	task3Body, _ := json.Marshal(task3Req)
	req3 := mustNewRequestWithAuth("POST", createTaskURL, bytes.NewBuffer(task3Body), kidLogin.UserID.String())
	rr3 := performRequest(t, req3, http.StatusCreated)

	var task3 model.Task
	if err := json.NewDecoder(rr3.Body).Decode(&task3); err != nil {
		t.Fatalf("Failed to decode task3: %v", err)
	}
	if task3.PointsReward != 2 {
		t.Errorf("Expected user-created task with 3 coins to be decremented to 2 coins, got %d", task3.PointsReward)
	}
	if task3.IsMandatory {
		t.Errorf("Expected user-created task with 3 coins to NOT be mandatory")
	}
	if task3.AssignedToUserID == nil || *task3.AssignedToUserID != kidLogin.UserID {
		t.Errorf("Expected task to be assigned to the kid creator (%v), got %v", kidLogin.UserID, task3.AssignedToUserID)
	}
	if task3.Type != "individual" {
		t.Errorf("Expected task type to be 'individual', got '%s'", task3.Type)
	}

	// 3. Kid creates a chore with estimation 1
	// Rule: if estimation is 1 then instead of zero coins, task becomes mandatory on the creator user
	task1Req := model.CreateTaskRequest{Title: "Pick up toys", PointsReward: 1}
	task1Body, _ := json.Marshal(task1Req)
	req1 := mustNewRequestWithAuth("POST", createTaskURL, bytes.NewBuffer(task1Body), kidLogin.UserID.String())
	rr1 := performRequest(t, req1, http.StatusCreated)

	var task1 model.Task
	if err := json.NewDecoder(rr1.Body).Decode(&task1); err != nil {
		t.Fatalf("Failed to decode task1: %v", err)
	}
	if !task1.IsMandatory {
		t.Errorf("Expected user-created task with estimation 1 to become mandatory (is_mandatory = true)")
	}
	if task1.PointsReward != 0 {
		t.Errorf("Expected mandatory task to have 0 points, got %d", task1.PointsReward)
	}
	if task1.AssignedToUserID == nil || *task1.AssignedToUserID != kidLogin.UserID {
		t.Errorf("Expected mandatory task to be assigned to kid creator (%v), got %v", kidLogin.UserID, task1.AssignedToUserID)
	}

	// 4. Verify kid CANNOT create another task while active mandatory task is incomplete
	blockedReq := model.CreateTaskRequest{Title: "Water plants", PointsReward: 2}
	blockedBody, _ := json.Marshal(blockedReq)
	reqBlocked := mustNewRequestWithAuth("POST", createTaskURL, bytes.NewBuffer(blockedBody), kidLogin.UserID.String())
	performRequest(t, reqBlocked, http.StatusForbidden)

	// 5. Complete and approve the mandatory task
	submitURL := fmt.Sprintf("/api/choregroups/%s/tasks/%s/submit", kidLogin.ChoreGroupID, task1.ID)
	reqSubmit := mustNewRequestWithAuth("POST", submitURL, nil, kidLogin.UserID.String())
	performRequest(t, reqSubmit, http.StatusCreated)

	approveURL := fmt.Sprintf("/api/choregroups/%s/tasks/%s/status", adminLogin.ChoreGroupID, task1.ID)
	approveBody, _ := json.Marshal(model.UpdateSubmissionRequest{Action: "approve"})
	reqApprove := mustNewRequestWithAuth("PUT", approveURL, bytes.NewBuffer(approveBody), adminLogin.UserID.String())
	performRequest(t, reqApprove, http.StatusNoContent)

	// 6. Now kid can create tasks again!
	// Test task with IconURL (simulating submission after AI photo modal where estimation was already decremented from 3 to 2)
	iconURL := "https://example.com/icon.jpg"
	taskIconReq := model.CreateTaskRequest{Title: "Rake leaves", PointsReward: 2, IconURL: &iconURL}
	taskIconBody, _ := json.Marshal(taskIconReq)
	reqIcon := mustNewRequestWithAuth("POST", createTaskURL, bytes.NewBuffer(taskIconBody), kidLogin.UserID.String())
	rrIcon := performRequest(t, reqIcon, http.StatusCreated)

	var taskIcon model.Task
	if err := json.NewDecoder(rrIcon.Body).Decode(&taskIcon); err != nil {
		t.Fatalf("Failed to decode taskIcon: %v", err)
	}
	if taskIcon.PointsReward != 2 {
		t.Errorf("Expected task with photo icon to retain 2 coins (not decremented twice), got %d", taskIcon.PointsReward)
	}
	if taskIcon.IsMandatory {
		t.Errorf("Expected taskIcon to NOT be mandatory")
	}

	// 7. Admin creates chore with 1 coin -> Admin chore should NOT be reduced or made mandatory
	adminTask1Req := model.CreateTaskRequest{Title: "Quick wipe", PointsReward: 1, Type: "cooperative"}
	adminTask1Body, _ := json.Marshal(adminTask1Req)
	reqAdmin1 := mustNewRequestWithAuth("POST", createTaskURL, bytes.NewBuffer(adminTask1Body), adminLogin.UserID.String())
	rrAdmin1 := performRequest(t, reqAdmin1, http.StatusCreated)

	var adminTask1 model.Task
	if err := json.NewDecoder(rrAdmin1.Body).Decode(&adminTask1); err != nil {
		t.Fatalf("Failed to decode adminTask1: %v", err)
	}
	if adminTask1.PointsReward != 1 {
		t.Errorf("Expected admin-created task to keep 1 coin, got %d", adminTask1.PointsReward)
	}
	if adminTask1.IsMandatory {
		t.Errorf("Expected admin-created task to NOT be mandatory")
	}
}

