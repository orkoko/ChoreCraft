package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"ChoreCraft/internal/model"
	"ChoreCraft/internal/repository"
)

var ErrForbidden = errors.New("user does not have permission to access this resource")
var ErrInvalidCredentials = errors.New("invalid username or password")
var ErrTaskAlreadyApproved = errors.New("task has already been approved")
var ErrNoPendingSubmission = errors.New("no pending submission found for this task")

// Service holds the dependencies for the service layer.
type Service struct {
	repo *repository.Repository
}

// New creates a new Service instance.
func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

// hashPassword generates a bcrypt hash of the password.
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// checkPasswordHash compares a password with a hash.
func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// SignUp hashes the password and creates a new household and user.
func (s *Service) SignUp(ctx context.Context, householdName, username, password string) (model.User, error) {
	passwordHash, err := hashPassword(password)
	if err != nil {
		return model.User{}, err
	}
	return s.repo.CreateHouseholdAndUser(ctx, householdName, username, passwordHash, "admin") // Role is always admin on signup
}

// AddUserToHousehold hashes the password and adds a new user to an existing household.
func (s *Service) AddUserToHousehold(ctx context.Context, householdName, username, password, userRole string) (model.User, error) {
	passwordHash, err := hashPassword(password)
	if err != nil {
		return model.User{}, err
	}
	return s.repo.AddUserToHousehold(ctx, householdName, username, passwordHash, userRole)
}

// Login verifies a user's credentials and returns their details.
func (s *Service) Login(ctx context.Context, username, password string) (model.LoginResponse, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return model.LoginResponse{}, ErrInvalidCredentials
	}
	if !checkPasswordHash(password, user.PasswordHash) {
		return model.LoginResponse{}, ErrInvalidCredentials
	}
	return model.LoginResponse{
		UserID:      user.ID,
		HouseholdID: user.HouseholdID,
		Role:        user.Role,
	}, nil
}

// GetUserByID retrieves a user by their ID.
func (s *Service) GetUserByID(ctx context.Context, userID uuid.UUID) (model.User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// GetHouseholdMembers retrieves all users belonging to a specific household.
func (s *Service) GetHouseholdMembers(ctx context.Context, householdID uuid.UUID) ([]model.User, error) {
	return s.repo.GetUsersByHouseholdID(ctx, householdID)
}

// CreateTask creates a new task from a DTO.
func (s *Service) CreateTask(ctx context.Context, householdID uuid.UUID, req model.CreateTaskRequest) (*model.Task, error) {
	task := &model.Task{
		ID:               uuid.New(),
		HouseholdID:      householdID,
		Title:            req.Title,
		Type:             req.Type,
		PointsReward:     req.PointsReward,
		AssignedToUserID: req.AssignedToUserID,
		Status:           "assigned", // Initial status for a new task
	}
	return task, s.repo.CreateTask(ctx, task)
}

// ListTasks retrieves tasks for a specific user in a household.
func (s *Service) ListTasks(ctx context.Context, householdID, userID uuid.UUID) ([]model.Task, error) {
	return s.repo.ListTasks(ctx, householdID, userID)
}

// GetLeaderboard retrieves the user leaderboard for a specific household.
func (s *Service) GetLeaderboard(ctx context.Context, householdID uuid.UUID) ([]model.User, error) {
	return s.repo.GetUsersByPoints(ctx, householdID)
}

// UpdateTaskStatusByAdmin approves or rejects a task based on the latest submission.
func (s *Service) UpdateTaskStatusByAdmin(ctx context.Context, adminUser model.User, taskID uuid.UUID, action string) error {
	if adminUser.Role != "admin" {
		return ErrForbidden
	}

	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.HouseholdID != adminUser.HouseholdID {
		return ErrForbidden
	}

	// Prevent re-approving an already approved task
	if task.Status == "done" && action == "approve" {
		return ErrTaskAlreadyApproved
	}

	// Get the latest pending submission for this task
	submission, err := s.repo.GetLatestPendingSubmissionForTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) { // Assuming repository returns ErrNotFound for no submission
			return ErrNoPendingSubmission
		}
		return err
	}

	if action == "approve" {
		err = s.repo.UpdatePoints(ctx, task.HouseholdID, submission.SubmittedBy, task.PointsReward, task.Type)
		if err != nil {
			return err
		}
		// Update task status to "done"
		if err := s.repo.UpdateTaskStatus(ctx, taskID, "done"); err != nil {
			return err
		}
		// Update submission status to "approved"
		if err := s.repo.UpdateTaskSubmissionStatus(ctx, submission.ID, "approved"); err != nil {
			return err
		}
		go s.broadcastFCMNotification(taskID)
		return nil
	}
	if action == "reject" {
		// Revert task status to "assigned" or "open"
		if err := s.repo.UpdateTaskStatus(ctx, taskID, "assigned"); err != nil { // Assuming "assigned" is the original state
			return err
		}
		// Update submission status to "rejected"
		if err := s.repo.UpdateTaskSubmissionStatus(ctx, submission.ID, "rejected"); err != nil {
			return err
		}
		return nil
	}
	return errors.New("invalid action")
}

// SubmitTask creates a new task submission for the logged-in user.
func (s *Service) SubmitTask(ctx context.Context, taskID, userID uuid.UUID) (*model.TaskSubmission, error) {
	// When a task is submitted, its status should change to "pending_approval"
	if err := s.repo.UpdateTaskStatus(ctx, taskID, "pending_approval"); err != nil {
		return nil, err
	}

	submission := &model.TaskSubmission{
		ID:          uuid.New(),
		TaskID:      taskID,
		SubmittedBy: userID,
		Status:      "pending_approval",
	}
	return submission, s.repo.CreateTaskSubmission(ctx, submission)
}

func (s *Service) broadcastFCMNotification(taskID uuid.UUID) {
	slog.Info("broadcasting FCM notification", "task_id", taskID)
}

// ListSubmissions retrieves submissions for a household.
func (s *Service) ListSubmissions(ctx context.Context, householdID uuid.UUID) ([]model.TaskSubmission, error) {
	return s.repo.ListSubmissions(ctx, householdID)
}
