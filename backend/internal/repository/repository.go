package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ChoreCraft/internal/model"
)

func parseUnlockedBackgrounds(raw []byte) []string {
	defaults := []string{"default", "autumn_leaves", "golden_stars"}
	if len(raw) == 0 {
		return defaults
	}
	var res []string
	if err := json.Unmarshal(raw, &res); err != nil {
		return defaults
	}
	for _, d := range defaults {
		found := false
		for _, r := range res {
			if r == d {
				found = true
				break
			}
		}
		if !found {
			res = append(res, d)
		}
	}
	return res
}

func parseUnlockedFrames(raw []byte) []string {
	defaults := []string{"none"}
	if len(raw) == 0 {
		return defaults
	}
	var res []string
	if err := json.Unmarshal(raw, &res); err != nil {
		return defaults
	}
	for _, d := range defaults {
		found := false
		for _, r := range res {
			if r == d {
				found = true
				break
			}
		}
		if !found {
			res = append(res, d)
		}
	}
	return res
}

// AllPointsBackgrounds contains all available background themes in ChoreCraft.
var AllPointsBackgrounds = []string{
	"default",
	"autumn_leaves",
	"golden_stars",
	"cherry_blossom",
	"cosmic_galaxy",
	"emerald_nature",
	"candy_party",
	"fire_dragon",
}

// AllAvatarFrames contains all available profile avatar frames in ChoreCraft.
var AllAvatarFrames = []string{
	"none",
	"golden_sparkle",
	"fire_flame",
	"cosmic_nebula",
	"emerald_nature",
	"rainbow_wave",
	"royal_crown",
	"neon_cyber",
}

func sanitizeUserCosmetics(user *model.User) {
	if user.Role == "admin" {
		user.UnlockedBackgrounds = make([]string, len(AllPointsBackgrounds))
		copy(user.UnlockedBackgrounds, AllPointsBackgrounds)
		user.UnlockedAvatarFrames = make([]string, len(AllAvatarFrames))
		copy(user.UnlockedAvatarFrames, AllAvatarFrames)
	}
}

var ErrNotFound = errors.New("not found")

// Repository holds the database connection pool.
type Repository struct {
	db *pgxpool.Pool
}

// New creates a new Repository instance.
func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateChoreGroupAndUser creates a new choregroup and an initial user with a hashed password.
func (r *Repository) CreateChoreGroupAndUser(ctx context.Context, choregroupName, username, passwordHash, userRole string) (model.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.User{}, err
	}
	defer tx.Rollback(ctx)

	choregroupID := uuid.New()
	_, err = tx.Exec(ctx, "INSERT INTO choregroups (id, name) VALUES ($1, $2)", choregroupID, choregroupName)
	if err != nil {
		return model.User{}, err
	}

	user := model.User{
		ID:                  uuid.New(),
		ChoreGroupID:        choregroupID,
		Username:            username,
		PasswordHash:        passwordHash,
		Role:                userRole,
		PointsBackground:    "default",
		AvatarFrame:         "none",
		UnlockedBackgrounds: []string{"default", "autumn_leaves", "golden_stars"},
		UnlockedAvatarFrames: []string{"none"},
	}
	sanitizeUserCosmetics(&user)
	_, err = tx.Exec(ctx, "INSERT INTO users (id, choregroup_id, username, password_hash, role) VALUES ($1, $2, $3, $4, $5)",
		user.ID, user.ChoreGroupID, user.Username, user.PasswordHash, user.Role)
	if err != nil {
		return model.User{}, err
	}

	return user, tx.Commit(ctx)
}

// AddUserToChoreGroup adds a new user with a hashed password to an existing choregroup.
func (r *Repository) AddUserToChoreGroup(ctx context.Context, choregroupName, username, passwordHash, userRole string) (model.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.User{}, err
	}
	defer tx.Rollback(ctx)

	var choregroupID uuid.UUID
	err = tx.QueryRow(ctx, "SELECT id FROM choregroups WHERE name = $1", choregroupName).Scan(&choregroupID)
	if err != nil {
		return model.User{}, err // ChoreGroup not found
	}

	user := model.User{
		ID:                  uuid.New(),
		ChoreGroupID:        choregroupID,
		Username:            username,
		PasswordHash:        passwordHash,
		Role:                userRole,
		PointsBackground:    "default",
		AvatarFrame:         "none",
		UnlockedBackgrounds: []string{"default", "autumn_leaves", "golden_stars"},
		UnlockedAvatarFrames: []string{"none"},
	}
	sanitizeUserCosmetics(&user)
	_, err = tx.Exec(ctx, "INSERT INTO users (id, choregroup_id, username, password_hash, role) VALUES ($1, $2, $3, $4, $5)",
		user.ID, user.ChoreGroupID, user.Username, user.PasswordHash, user.Role)
	if err != nil {
		return model.User{}, err
	}

	return user, tx.Commit(ctx)
}

// GetUserByUsername retrieves a user by their unique username.
func (r *Repository) GetUserByUsername(ctx context.Context, username string) (model.User, error) {
	var user model.User
	var unlockedRaw []byte
	var unlockedFramesRaw []byte
	err := r.db.QueryRow(ctx, "SELECT id, choregroup_id, username, password_hash, role, points, notifications_viewed, has_claimed_welcome_spin, profile_image_url, points_background, unlocked_backgrounds, avatar_frame, unlocked_avatar_frames FROM users WHERE username = $1", username).Scan(
		&user.ID, &user.ChoreGroupID, &user.Username, &user.PasswordHash, &user.Role, &user.Points, &user.NotificationsViewed, &user.HasClaimedWelcomeSpin, &user.ProfileImageURL, &user.PointsBackground, &unlockedRaw, &user.AvatarFrame, &unlockedFramesRaw)
	user.UnlockedBackgrounds = parseUnlockedBackgrounds(unlockedRaw)
	user.UnlockedAvatarFrames = parseUnlockedFrames(unlockedFramesRaw)
	if user.PointsBackground == "" {
		user.PointsBackground = "default"
	}
	if user.AvatarFrame == "" {
		user.AvatarFrame = "none"
	}
	sanitizeUserCosmetics(&user)
	return user, err
}

// UpdateUserPassword updates the password hash for a specific user.
func (r *Repository) UpdateUserPassword(ctx context.Context, userID, choregroupID uuid.UUID, newPasswordHash string) error {
	_, err := r.db.Exec(ctx, "UPDATE users SET password_hash = $1 WHERE id = $2 AND choregroup_id = $3", newPasswordHash, userID, choregroupID)
	return err
}


// GetUserByID retrieves a user by their ID.
func (r *Repository) GetUserByID(ctx context.Context, userID uuid.UUID) (model.User, error) {
	var user model.User
	var unlockedRaw []byte
	var unlockedFramesRaw []byte
	err := r.db.QueryRow(ctx, "SELECT id, choregroup_id, username, role, points, notifications_viewed, has_claimed_welcome_spin, profile_image_url, points_background, unlocked_backgrounds, avatar_frame, unlocked_avatar_frames FROM users WHERE id = $1", userID).Scan(
		&user.ID, &user.ChoreGroupID, &user.Username, &user.Role, &user.Points, &user.NotificationsViewed, &user.HasClaimedWelcomeSpin, &user.ProfileImageURL, &user.PointsBackground, &unlockedRaw, &user.AvatarFrame, &unlockedFramesRaw)
	user.UnlockedBackgrounds = parseUnlockedBackgrounds(unlockedRaw)
	user.UnlockedAvatarFrames = parseUnlockedFrames(unlockedFramesRaw)
	if user.PointsBackground == "" {
		user.PointsBackground = "default"
	}
	if user.AvatarFrame == "" {
		user.AvatarFrame = "none"
	}
	sanitizeUserCosmetics(&user)
	return user, err
}

