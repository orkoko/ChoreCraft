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
	AILanguage        string    `json:"ai_language"`
	AIInstructions    string    `json:"ai_instructions"`
}

// User represents the users table.
type User struct {
	ID                    uuid.UUID `json:"id"`
	ChoreGroupID          uuid.UUID `json:"choregroup_id"`
	Username              string    `json:"username"`
	PasswordHash          string    `json:"-"`
	Role                  string    `json:"role"`
	Points                int       `json:"points"`
	NotificationsViewed   bool      `json:"notifications_viewed"`
	HasClaimedWelcomeSpin bool      `json:"has_claimed_welcome_spin"`
	ProfileImageURL       *string   `json:"profile_image_url,omitempty"`
	PointsBackground      string    `json:"points_background"`
	UnlockedBackgrounds   []string  `json:"unlocked_backgrounds"`
	AvatarFrame           string    `json:"avatar_frame"`
	UnlockedAvatarFrames  []string  `json:"unlocked_avatar_frames"`
}

// UpdateAvatarRequest defines the body for updating a user's profile image / avatar.
type UpdateAvatarRequest struct {
	AvatarURL string `json:"avatar_url"`
}

// UpdateAvatarFrameRequest defines the body for updating a user's equipped avatar frame.
type UpdateAvatarFrameRequest struct {
	AvatarFrame string `json:"avatar_frame"`
}

// UpdatePointsBackgroundRequest defines the body for updating a user's active points background.
type UpdatePointsBackgroundRequest struct {
	Background string `json:"background"`
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
	ParentTaskID     *uuid.UUID `json:"parent_task_id,omitempty"`
	ContinuationCount int        `json:"continuation_count"`
	IconURL          *string    `json:"icon_url,omitempty"`
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
	IconURL          *string    `json:"icon_url,omitempty"`
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
	UserID                uuid.UUID `json:"user_id"`
	Username              string    `json:"username"`
	ChoreGroupID          uuid.UUID `json:"choregroup_id"`
	ChoreGroupName        string    `json:"choregroup_name"`
	Role                  string    `json:"role"`
	HasClaimedWelcomeSpin bool      `json:"has_claimed_welcome_spin"`
	ProfileImageURL       *string   `json:"profile_image_url,omitempty"`
	PointsBackground      string    `json:"points_background,omitempty"`
	UnlockedBackgrounds   []string  `json:"unlocked_backgrounds,omitempty"`
	AvatarFrame           string    `json:"avatar_frame,omitempty"`
	UnlockedAvatarFrames  []string  `json:"unlocked_avatar_frames,omitempty"`
	AILanguage            string    `json:"ai_language,omitempty"`
	AIInstructions        string    `json:"ai_instructions,omitempty"`
}

