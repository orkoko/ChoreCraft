package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ChoreCraft/internal/model"
)

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
		ID:           uuid.New(),
		ChoreGroupID: choregroupID,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         userRole,
	}
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
		ID:           uuid.New(),
		ChoreGroupID: choregroupID,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         userRole,
	}
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
	err := r.db.QueryRow(ctx, "SELECT id, choregroup_id, username, password_hash, role, points FROM users WHERE username = $1", username).Scan(
		&user.ID, &user.ChoreGroupID, &user.Username, &user.PasswordHash, &user.Role, &user.Points)
	return user, err
}

// GetUserByID retrieves a user by their ID.
func (r *Repository) GetUserByID(ctx context.Context, userID uuid.UUID) (model.User, error) {
	var user model.User
	err := r.db.QueryRow(ctx, "SELECT id, choregroup_id, username, role, points FROM users WHERE id = $1", userID).Scan(
		&user.ID, &user.ChoreGroupID, &user.Username, &user.Role, &user.Points)
	return user, err
}

// GetUsersByChoreGroupID retrieves all users belonging to a specific choregroup.
func (r *Repository) GetUsersByChoreGroupID(ctx context.Context, choregroupID uuid.UUID) ([]model.User, error) {
	rows, err := r.db.Query(ctx, "SELECT id, choregroup_id, username, role, points FROM users WHERE choregroup_id = $1", choregroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var user model.User
		if err := rows.Scan(&user.ID, &user.ChoreGroupID, &user.Username, &user.Role, &user.Points); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

// DeleteUser deletes a user by their ID.
func (r *Repository) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	return err
}

// GetChoreGroupByID retrieves a choregroup by its ID.
func (r *Repository) GetChoreGroupByID(ctx context.Context, choregroupID uuid.UUID) (model.ChoreGroup, error) {
	var group model.ChoreGroup
	err := r.db.QueryRow(ctx, "SELECT id, name, cooperative_points FROM choregroups WHERE id = $1", choregroupID).Scan(
		&group.ID, &group.Name, &group.CooperativePoints)
	return group, err
}

// CreateTask creates a new task scoped to a choregroup.
func (r *Repository) CreateTask(ctx context.Context, task *model.Task) error {
	_, err := r.db.Exec(ctx, "INSERT INTO tasks (id, choregroup_id, title, type, points_reward, is_mandatory, assigned_to_user_id, status, expires_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		task.ID, task.ChoreGroupID, task.Title, task.Type, task.PointsReward, task.IsMandatory, task.AssignedToUserID, task.Status, task.ExpiresAt)
	return err
}

// UpdateTask updates an existing task.
func (r *Repository) UpdateTask(ctx context.Context, taskID uuid.UUID, req model.UpdateTaskRequest) error {
	_, err := r.db.Exec(ctx, "UPDATE tasks SET title = $1, type = $2, points_reward = $3, is_mandatory = $4, assigned_to_user_id = $5, expires_at = $6 WHERE id = $7",
		req.Title, req.Type, req.PointsReward, req.IsMandatory, req.AssignedToUserID, req.ExpiresAt, taskID)
	return err
}

// DeleteTask deletes a task by its ID.
func (r *Repository) DeleteTask(ctx context.Context, taskID uuid.UUID) error {
	// Delete any submissions referencing this task first (self-healing for schemas missing ON DELETE CASCADE)
	_, err := r.db.Exec(ctx, "DELETE FROM task_submissions WHERE task_id = $1", taskID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, "DELETE FROM tasks WHERE id = $1", taskID)
	return err
}

// UpdateTaskStatus updates the status of a task.
func (r *Repository) UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error {
	_, err := r.db.Exec(ctx, "UPDATE tasks SET status = $1 WHERE id = $2", status, taskID)
	return err
}

// ListTasksForUser retrieves tasks for a specific user in a choregroup (public and assigned).
func (r *Repository) ListTasksForUser(ctx context.Context, choregroupID, userID uuid.UUID) ([]model.Task, error) {
	query := `
		SELECT id, choregroup_id, title, type, points_reward, is_mandatory, assigned_to_user_id, status, expires_at
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
		if err := rows.Scan(&task.ID, &task.ChoreGroupID, &task.Title, &task.Type, &task.PointsReward, &task.IsMandatory, &task.AssignedToUserID, &task.Status, &task.ExpiresAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// ListAllTasksForChoreGroup retrieves all tasks for a choregroup.
func (r *Repository) ListAllTasksForChoreGroup(ctx context.Context, choregroupID uuid.UUID) ([]model.Task, error) {
	query := `
		SELECT id, choregroup_id, title, type, points_reward, is_mandatory, assigned_to_user_id, status, expires_at
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
		if err := rows.Scan(&task.ID, &task.ChoreGroupID, &task.Title, &task.Type, &task.PointsReward, &task.IsMandatory, &task.AssignedToUserID, &task.Status, &task.ExpiresAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// GetUsersSortedByPoints retrieves users for a specific choregroup, sorted by points.
func (r *Repository) GetUsersSortedByPoints(ctx context.Context, choregroupID uuid.UUID) ([]model.User, error) {
	rows, err := r.db.Query(ctx, "SELECT id, choregroup_id, username, role, points FROM users WHERE choregroup_id = $1 ORDER BY points DESC", choregroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var user model.User
		if err := rows.Scan(&user.ID, &user.ChoreGroupID, &user.Username, &user.Role, &user.Points); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

// GetTask retrieves a task by ID.
func (r *Repository) GetTask(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	var task model.Task
	err := r.db.QueryRow(ctx, "SELECT id, choregroup_id, title, type, points_reward, is_mandatory, assigned_to_user_id, status, expires_at FROM tasks WHERE id = $1", id).Scan(
		&task.ID, &task.ChoreGroupID, &task.Title, &task.Type, &task.PointsReward, &task.IsMandatory, &task.AssignedToUserID, &task.Status, &task.ExpiresAt)
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

// GetTaskSubmission retrieves a task submission by ID.
func (r *Repository) GetTaskSubmission(ctx context.Context, id uuid.UUID) (*model.TaskSubmission, error) {
	var submission model.TaskSubmission
	err := r.db.QueryRow(ctx, "SELECT id, task_id, submitted_by, status, created_at FROM task_submissions WHERE id = $1", id).Scan(
		&submission.ID, &submission.TaskID, &submission.SubmittedBy, &submission.Status, &submission.CreatedAt)
	return &submission, err
}

// GetLatestPendingSubmissionForTask retrieves the most recent pending submission for a given task.
func (r *Repository) GetLatestPendingSubmissionForTask(ctx context.Context, taskID uuid.UUID) (*model.TaskSubmission, error) {
	var submission model.TaskSubmission
	err := r.db.QueryRow(ctx, `
		SELECT id, task_id, submitted_by, status, created_at
		FROM task_submissions
		WHERE task_id = $1 AND status = 'pending_approval'
		ORDER BY created_at DESC
		LIMIT 1
	`, taskID).Scan(&submission.ID, &submission.TaskID, &submission.SubmittedBy, &submission.Status, &submission.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &submission, nil
}

// UpdateTaskSubmissionStatus updates the status of a task submission.
func (r *Repository) UpdateTaskSubmissionStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.db.Exec(ctx, "UPDATE task_submissions SET status = $1 WHERE id = $2", status, id)
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
	_, err := r.db.Exec(ctx, "INSERT INTO rewards (id, choregroup_id, name, description, cost, type, assigned_to_user_id) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		reward.ID, reward.ChoreGroupID, reward.Name, reward.Description, reward.Cost, reward.Type, reward.AssignedToUserID)
	return err
}

func (r *Repository) GetReward(ctx context.Context, rewardID uuid.UUID) (model.Reward, error) {
	var reward model.Reward
	err := r.db.QueryRow(ctx, "SELECT id, choregroup_id, name, description, cost, type, assigned_to_user_id FROM rewards WHERE id = $1", rewardID).Scan(
		&reward.ID, &reward.ChoreGroupID, &reward.Name, &reward.Description, &reward.Cost, &reward.Type, &reward.AssignedToUserID)
	return reward, err
}

func (r *Repository) ListRewards(ctx context.Context, choregroupID uuid.UUID) ([]model.Reward, error) {
	rows, err := r.db.Query(ctx, "SELECT id, choregroup_id, name, description, cost, type, assigned_to_user_id FROM rewards WHERE choregroup_id = $1", choregroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rewards []model.Reward
	for rows.Next() {
		var reward model.Reward
		if err := rows.Scan(&reward.ID, &reward.ChoreGroupID, &reward.Name, &reward.Description, &reward.Cost, &reward.Type, &reward.AssignedToUserID); err != nil {
			return nil, err
		}
		rewards = append(rewards, reward)
	}
	return rewards, nil
}

func (r *Repository) UpdateReward(ctx context.Context, rewardID uuid.UUID, req model.CreateRewardRequest) error {
	_, err := r.db.Exec(ctx, "UPDATE rewards SET name = $1, description = $2, cost = $3, type = $4, assigned_to_user_id = $5 WHERE id = $6",
		req.Name, req.Description, req.Cost, req.Type, req.AssignedToUserID, rewardID)
	return err
}

func (r *Repository) DeleteReward(ctx context.Context, rewardID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM rewards WHERE id = $1", rewardID)
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

func (r *Repository) GetRewardPurchase(ctx context.Context, purchaseID uuid.UUID) (model.RewardPurchase, error) {
	var purchase model.RewardPurchase
	err := r.db.QueryRow(ctx, "SELECT id, reward_id, purchased_by_user_id, status, approvals FROM reward_purchases WHERE id = $1", purchaseID).Scan(
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

func (r *Repository) UpdateRewardPurchase(ctx context.Context, purchase *model.RewardPurchase) error {
	approvalsJSON, err := json.Marshal(purchase.Approvals)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, "UPDATE reward_purchases SET status = $1, approvals = $2 WHERE id = $3",
		purchase.Status, approvalsJSON, purchase.ID)
	return err
}

func (r *Repository) RefundCooperativePoints(ctx context.Context, choregroupID uuid.UUID, amount int) error {
	_, err := r.db.Exec(ctx, "UPDATE choregroups SET cooperative_points = cooperative_points + $1 WHERE id = $2", amount, choregroupID)
	return err
}

func (r *Repository) DeleteRewardPurchase(ctx context.Context, purchaseID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM reward_purchases WHERE id = $1", purchaseID)
	return err
}

func (r *Repository) GetEmojiForTitle(ctx context.Context, title string) (string, error) {
	var emoji string
	err := r.db.QueryRow(ctx, "SELECT emoji FROM icon_mappings WHERE $1 ILIKE '%' || keyword || '%' LIMIT 1", title).Scan(&emoji)
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