// GetUsersByChoreGroupID retrieves all users belonging to a specific choregroup.
func (r *Repository) GetUsersByChoreGroupID(ctx context.Context, choregroupID uuid.UUID) ([]model.User, error) {
	rows, err := r.db.Query(ctx, "SELECT id, choregroup_id, username, role, points, notifications_viewed, has_claimed_welcome_spin, profile_image_url, points_background, unlocked_backgrounds, avatar_frame, unlocked_avatar_frames FROM users WHERE choregroup_id = $1", choregroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var user model.User
		var unlockedRaw []byte
		var unlockedFramesRaw []byte
		if err := rows.Scan(&user.ID, &user.ChoreGroupID, &user.Username, &user.Role, &user.Points, &user.NotificationsViewed, &user.HasClaimedWelcomeSpin, &user.ProfileImageURL, &user.PointsBackground, &unlockedRaw, &user.AvatarFrame, &unlockedFramesRaw); err != nil {
			return nil, err
		}
		user.UnlockedBackgrounds = parseUnlockedBackgrounds(unlockedRaw)
		user.UnlockedAvatarFrames = parseUnlockedFrames(unlockedFramesRaw)
		if user.PointsBackground == "" {
			user.PointsBackground = "default"
		}
		if user.AvatarFrame == "" {
			user.AvatarFrame = "none"
		}
		sanitizeUserCosmetics(&user)
		users = append(users, user)
	}
	return users, nil
}

// DeleteUser deletes a user by their ID and choregroupID.
func (r *Repository) DeleteUser(ctx context.Context, userID, choregroupID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM users WHERE id = $1 AND choregroup_id = $2", userID, choregroupID)
	return err
}

// GetChoreGroupByID retrieves a choregroup by its ID.
func (r *Repository) GetChoreGroupByID(ctx context.Context, choregroupID uuid.UUID) (model.ChoreGroup, error) {
	var group model.ChoreGroup
	err := r.db.QueryRow(ctx, "SELECT id, name, cooperative_points, COALESCE(ai_language, 'he'), COALESCE(ai_instructions, '') FROM choregroups WHERE id = $1", choregroupID).Scan(
		&group.ID, &group.Name, &group.CooperativePoints, &group.AILanguage, &group.AIInstructions)
	return group, err
}

// CreateTask creates a new task scoped to a choregroup.
func (r *Repository) CreateTask(ctx context.Context, task *model.Task) error {
	_, err := r.db.Exec(ctx, "INSERT INTO tasks (id, choregroup_id, title, type, points_reward, is_mandatory, assigned_to_user_id, status, expires_at, parent_task_id, icon_url) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)",
		task.ID, task.ChoreGroupID, task.Title, task.Type, task.PointsReward, task.IsMandatory, task.AssignedToUserID, task.Status, task.ExpiresAt, task.ParentTaskID, task.IconURL)
	return err
}

// UpdateTask updates an existing task, verified by choregroupID.
func (r *Repository) UpdateTask(ctx context.Context, taskID, choregroupID uuid.UUID, req model.UpdateTaskRequest) error {
	_, err := r.db.Exec(ctx, "UPDATE tasks SET title = $1, type = $2, points_reward = $3, is_mandatory = $4, assigned_to_user_id = $5, expires_at = $6, parent_task_id = $7, icon_url = $8 WHERE id = $9 AND choregroup_id = $10",
		req.Title, req.Type, req.PointsReward, req.IsMandatory, req.AssignedToUserID, req.ExpiresAt, req.ParentTaskID, req.IconURL, taskID, choregroupID)
	return err
}

// DeleteTask deletes a task, verified by choregroupID.
func (r *Repository) DeleteTask(ctx context.Context, taskID, choregroupID uuid.UUID) error {
	// Delete any submissions referencing this task first
	_, err := r.db.Exec(ctx, "DELETE FROM task_submissions WHERE task_id = $1 AND task_id IN (SELECT id FROM tasks WHERE choregroup_id = $2)", taskID, choregroupID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, "DELETE FROM tasks WHERE id = $1 AND choregroup_id = $2", taskID, choregroupID)
	return err
}

// UpdateTaskStatus updates the status of a task, verified by choregroupID.
func (r *Repository) UpdateTaskStatus(ctx context.Context, taskID, choregroupID uuid.UUID, status string) error {
	_, err := r.db.Exec(ctx, "UPDATE tasks SET status = $1 WHERE id = $2 AND choregroup_id = $3", status, taskID, choregroupID)
	return err
}

// ClearTaskIcon clears the icon_url for a task so it falls back to its emoji.
func (r *Repository) ClearTaskIcon(ctx context.Context, taskID, choregroupID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "UPDATE tasks SET icon_url = NULL WHERE id = $1 AND choregroup_id = $2", taskID, choregroupID)
	return err
}