// CreateTaskRequest defines the body for creating a new task.
type CreateTaskRequest struct {
	Title            string     `json:"title"`
	Type             string     `json:"type"`
	PointsReward     int        `json:"points_reward"`
	IsMandatory      bool       `json:"is_mandatory"`
	AssignedToUserID *uuid.UUID `json:"assigned_to_user_id,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	ParentTaskID     *uuid.UUID `json:"parent_task_id,omitempty"`
	IconURL          *string    `json:"icon_url,omitempty"`
}

// UpdateTaskRequest defines the body for updating an existing task.
type UpdateTaskRequest struct {
	Title            string     `json:"title"`
	Type             string     `json:"type"`
	PointsReward     int        `json:"points_reward"`
	IsMandatory      bool       `json:"is_mandatory"`
	AssignedToUserID *uuid.UUID `json:"assigned_to_user_id,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	ParentTaskID     *uuid.UUID `json:"parent_task_id,omitempty"`
	IconURL          *string    `json:"icon_url,omitempty"`
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
	IconURL          *string    `json:"icon_url,omitempty"`
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

// AIImageAnalysisResult defines the response returned by the AI image analyzer.
type AIImageAnalysisResult struct {
	IsValid            bool    `json:"is_valid"`            // false if image doesn't show a recognizable chore or reward
	InvalidReason      string  `json:"invalid_reason"`      // AI explanation in target language why there is no chore
	ItemType           string  `json:"item_type"`           // "chore" or "reward"
	Title              string  `json:"title"`
	Description        string  `json:"description"`
	Category           string  `json:"category"` // "personal", "cooperative", "mandatory", "individual"
	EstimatedUSD       float64 `json:"estimated_usd"`
	PointsReward       int     `json:"points_reward"` // Coin count
	ValuationReasoning string  `json:"valuation_reasoning"`
	IconURL            string  `json:"icon_url"`
	SuggestedEmoji     string  `json:"suggested_emoji"`
}

// UserDailyProgress represents the user_daily_progress table.
type UserDailyProgress struct {
	ID                uuid.UUID `json:"id"`
	UserID            uuid.UUID `json:"user_id"`
	ChoreGroupID      uuid.UUID `json:"choregroup_id"`
	Date              time.Time `json:"date"`
	TasksCompleted    int       `json:"tasks_completed"`
	SpinsEarned       int       `json:"spins_earned"`
	SpinsUsed         int       `json:"spins_used"`
	DoubleCoinsActive bool      `json:"double_coins_active"`
	DoubleCoinsUsed   bool      `json:"double_coins_used"`
	SurpriseClaimed   bool      `json:"surprise_claimed"`
}

// DailyBonusStatusResponse defines the status returned for the daily bonus bar.
type DailyBonusStatusResponse struct {
	Date                  string `json:"date"`
	TasksCompleted        int    `json:"tasks_completed"`
	TargetWheel           int    `json:"target_wheel"`
	TargetDouble          int    `json:"target_double"`
	TargetSpin2           int    `json:"target_spin_2"`
	TargetSpin3           int    `json:"target_spin_3"`
	TargetSpin4           int    `json:"target_spin_4"`
	TargetSurprise        int    `json:"target_surprise"`
	SpinsAvailable        int    `json:"spins_available"`
	WheelUnlocked         bool   `json:"wheel_unlocked"`
	DoubleCoinsActive     bool   `json:"double_coins_active"`
	DoubleCoinsUsed       bool   `json:"double_coins_used"`
	SurpriseClaimed       bool   `json:"surprise_claimed"`
	HasClaimedWelcomeSpin bool   `json:"has_claimed_welcome_spin"`
}

// DailySurpriseResponse defines the response returned after claiming the 10-task daily surprise.
type DailySurpriseResponse struct {
	ThemeID    *string `json:"theme_id,omitempty"`
	FrameID    *string `json:"frame_id,omitempty"`
	BonusCoins int     `json:"bonus_coins"`
	UserPoints int     `json:"user_points"`
	Message    string  `json:"message"`
}

// WheelSpinResponse defines the response returned after spinning the lucky wheel.
type WheelSpinResponse struct {
	SliceIndex         int     `json:"slice_index"`
	Label              string  `json:"label"`
	RewardType         string  `json:"reward_type"` // "individual", "cooperative", "none"
	Amount             int     `json:"amount"`
	UserPoints         int     `json:"user_points"`
	CooperativePoints  int     `json:"cooperative_points"`
	SpinsRemaining     int     `json:"spins_remaining"`
	SurpriseBackground *string `json:"surprise_background,omitempty"`
	SurpriseFrame      *string `json:"surprise_frame,omitempty"`
}

// ActivityEvent represents a recorded activity in a household.
type ActivityEvent struct {
	ID                 uuid.UUID              `json:"id"`
	ChoreGroupID       uuid.UUID              `json:"choregroup_id"`
	UserID             *uuid.UUID             `json:"user_id,omitempty"`
	UserUsername       string                 `json:"user_username,omitempty"`
	UserProfileImage   *string                `json:"user_profile_image,omitempty"`
	UserAvatarFrame    *string                `json:"user_avatar_frame,omitempty"`
	UserRole           string                 `json:"user_role,omitempty"`
	TargetUserID       *uuid.UUID             `json:"target_user_id,omitempty"`
	TargetUsername     string                 `json:"target_username,omitempty"`
	TargetProfileImage *string                `json:"target_profile_image,omitempty"`
	TargetAvatarFrame  *string                `json:"target_avatar_frame,omitempty"`
	ActivityType       string                 `json:"activity_type"`
	Title              string                 `json:"title"`
	Description        string                 `json:"description"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
}

// HistoryFilterParams defines query filters for fetching household activity history.
type HistoryFilterParams struct {
	StartDate        *time.Time `json:"start_date,omitempty"`
	EndDate          *time.Time `json:"end_date,omitempty"`
	UserID           *uuid.UUID `json:"user_id,omitempty"`
	ActivityCategory string     `json:"activity_category,omitempty"` // "all", "chores", "rewards", "wheel", "themes", "members"
	Limit            int        `json:"limit,omitempty"`
	Offset           int        `json:"offset,omitempty"`
}

// HistoryResponse defines the paginated response for household activity history.
type HistoryResponse struct {
	Events     []ActivityEvent `json:"events"`
	TotalCount int             `json:"total_count"`
	StartDate  string          `json:"start_date,omitempty"`
	EndDate    string          `json:"end_date,omitempty"`
}

// Upload represents an uploaded file stored as binary BYTEA in the database repository.
type Upload struct {
	ID        uuid.UUID `json:"id"`
	Filename  string    `json:"filename"`
	MIMEType  string    `json:"mime_type"`
	Data      []byte    `json:"-"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}



