package model

import (
	"time"

	"github.com/google/uuid"
)

// --- Internal Database Models ---

// ChoreGroup represents the choregroups table.
type ChoreGroup struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// User represents the users table.
type User struct {
	ID            uuid.UUID `json:"id"`
	ChoreGroupID  uuid.UUID `json:"choregroup_id"`
	Username      string    `json:"username"`
	PasswordHash  string    `json:"-"`
	Role          string    `json:"role"`
	Points        int       `json:"points"`
}

// Task represents the tasks table.
type Task struct {
	ID               uuid.UUID  `json:"id"`
	ChoreGroupID     uuid.UUID  `json:"choregroup_id"`
	AssignedToUserID *uuid.UUID `json:"assigned_to_user_id,omitempty"`
	Title            string     `json:"title"`
	Type             string     `json:"type"`
	PointsReward     int        `json:"points_reward"`
	Status           string     `json:"status"`
}

// TaskSubmission represents the task_submissions table.
type TaskSubmission struct {
	ID          uuid.UUID `json:"id"`
	TaskID      uuid.UUID `json:"task_id"`
	SubmittedBy uuid.UUID `json:"submitted_by"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}


// --- API Data Transfer Objects (DTOs) ---

// SignUpRequest defines the body for creating a new choregroup and its first admin user.
type SignUpRequest struct {
	ChoreGroupName string `json:"choregroup_name"`
	Username       string `json:"username"`
	Password       string `json:"password"`
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
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse defines the data returned upon successful login.
type LoginResponse struct {
	UserID       uuid.UUID `json:"user_id"`
	ChoreGroupID uuid.UUID `json:"choregroup_id"`
	Role         string    `json:"role"`
}

// CreateTaskRequest defines the body for creating a new task.
type CreateTaskRequest struct {
	Title            string     `json:"title"`
	Type             string     `json:"type"`
	PointsReward     int        `json:"points_reward"`
	AssignedToUserID *uuid.UUID `json:"assigned_to_user_id,omitempty"`
}

// UpdateSubmissionRequest defines the body for approving or rejecting a submission.
type UpdateSubmissionRequest struct {
	Action string `json:"action"` // "approve" or "reject"
}