// ListTasksForUser retrieves tasks for a specific user in a choregroup (public and assigned).
func (r *Repository) ListTasksForUser(ctx context.Context, choregroupID, userID uuid.UUID) ([]model.Task, error) {
	query := `
		SELECT id, choregroup_id, title, type, points_reward, is_mandatory, assigned_to_user_id, status, expires_at, parent_task_id, icon_url
		FROM tasks
		WHERE choregroup_id = $1 AND (assigned_to_user_id IS NULL OR assigned_to_user_id = $2)
	`
	rows, err := r.db.Query(ctx, query, choregroupID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var task model.Task
		if err := rows.Scan(&task.ID, &task.ChoreGroupID, &task.Title, &task.Type, &task.PointsReward, &task.IsMandatory, &task.AssignedToUserID, &task.Status, &task.ExpiresAt, &task.ParentTaskID, &task.IconURL); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// HasActiveMandatoryTasksForUser checks if a user has any active mandatory tasks (assigned or pending approval).
func (r *Repository) HasActiveMandatoryTasksForUser(ctx context.Context, choregroupID, userID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM tasks
			WHERE choregroup_id = $1
			  AND is_mandatory = true
			  AND status IN ('assigned', 'pending_approval')
			  AND (assigned_to_user_id IS NULL OR assigned_to_user_id = $2)
		)
	`
	var exists bool
	err := r.db.QueryRow(ctx, query, choregroupID, userID).Scan(&exists)
	return exists, err
}

// ListAllTasks for ChoreGroup retrieves all tasks for a choregroup.
func (r *Repository) ListAllTasksForChoreGroup(ctx context.Context, choregroupID uuid.UUID) ([]model.Task, error) {
	query := `
		SELECT id, choregroup_id, title, type, points_reward, is_mandatory, assigned_to_user_id, status, expires_at, parent_task_id, icon_url
		FROM tasks
		WHERE choregroup_id = $1
	`
	rows, err := r.db.Query(ctx, query, choregroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var task model.Task
		if err := rows.Scan(&task.ID, &task.ChoreGroupID, &task.Title, &task.Type, &task.PointsReward, &task.IsMandatory, &task.AssignedToUserID, &task.Status, &task.ExpiresAt, &task.ParentTaskID, &task.IconURL); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// GetUsersSortedByPoints retrieves users for a specific choregroup, sorted by points.
func (r *Repository) GetUsersSortedByPoints(ctx context.Context, choregroupID uuid.UUID) ([]model.User, error) {
	rows, err := r.db.Query(ctx, "SELECT id, choregroup_id, username, role, points, notifications_viewed, has_claimed_welcome_spin, profile_image_url, points_background, unlocked_backgrounds, avatar_frame, unlocked_avatar_frames FROM users WHERE choregroup_id = $1 ORDER BY points DESC", choregroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var user model.User
		var unlockedRaw []byte
		var unlockedFramesRaw []byte
		if err := rows.Scan(&user.ID, &user.ChoreGroupID, &user.Username, &user.Role, &user.Points, &user.NotificationsViewed, &user.HasClaimedWelcomeSpin, &user.ProfileImageURL, &user.PointsBackground, &unlockedRaw, &user.AvatarFrame, &unlockedFramesRaw); err != nil {
			return nil, err
		}
		user.UnlockedBackgrounds = parseUnlockedBackgrounds(unlockedRaw)
		user.UnlockedAvatarFrames = parseUnlockedFrames(unlockedFramesRaw)
		if user.PointsBackground == "" {
			user.PointsBackground = "default"
		}
		if user.AvatarFrame == "" {
			user.AvatarFrame = "none"
		}
		sanitizeUserCosmetics(&user)
		users = append(users, user)
	}
	return users, nil
}

// MarkNotificationsViewed marks notifications_viewed to true for a user.
func (r *Repository) MarkNotificationsViewed(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "UPDATE users SET notifications_viewed = TRUE WHERE id = $1", userID)
	return err
}

// GetTask retrieves a task by ID and choregroupID.
func (r *Repository) GetTask(ctx context.Context, id, choregroupID uuid.UUID) (*model.Task, error) {
	var task model.Task
	err := r.db.QueryRow(ctx, "SELECT id, choregroup_id, title, type, points_reward, is_mandatory, assigned_to_user_id, status, expires_at, parent_task_id, icon_url FROM tasks WHERE id = $1 AND choregroup_id = $2", id, choregroupID).Scan(
		&task.ID, &task.ChoreGroupID, &task.Title, &task.Type, &task.PointsReward, &task.IsMandatory, &task.AssignedToUserID, &task.Status, &task.ExpiresAt, &task.ParentTaskID, &task.IconURL)
	return &task, err
}

// UpdatePoints updates user or group points based on task type.
func (r *Repository) UpdatePoints(ctx context.Context, choregroupID, userID uuid.UUID, points int, taskType string) error {
	if taskType == "cooperative" {
		_, err := r.db.Exec(ctx, "UPDATE choregroups SET cooperative_points = cooperative_points + $1 WHERE id = $2", points, choregroupID)
		return err
	}
	_, err := r.db.Exec(ctx, "UPDATE users SET points = points + $1 WHERE id = $2", points, userID)
	return err
}

// CreateTaskSubmission creates a new task submission.
func (r *Repository) CreateTaskSubmission(ctx context.Context, submission *model.TaskSubmission) error {
	_, err := r.db.Exec(ctx, "INSERT INTO task_submissions (id, task_id, submitted_by, status) VALUES ($1, $2, $3, $4)",
		submission.ID, submission.TaskID, submission.SubmittedBy, submission.Status)
	return err
}

// GetTaskSubmission retrieves a task submission by ID and choregroupID.
func (r *Repository) GetTaskSubmission(ctx context.Context, id, choregroupID uuid.UUID) (*model.TaskSubmission, error) {
	var submission model.TaskSubmission
	err := r.db.QueryRow(ctx, `
		SELECT s.id, s.task_id, s.submitted_by, s.status, s.created_at 
		FROM task_submissions s
		JOIN tasks t ON s.task_id = t.id
		WHERE s.id = $1 AND t.choregroup_id = $2`, id, choregroupID).Scan(
		&submission.ID, &submission.TaskID, &submission.SubmittedBy, &submission.Status, &submission.CreatedAt)
	return &submission, err
}

// GetLatestPendingSubmissionForTask retrieves the most recent pending submission for a given task and choregroupID.
func (r *Repository) GetLatestPendingSubmissionForTask(ctx context.Context, taskID, choregroupID uuid.UUID) (*model.TaskSubmission, error) {
	var submission model.TaskSubmission
	err := r.db.QueryRow(ctx, `
		SELECT s.id, s.task_id, s.submitted_by, s.status, s.created_at
		FROM task_submissions s
		JOIN tasks t ON s.task_id = t.id
		WHERE s.task_id = $1 AND t.choregroup_id = $2 AND s.status = 'pending_approval'
		ORDER BY s.created_at DESC
		LIMIT 1
	`, taskID, choregroupID).Scan(&submission.ID, &submission.TaskID, &submission.SubmittedBy, &submission.Status, &submission.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &submission, nil
}

// UpdateTaskSubmissionStatus updates the status of a task submission, verified by choregroupID.
func (r *Repository) UpdateTaskSubmissionStatus(ctx context.Context, id, choregroupID uuid.UUID, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE task_submissions 
		SET status = $1 
		WHERE id = $2 AND task_id IN (SELECT id FROM tasks WHERE choregroup_id = $3)`, status, id, choregroupID)
	return err
}

