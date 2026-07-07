package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Internal Database Models ---

// ChoreGroup represents the choregroups table.
type ChoreGroup struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	CooperativePoints int       `json:"cooperative_points"`
}

// User represents the users table.
type User struct {
	ID                  uuid.UUID `json:"id"`
	ChoreGroupID        uuid.UUID `json:"choregroup_id"`
	Username            string    `json:"username"`
	PasswordHash        string    `json:"-"`
	Role                string    `json:"role"`
	Points              int       `json:"points"`
	NotificationsViewed bool      `json:"notifications_viewed"`
}

// Task represents the tasks table.
type Task struct {
	ID               uuid.UUID  `json:"id"`
	ChoreGroupID     uuid.UUID  `json:"choregroup_id"`
	AssignedToUserID *uuid.UUID `json:"assigned_to_user_id,omitempty"`
	Title            string     `json:"title"`
	Type             string     `json:"type"`
	PointsReward     int        `json:"points_reward"`
	IsMandatory      bool       `json:"is_mandatory"`
	Status           string     `json:"status"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

// TaskSubmission represents the task_submissions table.
type TaskSubmission struct {
	ID          uuid.UUID `json:"id"`
	TaskID      uuid.UUID `json:"task_id"`
	SubmittedBy uuid.UUID `json:"submitted_by"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// Reward represents the rewards table.
type Reward struct {
	ID               uuid.UUID  `json:"id"`
	ChoreGroupID     uuid.UUID  `json:"choregroup_id"`
	Name             string     `json:"name"`
	Description      *string    `json:"description,omitempty"`
	Cost             int        `json:"cost"`
	Type             string     `json:"type"`
	AssignedToUserID *uuid.UUID `json:"assigned_to_user_id,omitempty"`
}

// RewardPurchase represents the reward_purchases table.
type RewardPurchase struct {
	ID                 uuid.UUID       `json:"id"`
	RewardID           uuid.UUID       `json:"reward_id"`
	PurchasedByUserID  uuid.UUID       `json:"purchased_by_user_id"`
	Status             string          `json:"status"`
	Approvals          json.RawMessage `json:"approvals,omitempty"`
}


// --- API Data Transfer Objects (DTOs) ---

// SignUpRequest defines the body for creating a new choregroup and its first admin user.
type SignUpRequest struct {
	ChoreGroupName string `json:"choregroup_name"`
	Username       string `json:"username"`
	Password       string `json:"password"`
}

// UpdatePasswordRequest defines the body for changing a user's password.
type UpdatePasswordRequest struct {
	Password string `json:"password"`
}


// AddUserRequest defines the body for adding a new user to an existing choregroup.
type AddUserRequest struct {
	ChoreGroupName string `json:"choregroup_name"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	UserRole       string `json:"user_role"` // Should be 'user' or 'admin'
}

// LoginRequest defines the body for user authentication.
type LoginRequest struct {
	ChoreGroupName string `json:"choregroup_name,omitempty"`
	Username       string `json:"username"`
	Password       string `json:"password"`
}

// PINLoginRequest defines the body for PIN-based user authentication.
type PINLoginRequest struct {
	UserID uuid.UUID `json:"user_id"`
	PIN    string    `json:"pin"`
}

// DelegatedLoginRequest defines the body for delegated link login.
type DelegatedLoginRequest struct {
	Token string `json:"token"`
}

// LoginResponse defines the data returned upon successful login.
type LoginResponse struct {
	UserID         uuid.UUID `json:"user_id"`
	Username       string    `json:"username"`
	ChoreGroupID   uuid.UUID `json:"choregroup_id"`
	ChoreGroupName string    `json:"choregroup_name"`
	Role           string    `json:"role"`
}

// CreateTaskRequest defines the body for creating a new task.
type CreateTaskRequest struct {
	Title            string     `json:"title"`
	Type             string     `json:"type"`
	PointsReward     int        `json:"points_reward"`
	IsMandatory      bool       `json:"is_mandatory"`
	AssignedToUserID *uuid.UUID `json:"assigned_to_user_id,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

// UpdateTaskRequest defines the body for updating an existing task.
type UpdateTaskRequest struct {
	Title            string     `json:"title"`
	Type             string     `json:"type"`
	PointsReward     int        `json:"points_reward"`
	IsMandatory      bool       `json:"is_mandatory"`
	AssignedToUserID *uuid.UUID `json:"assigned_to_user_id,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

// UpdateSubmissionRequest defines the body for approving or rejecting a submission.
type UpdateSubmissionRequest struct {
	Action string `json:"action"` // "approve" or "reject"
}

// StatisticsResponse defines the data returned by the statistics endpoint.
type StatisticsResponse struct {
	Users             []User `json:"users"`
	CooperativePoints int    `json:"cooperative_points"`
}

// CreateRewardRequest defines the body for creating a new reward.
type CreateRewardRequest struct {
	Name             string     `json:"name"`
	Description      *string    `json:"description,omitempty"`
	Cost             int        `json:"cost"`
	Type             string     `json:"type"`
	AssignedToUserID *uuid.UUID `json:"assigned_to_user_id,omitempty"`
}

// CreatePurchaseRequest defines the body for creating a new reward purchase.
type CreatePurchaseRequest struct {
	RewardID uuid.UUID `json:"reward_id"`
}

// CreateApprovalRequest defines the body for approving a reward purchase.
type CreateApprovalRequest struct {
	Vote string `json:"vote"` // "approved" or "rejected"
}

// UpdatePurchaseStatusRequest defines the body for updating a purchase status.
type UpdatePurchaseStatusRequest struct {
	Status string `json:"status"` // "fulfilled"
}

// PushSubscription represents a web push subscription stored in the database.
type PushSubscription struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Endpoint  string    `json:"endpoint"`
	P256dh    string    `json:"p256dh"`
	Auth      string    `json:"auth"`
	CreatedAt time.Time `json:"created_at"`
}

// PushSubscribeRequest defines the body for subscribing to web push notifications.
type PushSubscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}
