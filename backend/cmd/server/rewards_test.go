package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"ChoreCraft/internal/model"
)

func TestRewardLifecycle(t *testing.T) {
	clearAllTables(t)

	// 1. Setup Admin and 2 Users
	signup := model.SignUpRequest{ChoreGroupName: "Reward Group", Username: "reward_admin", Password: "pw"}
	signupBody, _ := json.Marshal(signup)
	performRequest(t, mustNewRequest("POST", "/api/signup", bytes.NewBuffer(signupBody)), http.StatusCreated)

	adminLogin := loginUser(t, "reward_admin", "pw")

	addUser(t, "reward_user1", "pw", "Reward Group")
	user1Login := loginUser(t, "reward_user1", "pw")

	addUser(t, "reward_user2", "pw", "Reward Group")
	user2Login := loginUser(t, "reward_user2", "pw")

	// 2. Admin creates an individual and a cooperative reward
	individualRewardReq := model.CreateRewardRequest{Name: "New Game", Cost: 100, Type: "individual"}
	cooperativeRewardReq := model.CreateRewardRequest{Name: "Pizza Night", Cost: 200, Type: "cooperative"}

	createRewardURL := fmt.Sprintf("/api/choregroups/%s/rewards", adminLogin.ChoreGroupID)
	rrInd := performRequest(t, mustNewRequestWithAuth("POST", createRewardURL, toBody(individualRewardReq), adminLogin.UserID.String()), http.StatusCreated)
	var individualReward model.Reward
	json.NewDecoder(rrInd.Body).Decode(&individualReward)

	rrCoop := performRequest(t, mustNewRequestWithAuth("POST", createRewardURL, toBody(cooperativeRewardReq), adminLogin.UserID.String()), http.StatusCreated)
	var cooperativeReward model.Reward
	json.NewDecoder(rrCoop.Body).Decode(&cooperativeReward)

	// 3. Give users points (manually, for testing)
	dbExec(t, "UPDATE users SET points = 150 WHERE username = 'reward_user1'")
	dbExec(t, "UPDATE choregroups SET cooperative_points = 250 WHERE name = 'Reward Group'")

	// 4. User1 buys an individual reward
	t.Run("Individual Reward Purchase", func(t *testing.T) {
		purchaseURL := fmt.Sprintf("/api/choregroups/%s/purchases", user1Login.ChoreGroupID)
		purchaseReq := model.CreatePurchaseRequest{RewardID: individualReward.ID}
		rrPurchase := performRequest(t, mustNewRequestWithAuth("POST", purchaseURL, toBody(purchaseReq), user1Login.UserID.String()), http.StatusCreated)
		var purchase model.RewardPurchase
		json.NewDecoder(rrPurchase.Body).Decode(&purchase)

		if purchase.Status != "approved" {
			t.Fatalf("Expected individual purchase to be immediately approved, got status %s", purchase.Status)
		}

		// Verify points were deducted
		stats := getStats(t, adminLogin)
		for _, u := range stats.Users {
			if u.Username == "reward_user1" && u.Points != 50 {
				t.Errorf("Expected user1 to have 50 points, but got %d", u.Points)
			}
		}
	})

	// 5. User1 buys a cooperative reward, User2 approves
	t.Run("Cooperative Reward Purchase and Approval", func(t *testing.T) {
		purchaseURL := fmt.Sprintf("/api/choregroups/%s/purchases", user1Login.ChoreGroupID)
		purchaseReq := model.CreatePurchaseRequest{RewardID: cooperativeReward.ID}
		rrPurchase := performRequest(t, mustNewRequestWithAuth("POST", purchaseURL, toBody(purchaseReq), user1Login.UserID.String()), http.StatusCreated)
		var purchase model.RewardPurchase
		json.NewDecoder(rrPurchase.Body).Decode(&purchase)

		if purchase.Status != "pending_approval" {
			t.Fatalf("Expected cooperative purchase to be pending approval, got status %s", purchase.Status)
		}

		// Verify cooperative points were deducted
		stats := getStats(t, adminLogin)
		if stats.CooperativePoints != 50 {
			t.Errorf("Expected cooperative points to be 50, but got %d", stats.CooperativePoints)
		}

		// User2 approves
		approvalURL := fmt.Sprintf("/api/choregroups/%s/purchases/%s/approvals", user2Login.ChoreGroupID, purchase.ID)
		approvalReq := model.CreateApprovalRequest{Vote: "approved"}
		performRequest(t, mustNewRequestWithAuth("POST", approvalURL, toBody(approvalReq), user2Login.UserID.String()), http.StatusNoContent)

		// Admin fulfills the reward
		fulfillURL := fmt.Sprintf("/api/choregroups/%s/purchases/%s/status", adminLogin.ChoreGroupID, purchase.ID)
		fulfillReq := model.UpdatePurchaseStatusRequest{Status: "fulfilled"}
		performRequest(t, mustNewRequestWithAuth("PUT", fulfillURL, toBody(fulfillReq), adminLogin.UserID.String()), http.StatusNoContent)

		// Verify status is fulfilled
		listPurchasesURL := fmt.Sprintf("/api/choregroups/%s/purchases?status=fulfilled", adminLogin.ChoreGroupID)
		rrList := performRequest(t, mustNewRequestWithAuth("GET", listPurchasesURL, nil, adminLogin.UserID.String()), http.StatusOK)
		var fulfilled []model.RewardPurchase
		json.NewDecoder(rrList.Body).Decode(&fulfilled)
		if len(fulfilled) != 1 || fulfilled[0].ID != purchase.ID {
			t.Fatalf("Expected to find 1 fulfilled purchase, but got %d", len(fulfilled))
		}
	})

	t.Run("Cancel Purchase by Kid", func(t *testing.T) {
		// Reset user1 points to 100
		dbExec(t, "UPDATE users SET points = 100 WHERE username = 'reward_user1'")
		// Reset cooperative points to 200
		dbExec(t, "UPDATE choregroups SET cooperative_points = 200 WHERE name = 'Reward Group'")

		// 1. Purchase individual reward
		purchaseURL := fmt.Sprintf("/api/choregroups/%s/purchases", user1Login.ChoreGroupID)
		purchaseReq := model.CreatePurchaseRequest{RewardID: individualReward.ID}
		rrPurchase := performRequest(t, mustNewRequestWithAuth("POST", purchaseURL, toBody(purchaseReq), user1Login.UserID.String()), http.StatusCreated)
		var purchase model.RewardPurchase
		json.NewDecoder(rrPurchase.Body).Decode(&purchase)

		// Points should be 0 (100 - 100)
		stats := getStats(t, adminLogin)
		for _, u := range stats.Users {
			if u.Username == "reward_user1" && u.Points != 0 {
				t.Errorf("Expected user1 points to be 0 after purchase, got %d", u.Points)
			}
		}

		// Cancel purchase
		cancelURL := fmt.Sprintf("/api/choregroups/%s/purchases/%s", user1Login.ChoreGroupID, purchase.ID)
		performRequest(t, mustNewRequestWithAuth("DELETE", cancelURL, nil, user1Login.UserID.String()), http.StatusNoContent)

		// Verify points are refunded (should be 100 again)
		stats = getStats(t, adminLogin)
		for _, u := range stats.Users {
			if u.Username == "reward_user1" && u.Points != 100 {
				t.Errorf("Expected user1 points to be 100 after cancel, got %d", u.Points)
			}
		}

		// Verify purchase is deleted
		listAllURL := fmt.Sprintf("/api/choregroups/%s/purchases", adminLogin.ChoreGroupID)
		rrAll := performRequest(t, mustNewRequestWithAuth("GET", listAllURL, nil, adminLogin.UserID.String()), http.StatusOK)
		var all []model.RewardPurchase
		json.NewDecoder(rrAll.Body).Decode(&all)
		// Check that the purchase ID doesn't exist anymore in the list (or list only contains the previously fulfilled one)
		for _, p := range all {
			if p.ID == purchase.ID {
				t.Errorf("Expected purchase %s to be deleted, but it is still returned", p.ID)
			}
		}

		// 2. Cooperative purchase cancellation
		coopPurchaseReq := model.CreatePurchaseRequest{RewardID: cooperativeReward.ID}
		rrCoop := performRequest(t, mustNewRequestWithAuth("POST", purchaseURL, toBody(coopPurchaseReq), user1Login.UserID.String()), http.StatusCreated)
		var coopPurchase model.RewardPurchase
		json.NewDecoder(rrCoop.Body).Decode(&coopPurchase)

		// Points check: user1 points should be 100 - 200 (error, wait! reward cost is 200, user1 only has 100!)
		// Oh! The individual points of user1 needs to be >= 200!
		// Let's set user1 points to 300
		dbExec(t, "UPDATE users SET points = 300 WHERE username = 'reward_user1'")
		// Let's set coop points to 300
		dbExec(t, "UPDATE choregroups SET cooperative_points = 300 WHERE name = 'Reward Group'")

		rrCoop2 := performRequest(t, mustNewRequestWithAuth("POST", purchaseURL, toBody(coopPurchaseReq), user1Login.UserID.String()), http.StatusCreated)
		var coopPurchase2 model.RewardPurchase
		json.NewDecoder(rrCoop2.Body).Decode(&coopPurchase2)

		// Cancel cooperative purchase
		cancelCoopURL := fmt.Sprintf("/api/choregroups/%s/purchases/%s", user1Login.ChoreGroupID, coopPurchase2.ID)
		performRequest(t, mustNewRequestWithAuth("DELETE", cancelCoopURL, nil, user1Login.UserID.String()), http.StatusNoContent)

		// Verify user1 points are refunded (should be 300 again)
		stats = getStats(t, adminLogin)
		for _, u := range stats.Users {
			if u.Username == "reward_user1" && u.Points != 300 {
				t.Errorf("Expected user1 points to be 300 after coop cancel, got %d", u.Points)
			}
		}
		// Verify coop points are refunded (should be 300 again)
		if stats.CooperativePoints != 300 {
			t.Errorf("Expected coop points to be 300 after cancel, got %d", stats.CooperativePoints)
		}
	})
}