// ListSubmissions retrieves submissions for a choregroup, filtered by status.
func (r *Repository) ListSubmissions(ctx context.Context, choregroupID uuid.UUID, status string) ([]model.TaskSubmission, error) {
	query := `
		SELECT s.id, s.task_id, s.submitted_by, s.status, s.created_at
		FROM task_submissions s
		JOIN tasks t ON s.task_id = t.id
		WHERE t.choregroup_id = $1 AND s.status = $2
	`
	rows, err := r.db.Query(ctx, query, choregroupID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var submissions []model.TaskSubmission
	for rows.Next() {
		var sub model.TaskSubmission
		if err := rows.Scan(&sub.ID, &sub.TaskID, &sub.SubmittedBy, &sub.Status, &sub.CreatedAt); err != nil {
			return nil, err
		}
		submissions = append(submissions, sub)
	}
	return submissions, nil
}

// --- Reward Functions ---

func (r *Repository) CreateReward(ctx context.Context, reward *model.Reward) error {
	_, err := r.db.Exec(ctx, "INSERT INTO rewards (id, choregroup_id, name, description, cost, type, assigned_to_user_id, icon_url) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		reward.ID, reward.ChoreGroupID, reward.Name, reward.Description, reward.Cost, reward.Type, reward.AssignedToUserID, reward.IconURL)
	return err
}

func (r *Repository) GetReward(ctx context.Context, rewardID, choregroupID uuid.UUID) (model.Reward, error) {
	var reward model.Reward
	err := r.db.QueryRow(ctx, "SELECT id, choregroup_id, name, description, cost, type, assigned_to_user_id, icon_url FROM rewards WHERE id = $1 AND choregroup_id = $2", rewardID, choregroupID).Scan(
		&reward.ID, &reward.ChoreGroupID, &reward.Name, &reward.Description, &reward.Cost, &reward.Type, &reward.AssignedToUserID, &reward.IconURL)
	return reward, err
}

func (r *Repository) ListRewards(ctx context.Context, choregroupID uuid.UUID) ([]model.Reward, error) {
	rows, err := r.db.Query(ctx, "SELECT id, choregroup_id, name, description, cost, type, assigned_to_user_id, icon_url FROM rewards WHERE choregroup_id = $1", choregroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rewards []model.Reward
	for rows.Next() {
		var reward model.Reward
		if err := rows.Scan(&reward.ID, &reward.ChoreGroupID, &reward.Name, &reward.Description, &reward.Cost, &reward.Type, &reward.AssignedToUserID, &reward.IconURL); err != nil {
			return nil, err
		}
		rewards = append(rewards, reward)
	}
	return rewards, nil
}

func (r *Repository) UpdateReward(ctx context.Context, rewardID, choregroupID uuid.UUID, req model.CreateRewardRequest) error {
	_, err := r.db.Exec(ctx, "UPDATE rewards SET name = $1, description = $2, cost = $3, type = $4, assigned_to_user_id = $5, icon_url = $6 WHERE id = $7 AND choregroup_id = $8",
		req.Name, req.Description, req.Cost, req.Type, req.AssignedToUserID, req.IconURL, rewardID, choregroupID)
	return err
}

func (r *Repository) DeleteReward(ctx context.Context, rewardID, choregroupID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM rewards WHERE id = $1 AND choregroup_id = $2", rewardID, choregroupID)
	return err
}

func (r *Repository) CreateRewardPurchase(ctx context.Context, purchase *model.RewardPurchase) error {
	approvalsJSON, err := json.Marshal(purchase.Approvals)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, "INSERT INTO reward_purchases (id, reward_id, purchased_by_user_id, status, approvals) VALUES ($1, $2, $3, $4, $5)",
		purchase.ID, purchase.RewardID, purchase.PurchasedByUserID, purchase.Status, approvalsJSON)
	return err
}

func (r *Repository) GetRewardPurchase(ctx context.Context, purchaseID, choregroupID uuid.UUID) (model.RewardPurchase, error) {
	var purchase model.RewardPurchase
	err := r.db.QueryRow(ctx, `
		SELECT p.id, p.reward_id, p.purchased_by_user_id, p.status, p.approvals 
		FROM reward_purchases p
		JOIN rewards r ON p.reward_id = r.id
		WHERE p.id = $1 AND r.choregroup_id = $2`, purchaseID, choregroupID).Scan(
		&purchase.ID, &purchase.RewardID, &purchase.PurchasedByUserID, &purchase.Status, &purchase.Approvals)
	return purchase, err
}

func (r *Repository) ListRewardPurchases(ctx context.Context, choregroupID uuid.UUID, status string) ([]model.RewardPurchase, error) {
	var rows pgx.Rows
	var err error
	if status == "" || status == "all" {
		rows, err = r.db.Query(ctx, `
			SELECT p.id, p.reward_id, p.purchased_by_user_id, p.status, p.approvals
			FROM reward_purchases p
			JOIN rewards r ON p.reward_id = r.id
			WHERE r.choregroup_id = $1
		`, choregroupID)
	} else {
		rows, err = r.db.Query(ctx, `
			SELECT p.id, p.reward_id, p.purchased_by_user_id, p.status, p.approvals
			FROM reward_purchases p
			JOIN rewards r ON p.reward_id = r.id
			WHERE r.choregroup_id = $1 AND p.status = $2
		`, choregroupID, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var purchases []model.RewardPurchase
	for rows.Next() {
		var purchase model.RewardPurchase
		if err := rows.Scan(&purchase.ID, &purchase.RewardID, &purchase.PurchasedByUserID, &purchase.Status, &purchase.Approvals); err != nil {
			return nil, err
		}
		purchases = append(purchases, purchase)
	}
	return purchases, nil
}

func (r *Repository) UpdateRewardPurchase(ctx context.Context, purchase *model.RewardPurchase, choregroupID uuid.UUID) error {
	approvalsJSON, err := json.Marshal(purchase.Approvals)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		UPDATE reward_purchases 
		SET status = $1, approvals = $2 
		WHERE id = $3 AND reward_id IN (SELECT id FROM rewards WHERE choregroup_id = $4)`,
		purchase.Status, approvalsJSON, purchase.ID, choregroupID)
	return err
}

func (r *Repository) RefundCooperativePoints(ctx context.Context, choregroupID uuid.UUID, amount int) error {
	_, err := r.db.Exec(ctx, "UPDATE choregroups SET cooperative_points = cooperative_points + $1 WHERE id = $2", amount, choregroupID)
	return err
}

func (r *Repository) DeleteRewardPurchase(ctx context.Context, purchaseID, choregroupID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM reward_purchases 
		WHERE id = $1 AND reward_id IN (SELECT id FROM rewards WHERE choregroup_id = $2)`, purchaseID, choregroupID)
	return err
}

func (r *Repository) GetEmojiForTitle(ctx context.Context, title string) (string, error) {
	var emoji string
	err := r.db.QueryRow(ctx, `
		SELECT emoji FROM icon_mappings 
		WHERE keyword = LOWER(TRIM($1)) 
		   OR keyword = ANY(string_to_array(LOWER(TRIM($1)), ' ')) 
		LIMIT 1`, title).Scan(&emoji)
	return emoji, err
}

func (r *Repository) SaveIconMapping(ctx context.Context, keyword, emoji string) error {
	_, err := r.db.Exec(ctx, "INSERT INTO icon_mappings (keyword, emoji) VALUES ($1, $2) ON CONFLICT (keyword) DO NOTHING", keyword, emoji)
	return err
}

func (r *Repository) UpdateTaskTitle(ctx context.Context, taskID uuid.UUID, title string) error {
	_, err := r.db.Exec(ctx, "UPDATE tasks SET title = $1 WHERE id = $2", title, taskID)
	return err
}

func (r *Repository) UpdateRewardName(ctx context.Context, rewardID uuid.UUID, name string) error {
	_, err := r.db.Exec(ctx, "UPDATE rewards SET name = $1 WHERE id = $2", name, rewardID)
	return err
}

// GetChoreGroupByName retrieves a choregroup by its name.
func (r *Repository) GetChoreGroupByName(ctx context.Context, name string) (model.ChoreGroup, error) {
	var group model.ChoreGroup
	err := r.db.QueryRow(ctx, "SELECT id, name, cooperative_points, COALESCE(ai_language, 'he'), COALESCE(ai_instructions, '') FROM choregroups WHERE name = $1", name).Scan(
		&group.ID, &group.Name, &group.CooperativePoints, &group.AILanguage, &group.AIInstructions)
	return group, err
}

// UpdateChoreGroupAILanguage updates the household's AI content language.
func (r *Repository) UpdateChoreGroupAILanguage(ctx context.Context, choregroupID uuid.UUID, language string) error {
	_, err := r.db.Exec(ctx, "UPDATE choregroups SET ai_language = $1 WHERE id = $2", language, choregroupID)
	return err
}

// UpdateChoreGroupAIInstructions updates the household's custom instructions for AI chore analysis and rewards.
func (r *Repository) UpdateChoreGroupAIInstructions(ctx context.Context, choregroupID uuid.UUID, instructions string) error {
	_, err := r.db.Exec(ctx, "UPDATE choregroups SET ai_instructions = $1 WHERE id = $2", instructions, choregroupID)
	return err
}

// GetUserByChoreGroupIDAndUsername retrieves a user by their choregroupID and username.
func (r *Repository) GetUserByChoreGroupIDAndUsername(ctx context.Context, choregroupID uuid.UUID, username string) (model.User, error) {
	var user model.User
	var unlockedRaw []byte
	var unlockedFramesRaw []byte
	err := r.db.QueryRow(ctx, "SELECT id, choregroup_id, username, password_hash, role, points, notifications_viewed, has_claimed_welcome_spin, profile_image_url, points_background, unlocked_backgrounds, avatar_frame, unlocked_avatar_frames FROM users WHERE choregroup_id = $1 AND username = $2", choregroupID, username).Scan(
		&user.ID, &user.ChoreGroupID, &user.Username, &user.PasswordHash, &user.Role, &user.Points, &user.NotificationsViewed, &user.HasClaimedWelcomeSpin, &user.ProfileImageURL, &user.PointsBackground, &unlockedRaw, &user.AvatarFrame, &unlockedFramesRaw)
	user.UnlockedBackgrounds = parseUnlockedBackgrounds(unlockedRaw)
	user.UnlockedAvatarFrames = parseUnlockedFrames(unlockedFramesRaw)
	if user.PointsBackground == "" {
		user.PointsBackground = "default"
	}
	if user.AvatarFrame == "" {
		user.AvatarFrame = "none"
	}
	sanitizeUserCosmetics(&user)
	return user, err
}

// UpdateUserProfileImage updates the profile image URL for a user.
func (r *Repository) UpdateUserProfileImage(ctx context.Context, userID uuid.UUID, profileImageURL *string) error {
	_, err := r.db.Exec(ctx, "UPDATE users SET profile_image_url = $1 WHERE id = $2", profileImageURL, userID)
	return err
}

// UpdateUserPointsBackground updates the active points background for a user.
func (r *Repository) UpdateUserPointsBackground(ctx context.Context, userID uuid.UUID, background string, bypassUnlockCheck ...bool) error {
	isValid := false
	for _, b := range AllPointsBackgrounds {
		if b == background {
			isValid = true
			break
		}
	}
	if !isValid {
		return errors.New("invalid background theme")
	}

	var userRole string
	var unlockedRaw []byte
	err := r.db.QueryRow(ctx, "SELECT role, unlocked_backgrounds FROM users WHERE id = $1", userID).Scan(&userRole, &unlockedRaw)
	if err != nil {
		return err
	}

	isUnlocked := (userRole == "admin") || (len(bypassUnlockCheck) > 0 && bypassUnlockCheck[0])
	if !isUnlocked {
		unlocked := parseUnlockedBackgrounds(unlockedRaw)
		for _, b := range unlocked {
			if b == background {
				isUnlocked = true
				break
			}
		}
	}
	if !isUnlocked {
		return errors.New("background is not unlocked")
	}

	_, err = r.db.Exec(ctx, "UPDATE users SET points_background = $1 WHERE id = $2", background, userID)
	return err
}

// UpdateUserAvatarFrame updates the equipped avatar frame for a user.
func (r *Repository) UpdateUserAvatarFrame(ctx context.Context, userID uuid.UUID, frame string, bypassUnlockCheck ...bool) error {
	isValid := false
	for _, f := range AllAvatarFrames {
		if f == frame {
			isValid = true
			break
		}
	}
	if !isValid {
		return errors.New("invalid avatar frame")
	}

	var userRole string
	var unlockedRaw []byte
	err := r.db.QueryRow(ctx, "SELECT role, unlocked_avatar_frames FROM users WHERE id = $1", userID).Scan(&userRole, &unlockedRaw)
	if err != nil {
		return err
	}

	isUnlocked := (userRole == "admin") || (frame == "none") || (len(bypassUnlockCheck) > 0 && bypassUnlockCheck[0])
	if !isUnlocked {
		unlocked := parseUnlockedFrames(unlockedRaw)
		for _, f := range unlocked {
			if f == frame {
				isUnlocked = true
				break
			}
		}
	}
	if !isUnlocked {
		return errors.New("avatar frame is not unlocked")
	}

	_, err = r.db.Exec(ctx, "UPDATE users SET avatar_frame = $1 WHERE id = $2", frame, userID)
	return err
}

// SavePushSubscription upserts a web push subscription for a user.
func (r *Repository) SavePushSubscription(ctx context.Context, userID uuid.UUID, endpoint, p256dh, auth string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth)
		 VALUES (uuid_generate_v4(), $1, $2, $3, $4)
		 ON CONFLICT (endpoint) DO UPDATE SET user_id = $1, p256dh = $3, auth = $4`,
		userID, endpoint, p256dh, auth)
	return err
}

// GetPushSubscriptionsByChoreGroup returns all push subscriptions for kid users in a choregroup.
func (r *Repository) GetPushSubscriptionsByChoreGroup(ctx context.Context, choregroupID uuid.UUID) ([]model.PushSubscription, error) {
	rows, err := r.db.Query(ctx,
		`SELECT ps.id, ps.user_id, ps.endpoint, ps.p256dh, ps.auth, ps.created_at
		 FROM push_subscriptions ps
		 JOIN users u ON ps.user_id = u.id
		 WHERE u.choregroup_id = $1 AND u.role = 'user'`, choregroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []model.PushSubscription
	for rows.Next() {
		var sub model.PushSubscription
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256dh, &sub.Auth, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// DeletePushSubscription removes a push subscription by endpoint.
func (r *Repository) DeletePushSubscription(ctx context.Context, endpoint string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM push_subscriptions WHERE endpoint = $1", endpoint)
	return err
}

var ErrNoSpinsAvailable = errors.New("no spins available")

// GetUserDailyProgress retrieves today's progress for a user, creating an initial record if not present.
func (r *Repository) GetUserDailyProgress(ctx context.Context, choregroupID, userID uuid.UUID) (*model.UserDailyProgress, error) {
	var p model.UserDailyProgress
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, choregroup_id, date, tasks_completed, spins_earned, spins_used, double_coins_active, double_coins_used, surprise_claimed
		 FROM user_daily_progress
		 WHERE choregroup_id = $1 AND user_id = $2 AND date = CURRENT_DATE`,
		choregroupID, userID,
	).Scan(&p.ID, &p.UserID, &p.ChoreGroupID, &p.Date, &p.TasksCompleted, &p.SpinsEarned, &p.SpinsUsed, &p.DoubleCoinsActive, &p.DoubleCoinsUsed, &p.SurpriseClaimed)

	if errors.Is(err, pgx.ErrNoRows) {
		id := uuid.New()
		err = r.db.QueryRow(ctx,
			`INSERT INTO user_daily_progress (id, user_id, choregroup_id, date, tasks_completed, spins_earned, spins_used, double_coins_active, double_coins_used, surprise_claimed)
			 VALUES ($1, $2, $3, CURRENT_DATE, 0, 0, 0, FALSE, FALSE, FALSE)
			 ON CONFLICT (user_id, date) DO UPDATE SET choregroup_id = EXCLUDED.choregroup_id
			 RETURNING id, user_id, choregroup_id, date, tasks_completed, spins_earned, spins_used, double_coins_active, double_coins_used, surprise_claimed`,
			id, userID, choregroupID,
		).Scan(&p.ID, &p.UserID, &p.ChoreGroupID, &p.Date, &p.TasksCompleted, &p.SpinsEarned, &p.SpinsUsed, &p.DoubleCoinsActive, &p.DoubleCoinsUsed, &p.SurpriseClaimed)
		if err != nil {
			return nil, err
		}
		return &p, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// RecordDailyTaskCompletion increments the completed tasks count for today and triggers milestone rewards (milestone 1: 1st spin, milestone 5: 2nd spin, milestone 8: 3rd spin).
func (r *Repository) RecordDailyTaskCompletion(ctx context.Context, choregroupID, userID uuid.UUID) error {
	// Ensure today's record exists first
	_, err := r.GetUserDailyProgress(ctx, choregroupID, userID)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx,
		`UPDATE user_daily_progress
		 SET tasks_completed = tasks_completed + 1,
		     spins_earned = CASE
		         WHEN tasks_completed + 1 >= 10 AND spins_earned < 4 THEN 4
		         WHEN tasks_completed + 1 >= 7 AND spins_earned < 3 THEN 3
		         WHEN tasks_completed + 1 >= 4 AND spins_earned < 2 THEN 2
		         WHEN tasks_completed + 1 >= 1 AND spins_earned < 1 THEN 1
		         ELSE spins_earned
		     END,
		     double_coins_active = FALSE
		 WHERE choregroup_id = $1 AND user_id = $2 AND date = CURRENT_DATE`,
		choregroupID, userID,
	)
	return err
}

// CheckAndConsumeDoubleCoins checks if today's double coins bonus is active, and if so, consumes it and returns true.
func (r *Repository) CheckAndConsumeDoubleCoins(ctx context.Context, choregroupID, userID uuid.UUID) (bool, error) {
	var consumed bool
	err := r.db.QueryRow(ctx,
		`UPDATE user_daily_progress
		 SET double_coins_active = FALSE, double_coins_used = TRUE
		 WHERE choregroup_id = $1 AND user_id = $2 AND date = CURRENT_DATE AND double_coins_active = TRUE
		 RETURNING TRUE`,
		choregroupID, userID,
	).Scan(&consumed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return consumed, nil
}

// ClaimWheelSpin validates and executes a wheel spin in a single transaction, consuming the spin and applying any won points.
func (r *Repository) ClaimWheelSpin(ctx context.Context, choregroupID, userID uuid.UUID, sliceIndex int, label string, pointsWon int, pointsType string) (*model.WheelSpinResponse, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Verify user role and welcome spin status
	var role string
	var hasClaimedWelcomeSpin bool
	var currentPoints int
	err = tx.QueryRow(ctx,
		"SELECT role, has_claimed_welcome_spin, points FROM users WHERE id = $1 AND choregroup_id = $2 FOR UPDATE",
		userID, choregroupID,
	).Scan(&role, &hasClaimedWelcomeSpin, &currentPoints)
	if err != nil {
		return nil, err
	}

	if role != "user" {
		return nil, errors.New("only kid users can spin the wheel")
	}

	// Fetch or initialize today's daily progress
	var spinsEarned, spinsUsed int
	err = tx.QueryRow(ctx,
		`SELECT spins_earned, spins_used FROM user_daily_progress
		 WHERE choregroup_id = $1 AND user_id = $2 AND date = CURRENT_DATE FOR UPDATE`,
		choregroupID, userID,
	).Scan(&spinsEarned, &spinsUsed)
	if errors.Is(err, pgx.ErrNoRows) {
		id := uuid.New()
		err = tx.QueryRow(ctx,
			`INSERT INTO user_daily_progress (id, user_id, choregroup_id, date, tasks_completed, spins_earned, spins_used, double_coins_active, double_coins_used)
			 VALUES ($1, $2, $3, CURRENT_DATE, 0, 0, 0, FALSE, FALSE)
			 ON CONFLICT (user_id, date) DO UPDATE SET choregroup_id = EXCLUDED.choregroup_id
			 RETURNING spins_earned, spins_used`,
			id, userID, choregroupID,
		).Scan(&spinsEarned, &spinsUsed)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	totalSpinsAvailable := spinsEarned - spinsUsed
	if totalSpinsAvailable <= 0 {
		return nil, ErrNoSpinsAvailable
	}

	// Mark welcome spin as claimed on user record if not yet set
	if !hasClaimedWelcomeSpin {
		_, err = tx.Exec(ctx, "UPDATE users SET has_claimed_welcome_spin = TRUE WHERE id = $1", userID)
		if err != nil {
			return nil, err
		}
	}

	// Consume spin in daily progress
	_, err = tx.Exec(ctx,
		`UPDATE user_daily_progress
		 SET spins_used = spins_used + 1
		 WHERE choregroup_id = $1 AND user_id = $2 AND date = CURRENT_DATE`,
		choregroupID, userID,
	)
	if err != nil {
		return nil, err
	}
	spinsUsed++

	// Award coins if points won, or unlock background / avatar frame if surprise
	var surpriseBackground *string
	var surpriseFrame *string
	if pointsType == "surprise" {
		var unlockedRaw, unlockedFramesRaw []byte
		_ = tx.QueryRow(ctx, "SELECT unlocked_backgrounds, unlocked_avatar_frames FROM users WHERE id = $1", userID).Scan(&unlockedRaw, &unlockedFramesRaw)
		unlockedThemes := parseUnlockedBackgrounds(unlockedRaw)
		unlockedFrames := parseUnlockedFrames(unlockedFramesRaw)

		surpriseThemes := []string{"cherry_blossom", "cosmic_galaxy", "emerald_nature", "candy_party", "fire_dragon"}
		surpriseFrames := []string{"golden_sparkle", "fire_flame", "cosmic_nebula", "emerald_nature", "rainbow_wave", "royal_crown", "neon_cyber"}

		type cosmeticOpt struct {
			kind string
			id   string
		}
		var available []cosmeticOpt

		for _, st := range surpriseThemes {
			owned := false
			for _, u := range unlockedThemes {
				if u == st {
					owned = true
					break
				}
			}
			if !owned {
				available = append(available, cosmeticOpt{kind: "theme", id: st})
			}
		}

		for _, sf := range surpriseFrames {
			owned := false
			for _, u := range unlockedFrames {
				if u == sf {
					owned = true
					break
				}
			}
			if !owned {
				available = append(available, cosmeticOpt{kind: "frame", id: sf})
			}
		}

		if len(available) > 0 {
			chosen := available[rand.IntN(len(available))]
			if chosen.kind == "theme" {
				unlockedThemes = append(unlockedThemes, chosen.id)
				newUnlockedJSON, _ := json.Marshal(unlockedThemes)
				_, err = tx.Exec(ctx, "UPDATE users SET unlocked_backgrounds = $1, points_background = $2 WHERE id = $3", newUnlockedJSON, chosen.id, userID)
				if err != nil {
					return nil, err
				}
				surpriseBackground = &chosen.id
			} else {
				unlockedFrames = append(unlockedFrames, chosen.id)
				newUnlockedJSON, _ := json.Marshal(unlockedFrames)
				_, err = tx.Exec(ctx, "UPDATE users SET unlocked_avatar_frames = $1, avatar_frame = $2 WHERE id = $3", newUnlockedJSON, chosen.id, userID)
				if err != nil {
					return nil, err
				}
				surpriseFrame = &chosen.id
			}
		} else {
			// All surprise cosmetics unlocked! Award 3 bonus coins
			pointsWon = 3
			pointsType = "individual"
			label = "📦 +3 Bonus Coins!"
			_, err = tx.Exec(ctx, "UPDATE users SET points = points + 3 WHERE id = $1", userID)
			if err != nil {
				return nil, err
			}
		}
	} else if pointsWon > 0 {
		if pointsType == "individual" {
			_, err = tx.Exec(ctx, "UPDATE users SET points = points + $1 WHERE id = $2", pointsWon, userID)
			if err != nil {
				return nil, err
			}
		} else if pointsType == "cooperative" {
			_, err = tx.Exec(ctx, "UPDATE choregroups SET cooperative_points = cooperative_points + $1 WHERE id = $2", pointsWon, choregroupID)
			if err != nil {
				return nil, err
			}
		}
	}

	// Read fresh points
	var newUserPoints, newCoopPoints int
	err = tx.QueryRow(ctx, "SELECT points FROM users WHERE id = $1", userID).Scan(&newUserPoints)
	if err != nil {
		return nil, err
	}
	err = tx.QueryRow(ctx, "SELECT cooperative_points FROM choregroups WHERE id = $1", choregroupID).Scan(&newCoopPoints)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	spinsRemaining := spinsEarned - spinsUsed
	if spinsRemaining < 0 {
		spinsRemaining = 0
	}

	return &model.WheelSpinResponse{
		SliceIndex:         sliceIndex,
		Label:              label,
		RewardType:         pointsType,
		Amount:             pointsWon,
		UserPoints:         newUserPoints,
		CooperativePoints:  newCoopPoints,
		SpinsRemaining:     spinsRemaining,
		SurpriseBackground: surpriseBackground,
		SurpriseFrame:      surpriseFrame,
	}, nil
}

// LogActivity persists an event into the activity_history table.
func (r *Repository) LogActivity(ctx context.Context, event model.ActivityEvent) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO activity_history (
			id, choregroup_id, user_id, user_username, target_user_id, target_username,
			activity_type, title, description, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, event.ID, event.ChoreGroupID, event.UserID, event.UserUsername, event.TargetUserID, event.TargetUsername,
		event.ActivityType, event.Title, event.Description, metadataJSON, event.CreatedAt)
	return err
}

// PurgeOldActivityHistory deletes activity history records older than retentionDays (defaults to 7 days).
func (r *Repository) PurgeOldActivityHistory(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 7
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	tag, err := r.db.Exec(ctx, `
		DELETE FROM activity_history
		WHERE created_at < $1
	`, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// GetRecentActivity retrieves the most recent activity items for a household or user within the 7-day retention window.
func (r *Repository) GetRecentActivity(ctx context.Context, choregroupID uuid.UUID, userID *uuid.UUID, limit int) ([]model.ActivityEvent, error) {
	if limit <= 0 {
		limit = 3
	}

	var rows pgx.Rows
	var err error

	if userID != nil {
		rows, err = r.db.Query(ctx, `
			SELECT a.id, a.choregroup_id, a.user_id, a.user_username, a.target_user_id, a.target_username,
			       a.activity_type, a.title, a.description, a.metadata, a.created_at,
			       u.profile_image_url, COALESCE(u.role, 'user')
			FROM activity_history a
			LEFT JOIN users u ON a.user_id = u.id
			WHERE a.choregroup_id = $1
			  AND a.user_id = $2
			  AND a.created_at >= NOW() - INTERVAL '7 days'
			ORDER BY a.created_at DESC
			LIMIT $3
		`, choregroupID, *userID, limit)
	} else {
		rows, err = r.db.Query(ctx, `
			SELECT a.id, a.choregroup_id, a.user_id, a.user_username, a.target_user_id, a.target_username,
			       a.activity_type, a.title, a.description, a.metadata, a.created_at,
			       u.profile_image_url, COALESCE(u.role, 'user')
			FROM activity_history a
			LEFT JOIN users u ON a.user_id = u.id
			WHERE a.choregroup_id = $1
			  AND a.created_at >= NOW() - INTERVAL '7 days'
			ORDER BY a.created_at DESC
			LIMIT $2
		`, choregroupID, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []model.ActivityEvent{}
	for rows.Next() {
		var e model.ActivityEvent
		var metaRaw []byte
		if err := rows.Scan(&e.ID, &e.ChoreGroupID, &e.UserID, &e.UserUsername, &e.TargetUserID, &e.TargetUsername,
			&e.ActivityType, &e.Title, &e.Description, &metaRaw, &e.CreatedAt, &e.UserProfileImage, &e.UserRole); err != nil {
			return nil, err
		}
		if len(metaRaw) > 0 {
			_ = json.Unmarshal(metaRaw, &e.Metadata)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetActivityHistory fetches filtered activity history with total count (enforcing a 7-day retention policy).
func (r *Repository) GetActivityHistory(ctx context.Context, choregroupID uuid.UUID, filter model.HistoryFilterParams) ([]model.ActivityEvent, int, error) {
	var args []interface{}
	args = append(args, choregroupID)
	paramIdx := 2

	whereClauses := []string{
		"a.choregroup_id = $1",
		"a.created_at >= NOW() - INTERVAL '7 days'",
	}

	if filter.UserID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.user_id = $%d", paramIdx))
		args = append(args, *filter.UserID)
		paramIdx++
	}

	if filter.StartDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.created_at >= $%d", paramIdx))
		args = append(args, *filter.StartDate)
		paramIdx++
	}

	if filter.EndDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.created_at <= $%d", paramIdx))
		args = append(args, *filter.EndDate)
		paramIdx++
	}

	if filter.ActivityCategory != "" && filter.ActivityCategory != "all" {
		switch filter.ActivityCategory {
		case "tasks", "chores":
			whereClauses = append(whereClauses, fmt.Sprintf("a.activity_type IN ('task_completed', 'task_deleted', 'chore_approved', 'chore_deleted')"))
		case "rewards":
			whereClauses = append(whereClauses, fmt.Sprintf("a.activity_type IN ('reward_obtained', 'reward_created', 'reward_deleted', 'reward_purchased', 'reward_fulfilled')"))
		case "wheel":
			whereClauses = append(whereClauses, fmt.Sprintf("a.activity_type IN ('wheel_spin', 'wheel_spun')"))
		}
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM activity_history a WHERE %s", whereSQL)
	var totalCount int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	querySQL := fmt.Sprintf(`
		SELECT a.id, a.choregroup_id, a.user_id, a.user_username, a.target_user_id, a.target_username,
		       a.activity_type, a.title, a.description, a.metadata, a.created_at,
		       u.profile_image_url, COALESCE(u.role, 'user')
		FROM activity_history a
		LEFT JOIN users u ON a.user_id = u.id
		WHERE %s
		ORDER BY a.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, paramIdx, paramIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	events := []model.ActivityEvent{}
	for rows.Next() {
		var e model.ActivityEvent
		var metaRaw []byte
		if err := rows.Scan(&e.ID, &e.ChoreGroupID, &e.UserID, &e.UserUsername, &e.TargetUserID, &e.TargetUsername,
			&e.ActivityType, &e.Title, &e.Description, &metaRaw, &e.CreatedAt, &e.UserProfileImage, &e.UserRole); err != nil {
			return nil, 0, err
		}
		if len(metaRaw) > 0 {
			_ = json.Unmarshal(metaRaw, &e.Metadata)
		}
		events = append(events, e)
	}

	return events, totalCount, rows.Err()
}

// BackfillInitialActivityHistory populates history from existing tasks, submissions, and purchases if empty.
func (r *Repository) BackfillInitialActivityHistory(ctx context.Context) error {
	var count int
	_ = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM activity_history").Scan(&count)
	if count > 0 {
		return nil
	}

	// Backfill completed task submissions
	_, _ = r.db.Exec(ctx, `
		INSERT INTO activity_history (choregroup_id, user_id, user_username, activity_type, title, description, metadata, created_at)
		SELECT 
			t.choregroup_id,
			s.submitted_by,
			COALESCE(u.username, 'User'),
			'task_completed',
			'Task Completed',
			t.title,
			json_build_object('task_id', t.id, 'title', t.title, 'points', t.points_reward),
			COALESCE(s.created_at, NOW())
		FROM task_submissions s
		JOIN tasks t ON s.task_id = t.id
		LEFT JOIN users u ON s.submitted_by = u.id
		WHERE s.status = 'approved'
	`)

	// Backfill reward purchases
	_, _ = r.db.Exec(ctx, `
		INSERT INTO activity_history (choregroup_id, user_id, user_username, activity_type, title, description, metadata, created_at)
		SELECT
			r.choregroup_id,
			p.purchased_by_user_id,
			COALESCE(u.username, 'User'),
			'reward_obtained',
			'Reward Obtained',
			r.name,
			json_build_object('reward_id', r.id, 'name', r.name, 'cost', r.cost),
			NOW()
		FROM reward_purchases p
		JOIN rewards r ON p.reward_id = r.id
		LEFT JOIN users u ON p.purchased_by_user_id = u.id
	`)

	return nil
}

// ClaimDailySurprise validates and claims the 10-task daily milestone surprise.
func (r *Repository) ClaimDailySurprise(ctx context.Context, choregroupID, userID uuid.UUID) (*model.DailySurpriseResponse, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var role, username string
	var currentPoints int
	err = tx.QueryRow(ctx,
		"SELECT role, username, points FROM users WHERE id = $1 AND choregroup_id = $2 FOR UPDATE",
		userID, choregroupID,
	).Scan(&role, &username, &currentPoints)
	if err != nil {
		return nil, err
	}

	if role != "user" {
		return nil, errors.New("only kid users can claim the daily surprise")
	}

	var tasksCompleted int
	var surpriseClaimed bool
	err = tx.QueryRow(ctx,
		`SELECT tasks_completed, surprise_claimed FROM user_daily_progress
		 WHERE choregroup_id = $1 AND user_id = $2 AND date = CURRENT_DATE FOR UPDATE`,
		choregroupID, userID,
	).Scan(&tasksCompleted, &surpriseClaimed)
	if errors.Is(err, pgx.ErrNoRows) || tasksCompleted < 10 {
		return nil, errors.New("you need to complete at least 10 daily tasks to claim the surprise")
	}
	if err != nil {
		return nil, err
	}
	if surpriseClaimed {
		return nil, errors.New("today's daily surprise has already been claimed")
	}

	// Mark surprise as claimed
	_, err = tx.Exec(ctx,
		`UPDATE user_daily_progress
		 SET surprise_claimed = TRUE
		 WHERE choregroup_id = $1 AND user_id = $2 AND date = CURRENT_DATE`,
		choregroupID, userID,
	)
	if err != nil {
		return nil, err
	}

	var unlockedRaw []byte
	var unlockedFramesRaw []byte
	_ = tx.QueryRow(ctx, "SELECT unlocked_backgrounds, unlocked_avatar_frames FROM users WHERE id = $1", userID).Scan(&unlockedRaw, &unlockedFramesRaw)
	unlockedThemes := parseUnlockedBackgrounds(unlockedRaw)
	unlockedFrames := parseUnlockedFrames(unlockedFramesRaw)

	surpriseThemes := []string{"cherry_blossom", "cosmic_galaxy", "emerald_nature", "candy_party", "fire_dragon"}
	surpriseFrames := []string{"golden_sparkle", "fire_flame", "cosmic_nebula", "emerald_nature", "rainbow_wave", "royal_crown", "neon_cyber"}

	type cosmeticOpt struct {
		kind string
		id   string
	}
	var available []cosmeticOpt

	for _, st := range surpriseThemes {
		owned := false
		for _, u := range unlockedThemes {
			if u == st {
				owned = true
				break
			}
		}
		if !owned {
			available = append(available, cosmeticOpt{kind: "theme", id: st})
		}
	}

	for _, sf := range surpriseFrames {
		owned := false
		for _, u := range unlockedFrames {
			if u == sf {
				owned = true
				break
			}
		}
		if !owned {
			available = append(available, cosmeticOpt{kind: "frame", id: sf})
		}
	}

	var chosenTheme *string
	var chosenFrame *string
	bonusCoins := 0
	newUserPoints := currentPoints
	message := ""

	if len(available) > 0 {
		chosen := available[rand.IntN(len(available))]
		if chosen.kind == "theme" {
			unlockedThemes = append(unlockedThemes, chosen.id)
			newUnlockedJSON, _ := json.Marshal(unlockedThemes)
			_, err = tx.Exec(ctx, "UPDATE users SET unlocked_backgrounds = $1, points_background = $2 WHERE id = $3", newUnlockedJSON, chosen.id, userID)
			if err != nil {
				return nil, err
			}
			chosenTheme = &chosen.id
			message = "Unlocked new theme!"
		} else {
			unlockedFrames = append(unlockedFrames, chosen.id)
			newUnlockedJSON, _ := json.Marshal(unlockedFrames)
			_, err = tx.Exec(ctx, "UPDATE users SET unlocked_avatar_frames = $1, avatar_frame = $2 WHERE id = $3", newUnlockedJSON, chosen.id, userID)
			if err != nil {
				return nil, err
			}
			chosenFrame = &chosen.id
			message = "Unlocked new avatar frame!"
		}
	} else {
		// All surprise cosmetics already unlocked! Grant 5 bonus coins
		bonusCoins = 5
		_, err = tx.Exec(ctx, "UPDATE users SET points = points + $1 WHERE id = $2", bonusCoins, userID)
		if err != nil {
			return nil, err
		}
		newUserPoints += bonusCoins
		message = "All cosmetics unlocked! +5 bonus coins awarded!"
	}

	// Log activity history
	title := "Daily Surprise Unlocked!"
	desc := fmt.Sprintf("%s completed 10 daily tasks and unlocked a surprise reward!", username)
	if chosenTheme != nil {
		desc = fmt.Sprintf("%s completed 10 daily tasks and unlocked the %s theme!", username, *chosenTheme)
	} else if chosenFrame != nil {
		desc = fmt.Sprintf("%s completed 10 daily tasks and unlocked the %s avatar frame!", username, *chosenFrame)
	}
	metaJSON, _ := json.Marshal(map[string]interface{}{
		"theme_id":    chosenTheme,
		"frame_id":    chosenFrame,
		"bonus_coins": bonusCoins,
	})
	_, _ = tx.Exec(ctx,
		`INSERT INTO activity_history (choregroup_id, user_id, user_username, activity_type, title, description, metadata)
		 VALUES ($1, $2, $3, 'daily_surprise', $4, $5, $6)`,
		choregroupID, userID, username, title, desc, metaJSON,
	)

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &model.DailySurpriseResponse{
		ThemeID:    chosenTheme,
		FrameID:    chosenFrame,
		BonusCoins: bonusCoins,
		UserPoints: newUserPoints,
		Message:    message,
	}, nil
}

// SaveUpload stores binary file content (BYTEA) in the PostgreSQL database repository.
func (r *Repository) SaveUpload(ctx context.Context, upload model.Upload) error {
	if upload.ID == uuid.Nil {
		upload.ID = uuid.New()
	}
	if upload.CreatedAt.IsZero() {
		upload.CreatedAt = time.Now()
	}
	if upload.SizeBytes <= 0 {
		upload.SizeBytes = int64(len(upload.Data))
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO uploads (id, filename, mime_type, data, size_bytes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			filename = EXCLUDED.filename,
			mime_type = EXCLUDED.mime_type,
			data = EXCLUDED.data,
			size_bytes = EXCLUDED.size_bytes,
			created_at = EXCLUDED.created_at
	`, upload.ID, upload.Filename, upload.MIMEType, upload.Data, upload.SizeBytes, upload.CreatedAt)
	return err
}

// GetUploadByID retrieves an upload record with its binary BYTEA data from PostgreSQL by UUID.
func (r *Repository) GetUploadByID(ctx context.Context, id uuid.UUID) (*model.Upload, error) {
	var u model.Upload
	err := r.db.QueryRow(ctx, `
		SELECT id, filename, mime_type, data, size_bytes, created_at
		FROM uploads
		WHERE id = $1
	`, id).Scan(&u.ID, &u.Filename, &u.MIMEType, &u.Data, &u.SizeBytes, &u.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// DeleteUploadByID removes an upload record from the PostgreSQL database repository by UUID.
func (r *Repository) DeleteUploadByID(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM uploads WHERE id = $1", id)
	return err
}

// MigrateDiskUploadsToDB scans local disk folders (./uploads) for existing upload files,
// imports their binary contents into the PostgreSQL uploads table, and cleans up local files.
func (r *Repository) MigrateDiskUploadsToDB(ctx context.Context) (int, error) {
	migratedCount := 0
	dirs := []string{"./uploads", "uploads", "backend/uploads"}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			filename := entry.Name()
			filePath := filepath.Join(dir, filename)

			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			ext := strings.ToLower(filepath.Ext(filename))
			mimeType := "image/jpeg"
			if ext == ".png" {
				mimeType = "image/png"
			} else if ext == ".gif" {
				mimeType = "image/gif"
			} else if ext == ".webp" {
				mimeType = "image/webp"
			} else if ext == ".svg" {
				mimeType = "image/svg+xml"
			}

			rawID := strings.TrimSuffix(filename, filepath.Ext(filename))
			fileID, err := uuid.Parse(rawID)
			if err != nil {
				fileID = uuid.New()
			}

			upload := model.Upload{
				ID:        fileID,
				Filename:  filename,
				MIMEType:  mimeType,
				Data:      data,
				SizeBytes: int64(len(data)),
				CreatedAt: time.Now(),
			}

			if err := r.SaveUpload(ctx, upload); err == nil {
				_ = os.Remove(filePath)
				migratedCount++
			}
		}
	}
	return migratedCount, nil
}

// PurgeOrphanedUploads deletes upload records from PostgreSQL repository that are older than minAge
// and are not referenced by any user avatar, task icon, reward icon, or active activity history metadata.
func (r *Repository) PurgeOrphanedUploads(ctx context.Context, minAge time.Duration) (int, error) {
	seconds := int64(minAge.Seconds())
	if seconds < 60 {
		seconds = 3600 // Default to 1 hour minimum age
	}

	query := `
		WITH orphaned AS (
			SELECT u.id
			FROM uploads u
			WHERE u.created_at < NOW() - ($1 * INTERVAL '1 second')
			  AND NOT EXISTS (
				  SELECT 1 FROM users
				  WHERE profile_image_url LIKE '%' || u.filename || '%'
					 OR profile_image_url LIKE '%' || u.id::text || '%'
			  )
			  AND NOT EXISTS (
				  SELECT 1 FROM tasks
				  WHERE icon_url LIKE '%' || u.filename || '%'
					 OR icon_url LIKE '%' || u.id::text || '%'
			  )
			  AND NOT EXISTS (
				  SELECT 1 FROM rewards
				  WHERE icon_url LIKE '%' || u.filename || '%'
					 OR icon_url LIKE '%' || u.id::text || '%'
			  )
			  AND NOT EXISTS (
				  SELECT 1 FROM activity_history
				  WHERE metadata::text LIKE '%' || u.filename || '%'
					 OR metadata::text LIKE '%' || u.id::text || '%'
					 OR description LIKE '%' || u.filename || '%'
			  )
		), deleted AS (
			DELETE FROM uploads
			WHERE id IN (SELECT id FROM orphaned)
			RETURNING id
		)
		SELECT COUNT(*) FROM deleted;
	`

	var count int
	err := r.db.QueryRow(ctx, query, seconds).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// PurgeStaleSubmissionsAndPush cleans up rejected task submissions older than 7 days
// and removes push subscriptions associated with deleted users.
func (r *Repository) PurgeStaleSubmissionsAndPush(ctx context.Context) (int, error) {
	// Clean rejected submissions older than 7 days
	tag, err := r.db.Exec(ctx, `
		DELETE FROM task_submissions
		WHERE status = 'rejected'
		  AND created_at < NOW() - INTERVAL '7 days'
	`)
	if err != nil {
		return 0, err
	}
	submissionsPurged := tag.RowsAffected()

	// Clean orphaned push subscriptions
	_, _ = r.db.Exec(ctx, `
		DELETE FROM push_subscriptions
		WHERE user_id NOT IN (SELECT id FROM users)
	`)

	return int(submissionsPurged), nil
}


