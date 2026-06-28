package repository

import (
	"context"
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

// CreateTask creates a new task scoped to a choregroup.
func (r *Repository) CreateTask(ctx context.Context, task *model.Task) error {
	_, err := r.db.Exec(ctx, "INSERT INTO tasks (id, choregroup_id, title, type, points_reward, assigned_to_user_id, status) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		task.ID, task.ChoreGroupID, task.Title, task.Type, task.PointsReward, task.AssignedToUserID, task.Status)
	return err
}

// UpdateTaskStatus updates the status of a task.
func (r *Repository) UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error {
	_, err := r.db.Exec(ctx, "UPDATE tasks SET status = $1 WHERE id = $2", status, taskID)
	return err
}

// ListTasks retrieves tasks for a specific user in a choregroup.
func (r *Repository) ListTasks(ctx context.Context, choregroupID, userID uuid.UUID) ([]model.Task, error) {
	query := `
		SELECT id, choregroup_id, title, type, points_reward, assigned_to_user_id, status
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
		if err := rows.Scan(&task.ID, &task.ChoreGroupID, &task.Title, &task.Type, &task.PointsReward, &task.AssignedToUserID, &task.Status); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// GetUsersByPoints retrieves users for a specific choregroup, sorted by points.
func (r *Repository) GetUsersByPoints(ctx context.Context, choregroupID uuid.UUID) ([]model.User, error) {
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
	err := r.db.QueryRow(ctx, "SELECT id, choregroup_id, title, type, points_reward, assigned_to_user_id, status FROM tasks WHERE id = $1", id).Scan(
		&task.ID, &task.ChoreGroupID, &task.Title, &task.Type, &task.PointsReward, &task.AssignedToUserID, &task.Status)
	return &task, err
}

// UpdatePoints updates user points within a transaction.
func (r *Repository) UpdatePoints(ctx context.Context, choregroupID, userID uuid.UUID, points int, taskType string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if taskType == "cooperative" {
		_, err = tx.Exec(ctx, "UPDATE users SET points = points + $1 WHERE role = 'user' AND choregroup_id = $2", points, choregroupID)
	} else {
		_, err = tx.Exec(ctx, "UPDATE users SET points = points + $1 WHERE id = $2", points, userID)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
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

// ListSubmissions retrieves submissions for a choregroup.
func (r *Repository) ListSubmissions(ctx context.Context, choregroupID uuid.UUID) ([]model.TaskSubmission, error) {
	query := `
		SELECT s.id, s.task_id, s.submitted_by, s.status, s.created_at
		FROM task_submissions s
		JOIN tasks t ON s.task_id = t.id
		WHERE t.choregroup_id = $1
	`
	rows, err := r.db.Query(ctx, query, choregroupID)
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