// Helper functions to reduce boilerplate
func mustNewRequest(method, url string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	return req
}

var userTokens = make(map[string]string)

func mustNewRequestWithAuth(method, url string, body io.Reader, userID string) *http.Request {
	req := mustNewRequest(method, url, body)
	if token, ok := userTokens[userID]; ok {
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		req.Header.Set("X-User-ID", userID)
	}
	return req
}

func toBody(v interface{}) *bytes.Buffer {
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(v)
	return body
}

func loginUser(t *testing.T, username, password string) model.LoginResponse {
	t.Helper()
	loginReq := model.LoginRequest{Username: username, Password: password}
	rr := performRequest(t, mustNewRequest("POST", "/api/login", toBody(loginReq)), http.StatusOK)
	var loginRes model.LoginResponse
	json.NewDecoder(rr.Body).Decode(&loginRes)

	// Capture token cookie
	cookies := rr.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "token" {
			userTokens[loginRes.UserID.String()] = c.Value
		}
	}
	return loginRes
}

func addUser(t *testing.T, username, password, groupName string) {
	t.Helper()
	addUserReq := model.AddUserRequest{ChoreGroupName: groupName, Username: username, Password: password, UserRole: "user"}
	performRequest(t, mustNewRequest("POST", "/api/users", toBody(addUserReq)), http.StatusCreated)
}

func dbExec(t *testing.T, query string, args ...interface{}) {
	t.Helper()
	if _, err := testDbPool.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("DB exec failed: %v", err)
	}
}

func getStats(t *testing.T, login model.LoginResponse) model.StatisticsResponse {
	t.Helper()
	statsURL := fmt.Sprintf("/api/choregroups/%s/statistics", login.ChoreGroupID)
	req := mustNewRequestWithAuth("GET", statsURL, nil, login.UserID.String())
	rr := performRequest(t, req, http.StatusOK)
	var stats model.StatisticsResponse
	json.NewDecoder(rr.Body).Decode(&stats)
	return stats
}
