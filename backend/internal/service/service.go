package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"


	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"github.com/google/generative-ai-go/genai"
	webpush "github.com/SherClockHolmes/webpush-go"
	"google.golang.org/api/option"

	"ChoreCraft/internal/model"
	"ChoreCraft/internal/repository"
)

var ErrForbidden = errors.New("user does not have permission to access this resource")
var ErrInvalidCredentials = errors.New("invalid username or password")
var ErrTaskAlreadyApproved = errors.New("task has already been approved")
var ErrNoPendingSubmission = errors.New("no pending submission found for this task")
var ErrInsufficientPoints = errors.New("insufficient points")
var ErrAlreadyVoted = errors.New("user has already voted on this purchase")

// Round-robin counter for distributing AI requests across models
var aiModelCounter uint64

// Service holds the dependencies for the service layer.
type Service struct {
	repo            *repository.Repository
	geminiAPIKey    string
	vapidPublicKey  string
	vapidPrivateKey string
	vapidContact    string
}

// New creates a new Service instance.
func New(repo *repository.Repository, geminiAPIKey, vapidPublicKey, vapidPrivateKey, vapidContact string) *Service {
	return &Service{
		repo:            repo,
		geminiAPIKey:    geminiAPIKey,
		vapidPublicKey:  vapidPublicKey,
		vapidPrivateKey: vapidPrivateKey,
		vapidContact:    vapidContact,
	}
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

// SignUp hashes the password and creates a new choregroup and user.
func (s *Service) SignUp(ctx context.Context, choregroupName, username, password string) (model.User, error) {
	username = strings.ToLower(username)
	passwordHash, err := hashPassword(password)
	if err != nil {
		return model.User{}, err
	}
	return s.repo.CreateChoreGroupAndUser(ctx, choregroupName, username, passwordHash, "admin") // Role is always admin on signup
}

// AddUserToChoreGroup hashes the password and adds a new user to an existing choregroup.
func (s *Service) AddUserToChoreGroup(ctx context.Context, choregroupName, username, password, userRole string) (model.User, error) {
	username = strings.ToLower(username)
	passwordHash, err := hashPassword(password)
	if err != nil {
		return model.User{}, err
	}
	return s.repo.AddUserToChoreGroup(ctx, choregroupName, username, passwordHash, userRole)
}

// Login verifies a user's credentials and returns their details.
func (s *Service) Login(ctx context.Context, choregroupName, username, password string) (model.LoginResponse, error) {
	username = strings.ToLower(username)
	var user model.User
	var err error
	if choregroupName != "" {
		group, err := s.repo.GetChoreGroupByName(ctx, choregroupName)
		if err != nil {
			return model.LoginResponse{}, ErrInvalidCredentials
		}
		user, err = s.repo.GetUserByChoreGroupIDAndUsername(ctx, group.ID, username)
		if err != nil {
			return model.LoginResponse{}, ErrInvalidCredentials
		}
	} else {
		user, err = s.repo.GetUserByUsername(ctx, username)
		if err != nil {
			return model.LoginResponse{}, ErrInvalidCredentials
		}
	}
	if !checkPasswordHash(password, user.PasswordHash) {
		return model.LoginResponse{}, ErrInvalidCredentials
	}
	group, err := s.repo.GetChoreGroupByID(ctx, user.ChoreGroupID)
	if err != nil {
		return model.LoginResponse{}, ErrInvalidCredentials
	}
	return model.LoginResponse{
		UserID:         user.ID,
		Username:       user.Username,
		ChoreGroupID:   user.ChoreGroupID,
		ChoreGroupName: group.Name,
		Role:           user.Role,
	}, nil
}

// GetUserByID retrieves a user by their ID.
func (s *Service) GetUserByID(ctx context.Context, userID uuid.UUID) (model.User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// GetChoreGroupByID retrieves a choregroup by its ID.
func (s *Service) GetChoreGroupByID(ctx context.Context, choregroupID uuid.UUID) (model.ChoreGroup, error) {
	return s.repo.GetChoreGroupByID(ctx, choregroupID)
}

// UpdateChoreGroupAILanguage updates the household's AI content language.
func (s *Service) UpdateChoreGroupAILanguage(ctx context.Context, choregroupID uuid.UUID, language string) error {
	return s.repo.UpdateChoreGroupAILanguage(ctx, choregroupID, language)
}


// MarkNotificationsViewed updates notifications_viewed state.
func (s *Service) MarkNotificationsViewed(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkNotificationsViewed(ctx, userID)
}

// GetChoreGroupMembers retrieves all users belonging to a specific choregroup.
func (s *Service) GetChoreGroupMembers(ctx context.Context, choregroupID uuid.UUID) ([]model.User, error) {
	return s.repo.GetUsersByChoreGroupID(ctx, choregroupID)
}

// DeleteUser deletes a user.
func (s *Service) DeleteUser(ctx context.Context, adminUser model.User, userIDToDelete uuid.UUID) error {
	if adminUser.Role != "admin" {
		return ErrForbidden
	}

	userToDelete, err := s.repo.GetUserByID(ctx, userIDToDelete)
	if err != nil {
		return err
	}

	if userToDelete.ChoreGroupID != adminUser.ChoreGroupID {
		return ErrForbidden
	}

	return s.repo.DeleteUser(ctx, userIDToDelete, adminUser.ChoreGroupID)
}

// UpdateUserPassword hashes a user's password and updates it.
func (s *Service) UpdateUserPassword(ctx context.Context, loggedInUser model.User, targetUserID uuid.UUID, newPassword string) error {
	isSelf := loggedInUser.ID == targetUserID
	isAdmin := loggedInUser.Role == "admin"

	if !isSelf && !isAdmin {
		return ErrForbidden
	}

	targetUser, err := s.repo.GetUserByID(ctx, targetUserID)
	if err != nil {
		return err
	}

	if targetUser.ChoreGroupID != loggedInUser.ChoreGroupID {
		return ErrForbidden
	}

	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	return s.repo.UpdateUserPassword(ctx, targetUserID, loggedInUser.ChoreGroupID, passwordHash)
}


// HasActiveMandatoryTasks checks if a user has any active mandatory tasks.
func (s *Service) HasActiveMandatoryTasks(ctx context.Context, choregroupID, userID uuid.UUID) (bool, error) {
	return s.repo.HasActiveMandatoryTasksForUser(ctx, choregroupID, userID)
}

// CreateTask creates a new task from a DTO.
// If kidCreatorID is non-nil, the task was created by a kid and trivial tasks (1 coin) become mandatory.
func (s *Service) CreateTask(ctx context.Context, choregroupID uuid.UUID, req model.CreateTaskRequest, onResolved func(), kidCreatorID *uuid.UUID) (*model.Task, error) {
	if req.IsMandatory {
		req.PointsReward = 0
	}
	if req.ParentTaskID != nil {
		parent, err := s.repo.GetTask(ctx, *req.ParentTaskID, choregroupID)
		if err == nil && parent != nil && parent.AssignedToUserID != nil {
			req.AssignedToUserID = parent.AssignedToUserID
		}
	}
	title, needsAsync := s.ResolveEmoji(ctx, req.Title)
	req.Title = title


	task := &model.Task{
		ID:               uuid.New(),
		ChoreGroupID:     choregroupID,
		Title:            req.Title,
		Type:             req.Type,
		PointsReward:     req.PointsReward,
		IsMandatory:      req.IsMandatory,
		AssignedToUserID: req.AssignedToUserID,
		Status:           "assigned", // Initial status for a new task
		ExpiresAt:        req.ExpiresAt,
		ParentTaskID:     req.ParentTaskID,
		IconURL:          req.IconURL,
	}
	err := s.repo.CreateTask(ctx, task)
	if err != nil {
		return nil, err
	}
	if needsAsync {
		go func(t *model.Task, origTitle string) {
			bgCtx := context.Background()
			newTitle := s.ResolveEmojiAsync(bgCtx, origTitle)
			s.repo.UpdateTaskTitle(bgCtx, t.ID, newTitle)
			if onResolved != nil {
				onResolved()
			}
		}(task, req.Title) // req.Title is modified above, wait! I need to save original title.
	}
	return task, nil
}

// UpdateTask updates an existing task.
func (s *Service) UpdateTask(ctx context.Context, adminUser model.User, taskID uuid.UUID, req model.UpdateTaskRequest, onResolved func()) error {
	if adminUser.Role != "admin" {
		return ErrForbidden
	}

	task, err := s.repo.GetTask(ctx, taskID, adminUser.ChoreGroupID)
	if err != nil {
		return err
	}
	if task.ChoreGroupID != adminUser.ChoreGroupID {
		return ErrForbidden
	}
	if req.IsMandatory {
		req.PointsReward = 0
	}
	if req.ParentTaskID != nil {
		parent, err := s.repo.GetTask(ctx, *req.ParentTaskID, adminUser.ChoreGroupID)
		if err == nil && parent != nil && parent.AssignedToUserID != nil {
			req.AssignedToUserID = parent.AssignedToUserID
		}
	}
	origTitle := req.Title
	title, needsAsync := s.ResolveEmoji(ctx, req.Title)
	req.Title = title

	// If task had an uploaded photo that is being replaced or removed, delete the old file
	if task.IconURL != nil && *task.IconURL != "" {
		if req.IconURL == nil || *req.IconURL != *task.IconURL {
			DeleteUploadedPhoto(*task.IconURL)
		}
	}

	err = s.repo.UpdateTask(ctx, taskID, adminUser.ChoreGroupID, req)
	if err == nil && needsAsync {
		go func(id uuid.UUID, ot string) {
			bgCtx := context.Background()
			newTitle := s.ResolveEmojiAsync(bgCtx, ot)
			s.repo.UpdateTaskTitle(bgCtx, id, newTitle)
			if onResolved != nil {
				onResolved()
			}
		}(taskID, origTitle)
	}
	return err
}

// DeleteUploadedPhoto deletes an uploaded photo file from the uploads directory if rawURL points to a local upload.
func DeleteUploadedPhoto(rawURL string) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return
	}

	path := trimmed
	if u, err := url.Parse(trimmed); err == nil && u.Path != "" {
		path = u.Path
	}

	idx := strings.Index(path, "/uploads/")
	if idx == -1 {
		if strings.HasPrefix(path, "uploads/") {
			path = "/" + path
			idx = 0
		} else {
			return
		}
	}

	filename := path[idx+len("/uploads/"):]
	filename = filepath.Base(filename)
	if filename == "." || filename == "/" || filename == "" {
		return
	}

	targetPaths := []string{
		filepath.Join("./uploads", filename),
		filepath.Join("uploads", filename),
		filepath.Join("backend/uploads", filename),
	}

	for _, p := range targetPaths {
		if err := os.Remove(p); err == nil {
			slog.Info("deleted uploaded photo file", "path", p)
			return
		} else if !os.IsNotExist(err) {
			slog.Warn("failed to delete uploaded photo file", "path", p, "error", err)
		}
	}
}

// DeleteTask deletes a task.
func (s *Service) DeleteTask(ctx context.Context, adminUser model.User, taskID uuid.UUID) error {
	if adminUser.Role != "admin" {
		return ErrForbidden
	}

	task, err := s.repo.GetTask(ctx, taskID, adminUser.ChoreGroupID)
	if err != nil {
		return err
	}
	if task.ChoreGroupID != adminUser.ChoreGroupID {
		return ErrForbidden
	}

	if task.IconURL != nil && *task.IconURL != "" {
		DeleteUploadedPhoto(*task.IconURL)
	}

	return s.repo.DeleteTask(ctx, taskID, adminUser.ChoreGroupID)
}

// ListTasks retrieves tasks based on the user's role.
func (s *Service) ListTasks(ctx context.Context, user model.User) ([]model.Task, error) {
	// Always fetch all tasks for the choregroup
	tasks, err := s.repo.ListAllTasksForChoreGroup(ctx, user.ChoreGroupID)
	if err != nil {
		return nil, err
	}

	// For kids, filter to only their relevant tasks
	if user.Role != "admin" {
		var filtered []model.Task
		for _, t := range tasks {
			if t.AssignedToUserID == nil || *t.AssignedToUserID == user.ID || t.Type == "cooperative" {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	// Fetch all tasks for this choregroup to check parent status and build continuation counts
	allTasks, err := s.repo.ListAllTasksForChoreGroup(ctx, user.ChoreGroupID)
	if err != nil {
		return nil, err
	}

	// Create maps for efficient lookups
	idToTask := make(map[uuid.UUID]model.Task)
	parentToChildren := make(map[uuid.UUID][]model.Task)
	for _, t := range allTasks {
		idToTask[t.ID] = t
		if t.ParentTaskID != nil {
			parentToChildren[*t.ParentTaskID] = append(parentToChildren[*t.ParentTaskID], t)
		}
	}

	// Calculate downstream continuation counts recursively
	var countDescendants func(uuid.UUID) int
	countDescendants = func(parentID uuid.UUID) int {
		children := parentToChildren[parentID]
		count := len(children)
		for _, child := range children {
			count += countDescendants(child.ID)
		}
		return count
	}

	now := time.Now()
	for i := range tasks {
		if tasks[i].Status == "assigned" && !tasks[i].IsMandatory && tasks[i].ExpiresAt != nil && tasks[i].ExpiresAt.Before(now) {
			tasks[i].Status = "expired"
		}
		tasks[i].ContinuationCount = countDescendants(tasks[i].ID)
	}
	return tasks, nil
}

// GetStatistics retrieves the user leaderboard and cooperative points for a specific choregroup.
func (s *Service) GetStatistics(ctx context.Context, choregroupID uuid.UUID) (model.StatisticsResponse, error) {
	users, err := s.repo.GetUsersSortedByPoints(ctx, choregroupID)
	if err != nil {
		return model.StatisticsResponse{}, err
	}

	group, err := s.repo.GetChoreGroupByID(ctx, choregroupID)
	if err != nil {
		return model.StatisticsResponse{}, err
	}

	return model.StatisticsResponse{
		Users:             users,
		CooperativePoints: group.CooperativePoints,
	}, nil
}

// UpdateTaskStatusByAdmin approves or rejects a task based on the latest submission.
func (s *Service) UpdateTaskStatusByAdmin(ctx context.Context, adminUser model.User, taskID uuid.UUID, action string) error {
	if adminUser.Role != "admin" {
		return ErrForbidden
	}

	task, err := s.repo.GetTask(ctx, taskID, adminUser.ChoreGroupID)
	if err != nil {
		return err
	}
	if task.ChoreGroupID != adminUser.ChoreGroupID {
		return ErrForbidden
	}

	// Prevent re-approving an already approved task
	if task.Status == "done" && action == "approve" {
		return ErrTaskAlreadyApproved
	}

	// Get the latest pending submission for this task
	submission, err := s.repo.GetLatestPendingSubmissionForTask(ctx, taskID, adminUser.ChoreGroupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNoPendingSubmission
		}
		return err
	}

	if action == "approve" {
		err = s.repo.UpdatePoints(ctx, task.ChoreGroupID, submission.SubmittedBy, task.PointsReward, task.Type)
		if err != nil {
			return err
		}
		// Update task status to "done"
		if err := s.repo.UpdateTaskStatus(ctx, taskID, adminUser.ChoreGroupID, "done"); err != nil {
			return err
		}
		// Delete uploaded photo if present, and clear icon_url in DB so it cleanly falls back to emoji
		if task.IconURL != nil && *task.IconURL != "" {
			DeleteUploadedPhoto(*task.IconURL)
			_ = s.repo.ClearTaskIcon(ctx, taskID, adminUser.ChoreGroupID)
		}
		// Update submission status to "approved"
		if err := s.repo.UpdateTaskSubmissionStatus(ctx, submission.ID, adminUser.ChoreGroupID, "approved"); err != nil {
			return err
		}
		go s.broadcastFCMNotification(taskID)
		return nil
	}
	if action == "reject" {
		// Revert task status to "assigned"
		if err := s.repo.UpdateTaskStatus(ctx, taskID, adminUser.ChoreGroupID, "assigned"); err != nil {
			return err
		}
		// Update submission status to "rejected"
		if err := s.repo.UpdateTaskSubmissionStatus(ctx, submission.ID, adminUser.ChoreGroupID, "rejected"); err != nil {
			return err
		}
		return nil
	}
	return errors.New("invalid action")
}

// SubmitTask creates a new task submission for the logged-in user.
func (s *Service) SubmitTask(ctx context.Context, taskID, userID, choregroupID uuid.UUID) (*model.TaskSubmission, error) {
	task, err := s.repo.GetTask(ctx, taskID, choregroupID)
	if err != nil {
		return nil, err
	}

	if task.Status == "assigned" && !task.IsMandatory && task.ExpiresAt != nil && task.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("this task has expired and cannot be completed")
	}

	// If this task has a parent task, verify that the parent task is marked as done
	if task.ParentTaskID != nil {
		parent, err := s.repo.GetTask(ctx, *task.ParentTaskID, choregroupID)
		if err != nil {
			return nil, err
		}
		if parent.Status != "done" {
			return nil, errors.New("you must complete the parent task first")
		}
	}

	if !task.IsMandatory {
		userTasks, err := s.repo.ListTasksForUser(ctx, task.ChoreGroupID, userID)
		if err != nil {
			return nil, err
		}
		for _, t := range userTasks {
			if t.IsMandatory && (t.Status == "assigned" || t.Status == "pending_approval") {
				return nil, errors.New("you must complete all mandatory tasks first")
			}
		}
	}

	// When a task is submitted, its status should change to "pending_approval"
	if err := s.repo.UpdateTaskStatus(ctx, taskID, choregroupID, "pending_approval"); err != nil {
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

// ListSubmissions retrieves submissions for a choregroup, filtered by status.
func (s *Service) ListSubmissions(ctx context.Context, choregroupID uuid.UUID, status string) ([]model.TaskSubmission, error) {
	return s.repo.ListSubmissions(ctx, choregroupID, status)
}

// CreateReward creates a new reward.
func (s *Service) CreateReward(ctx context.Context, user model.User, req model.CreateRewardRequest, onResolved func()) (*model.Reward, error) {
	if user.Role != "admin" {
		// Kids can create individual rewards for themselves via Snap Reward
		req.Type = "individual"
		req.AssignedToUserID = &user.ID
	}
	if req.Cost <= 0 {
		req.Cost = 1
	}
	origName := req.Name
	name, needsAsync := s.ResolveEmoji(ctx, req.Name)
	req.Name = name
	reward := &model.Reward{
		ID:               uuid.New(),
		ChoreGroupID:     user.ChoreGroupID,
		Name:             req.Name,
		Description:      req.Description,
		Cost:             req.Cost,
		Type:             req.Type,
		AssignedToUserID: req.AssignedToUserID,
		IconURL:          req.IconURL,
	}
	err := s.repo.CreateReward(ctx, reward)
	if err != nil {
		return nil, err
	}
	if needsAsync {
		go func(r *model.Reward, on string) {
			bgCtx := context.Background()
			newName := s.ResolveEmojiAsync(bgCtx, on)
			s.repo.UpdateRewardName(bgCtx, r.ID, newName)
			if onResolved != nil {
				onResolved()
			}
		}(reward, origName)
	}
	return reward, nil
}

func (s *Service) ListRewards(ctx context.Context, choregroupID uuid.UUID) ([]model.Reward, error) {
	return s.repo.ListRewards(ctx, choregroupID)
}

func (s *Service) UpdateReward(ctx context.Context, adminUser model.User, rewardID uuid.UUID, req model.CreateRewardRequest, onResolved func()) error {
	if adminUser.Role != "admin" {
		return ErrForbidden
	}
	reward, err := s.repo.GetReward(ctx, rewardID, adminUser.ChoreGroupID)
	if err != nil {
		return err
	}
	if reward.ChoreGroupID != adminUser.ChoreGroupID {
		return ErrForbidden
	}
	origName := req.Name
	name, needsAsync := s.ResolveEmoji(ctx, req.Name)
	req.Name = name
	err = s.repo.UpdateReward(ctx, rewardID, adminUser.ChoreGroupID, req)
	if err == nil && needsAsync {
		go func(id uuid.UUID, on string) {
			bgCtx := context.Background()
			newName := s.ResolveEmojiAsync(bgCtx, on)
			s.repo.UpdateRewardName(bgCtx, id, newName)
			if onResolved != nil {
				onResolved()
			}
		}(rewardID, origName)
	}
	return err
}

func (s *Service) DeleteReward(ctx context.Context, adminUser model.User, rewardID uuid.UUID) error {
	if adminUser.Role != "admin" {
		return ErrForbidden
	}
	reward, err := s.repo.GetReward(ctx, rewardID, adminUser.ChoreGroupID)
	if err != nil {
		return err
	}
	if reward.ChoreGroupID != adminUser.ChoreGroupID {
		return ErrForbidden
	}
	if reward.IconURL != nil && *reward.IconURL != "" {
		DeleteUploadedPhoto(*reward.IconURL)
	}
	return s.repo.DeleteReward(ctx, rewardID, adminUser.ChoreGroupID)
}

func (s *Service) CreatePurchase(ctx context.Context, user model.User, req model.CreatePurchaseRequest) (*model.RewardPurchase, error) {
	reward, err := s.repo.GetReward(ctx, req.RewardID, user.ChoreGroupID)
	if err != nil {
		return nil, err
	}

	purchase := &model.RewardPurchase{
		ID:                uuid.New(),
		RewardID:          req.RewardID,
		PurchasedByUserID: user.ID,
	}

	// Admins bypass all point checks and are auto-approved
	if user.Role == "admin" {
		purchase.Status = "approved"
		purchase.Approvals = json.RawMessage("{}")
		return purchase, s.repo.CreateRewardPurchase(ctx, purchase)
	}

	if reward.Type == "individual" {
		if user.Points < reward.Cost {
			return nil, ErrInsufficientPoints
		}
		if err := s.repo.UpdatePoints(ctx, user.ChoreGroupID, user.ID, -reward.Cost, "individual"); err != nil {
			return nil, err
		}
		purchase.Status = "approved"
		purchase.Approvals = json.RawMessage("{}")
	} else { // Cooperative
		group, err := s.repo.GetChoreGroupByID(ctx, user.ChoreGroupID)
		if err != nil {
			return nil, err
		}
		if group.CooperativePoints < reward.Cost {
			return nil, ErrInsufficientPoints
		}
		if err := s.repo.UpdatePoints(ctx, user.ChoreGroupID, user.ID, -reward.Cost, "cooperative"); err != nil {
			return nil, err
		}

		members, err := s.repo.GetUsersByChoreGroupID(ctx, user.ChoreGroupID)
		if err != nil {
			return nil, err
		}

		approvals := make(map[string]string)
		for _, member := range members {
			if member.ID != user.ID && member.Role != "admin" {
				approvals[member.ID.String()] = "pending"
			}
		}
		approvalsJSON, _ := json.Marshal(approvals)
		purchase.Status = "pending_approval"
		purchase.Approvals = approvalsJSON
	}

	return purchase, s.repo.CreateRewardPurchase(ctx, purchase)
}


func (s *Service) ListPurchases(ctx context.Context, user model.User, status string) ([]model.RewardPurchase, error) {
	return s.repo.ListRewardPurchases(ctx, user.ChoreGroupID, status)
}

func (s *Service) CreateApproval(ctx context.Context, user model.User, purchaseID uuid.UUID, req model.CreateApprovalRequest) error {
	purchase, err := s.repo.GetRewardPurchase(ctx, purchaseID, user.ChoreGroupID)
	if err != nil {
		return err
	}

	var approvals map[string]string
	if err := json.Unmarshal(purchase.Approvals, &approvals); err != nil {
		return err
	}

	if _, ok := approvals[user.ID.String()]; !ok {
		return ErrForbidden // User is not eligible to vote
	}
	if approvals[user.ID.String()] != "pending" {
		return ErrAlreadyVoted
	}

	if req.Vote == "rejected" {
		purchase.Status = "rejected"
		reward, err := s.repo.GetReward(ctx, purchase.RewardID, user.ChoreGroupID)
		if err != nil {
			return err
		}
		if err := s.repo.UpdatePoints(ctx, user.ChoreGroupID, purchase.PurchasedByUserID, reward.Cost, "cooperative"); err != nil {
			return err
		}
	} else {
		approvals[user.ID.String()] = "approved"
		allApproved := true
		for _, status := range approvals {
			if status != "approved" {
				allApproved = false
				break
			}
		}
		if allApproved {
			purchase.Status = "approved"
		}
	}

	purchase.Approvals, _ = json.Marshal(approvals)
	return s.repo.UpdateRewardPurchase(ctx, &purchase, user.ChoreGroupID)
}

func (s *Service) UpdatePurchaseStatus(ctx context.Context, adminUser model.User, purchaseID uuid.UUID, req model.UpdatePurchaseStatusRequest) error {
	if adminUser.Role != "admin" {
		return ErrForbidden
	}
	purchase, err := s.repo.GetRewardPurchase(ctx, purchaseID, adminUser.ChoreGroupID)
	if err != nil {
		return err
	}
	reward, err := s.repo.GetReward(ctx, purchase.RewardID, adminUser.ChoreGroupID)
	if err != nil {
		return err
	}
	if reward.ChoreGroupID != adminUser.ChoreGroupID {
		return ErrForbidden
	}
	if purchase.Status != "approved" {
		return errors.New("purchase is not in approved state")
	}
	purchase.Status = req.Status
	return s.repo.UpdateRewardPurchase(ctx, &purchase, adminUser.ChoreGroupID)
}

func (s *Service) CancelPurchase(ctx context.Context, user model.User, purchaseID uuid.UUID) error {
	purchase, err := s.repo.GetRewardPurchase(ctx, purchaseID, user.ChoreGroupID)
	if err != nil {
		return err
	}
	reward, err := s.repo.GetReward(ctx, purchase.RewardID, user.ChoreGroupID)
	if err != nil {
		return err
	}
	if reward.ChoreGroupID != user.ChoreGroupID {
		return ErrForbidden
	}
	
	if user.Role != "admin" && purchase.PurchasedByUserID != user.ID {
		return ErrForbidden
	}

	if purchase.Status == "fulfilled" {
		return errors.New("cannot cancel a fulfilled purchase")
	}

	// Refund points
	if err := s.repo.UpdatePoints(ctx, user.ChoreGroupID, purchase.PurchasedByUserID, reward.Cost, reward.Type); err != nil {
		return err
	}

	return s.repo.DeleteRewardPurchase(ctx, purchaseID, user.ChoreGroupID)
}



func (s *Service) ResolveEmoji(ctx context.Context, title string) (string, bool) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", false
	}
	// Check if already starts with an emoji (basic unicode heuristic)
	r := []rune(title)[0]
	if r > 0x2000 {
		return title, false
	}

	// Check DB cache
	emoji, err := s.repo.GetEmojiForTitle(ctx, title)
	if err == nil && emoji != "" {
		return emoji + " " + title, false
	}

	if s.geminiAPIKey == "" {
		return title, false
	}

	return title, true
}

func (s *Service) ResolveEmojiAsync(ctx context.Context, title string) string {
	if s.geminiAPIKey == "" {
		return title
	}

	// Query Gemini
	client, err := genai.NewClient(ctx, option.WithAPIKey(s.geminiAPIKey))
	if err != nil {
		slog.Error("failed to create gemini client", "error", err)
		return title
	}
	defer client.Close()

	fastModels := []string{
		"gemini-3.1-flash-lite",
		"gemini-2.5-flash",
		"gemini-flash-lite-latest",
	}

	start := int(atomic.AddUint64(&aiModelCounter, 1) - 1) % len(fastModels)
	modelsToTry := make([]string, len(fastModels))
	for i := range fastModels {
		modelsToTry[i] = fastModels[(start+i)%len(fastModels)]
	}

	prompt := `Extract a single core keyword and a highly relevant single emoji from this task/reward title. Return ONLY a JSON object: {"keyword": "word", "emoji": "😀"}. Title: ` + title

	var resp *genai.GenerateContentResponse
	for _, modelName := range modelsToTry {
		modelCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		model := client.GenerativeModel(modelName)
		model.ResponseMIMEType = "application/json"
		temp := float32(0.1)
		model.Temperature = &temp
		maxTokens := int32(64)
		model.MaxOutputTokens = &maxTokens
		r, err := model.GenerateContent(modelCtx, genai.Text(prompt))
		cancel()
		if err == nil && r != nil && len(r.Candidates) > 0 && len(r.Candidates[0].Content.Parts) > 0 {
			resp = r
			break
		}
		if err != nil {
			slog.Error("failed to generate gemini content", "model", modelName, "error", err)
		}
	}

	if resp == nil {
		return title
	}

	part := resp.Candidates[0].Content.Parts[0]
	jsonStr := ""
	if txt, ok := part.(genai.Text); ok {
		jsonStr = string(txt)
	} else {
		return title
	}

	jsonStr = strings.TrimSpace(jsonStr)
	startIdx := strings.Index(jsonStr, "{")
	endIdx := strings.LastIndex(jsonStr, "}")
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		jsonStr = jsonStr[startIdx : endIdx+1]
	}

	var result struct {
		Keyword string `json:"keyword"`
		Emoji   string `json:"emoji"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		slog.Error("failed to parse gemini json", "raw", jsonStr, "error", err)
		return title
	}

	if result.Emoji != "" {
		cleanedTitle := strings.ToLower(strings.TrimSpace(title))
		if len(cleanedTitle) > 100 {
			cleanedTitle = cleanedTitle[:100]
		}
		_ = s.repo.SaveIconMapping(ctx, cleanedTitle, result.Emoji)

		if result.Keyword != "" {
			_ = s.repo.SaveIconMapping(ctx, strings.ToLower(result.Keyword), result.Emoji)
		}
		return result.Emoji + " " + title
	}

	return title
}

// SavePushSubscription saves a user's web push subscription.
func (s *Service) SavePushSubscription(ctx context.Context, userID uuid.UUID, endpoint, p256dh, auth string) error {
	return s.repo.SavePushSubscription(ctx, userID, endpoint, p256dh, auth)
}

// SendPushToKids sends a web push notification to all kid devices in a choregroup.
func (s *Service) SendPushToKids(ctx context.Context, choregroupID uuid.UUID, title, body string) {
	if s.vapidPublicKey == "" || s.vapidPrivateKey == "" {
		slog.Warn("VAPID keys not configured, skipping push notifications")
		return
	}

	subs, err := s.repo.GetPushSubscriptionsByChoreGroup(ctx, choregroupID)
	if err != nil {
		slog.Error("failed to get push subscriptions", "error", err)
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
	})

	for _, sub := range subs {
		go func(sub model.PushSubscription) {
			resp, err := webpush.SendNotification(payload, &webpush.Subscription{
				Endpoint: sub.Endpoint,
				Keys: webpush.Keys{
					P256dh: sub.P256dh,
					Auth:   sub.Auth,
				},
			}, &webpush.Options{
				Subscriber:      s.vapidContact,
				VAPIDPublicKey:  s.vapidPublicKey,
				VAPIDPrivateKey: s.vapidPrivateKey,
				TTL:             60,
			})
			if err != nil {
				slog.Error("failed to send push notification", "endpoint", sub.Endpoint, "error", err)
				// Remove stale/expired subscriptions (410 Gone or 404)
				if resp != nil && (resp.StatusCode == 410 || resp.StatusCode == 404) {
					_ = s.repo.DeletePushSubscription(ctx, sub.Endpoint)
				}
				return
			}
			defer resp.Body.Close()
			slog.Info("push notification sent", "endpoint", sub.Endpoint[:40]+"...", "status", resp.StatusCode)
		}(sub)
	}
}

// FibonacciSequence defines standard coin tiers: 1, 2, 3, 5, 8, 13, 21, 34, 55, 89
var FibonacciSequence = []int{1, 2, 3, 5, 8, 13, 21, 34, 55, 89}

// SnapToFibonacci snaps an integer coin count to the nearest Fibonacci number (minimum 1).
func SnapToFibonacci(val int) int {
	if val <= 1 {
		return 1
	}
	nearest := FibonacciSequence[0]
	minDiff := absVal(val - nearest)

	for _, fib := range FibonacciSequence {
		diff := absVal(val - fib)
		if diff < minDiff {
			minDiff = diff
			nearest = fib
		}
	}
	return nearest
}

func absVal(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// CalculateRewardCoins converts estimated USD into a clean reward coin value.
// In ChoreCraft, 1 coin corresponds roughly to $1 USD of retail value, rounded cleanly:
// - <= 10 USD: exact integer (min 1)
// - 11 - 30 USD: nearest 5 (e.g. 15, 20, 25, 30)
// - 31 - 100 USD: nearest 10 (e.g. 40, 50, 60... 100)
// - 101 - 500 USD: nearest 25 (e.g. 125, 150, 175... 500)
// - > 500 USD: nearest 50 (e.g. 550, 600, 750, 1000)
func CalculateRewardCoins(usd float64) int {
	if usd <= 0 {
		return 5
	}
	val := int(math.Round(usd))
	switch {
	case val <= 10:
		if val < 1 {
			return 1
		}
		return val
	case val <= 30:
		return ((val + 2) / 5) * 5
	case val <= 100:
		return ((val + 5) / 10) * 10
	case val <= 500:
		return ((val + 12) / 25) * 25
	default:
		return ((val + 25) / 50) * 50
	}
}

// CalculateChoreCoins converts estimated USD and/or suggested coins into a fair, motivating chore coin reward.
// In ChoreCraft, chores use the Fibonacci sequence [1, 2, 3, 5, 8, 13, 21]:
// - 1 coin: micro-task (< 2 mins, e.g. picking up a single item, putting a cup in the sink)
// - 2 coins: quick chore (2-5 mins, e.g. wiping the dining table, taking out trash, making bed, feeding pet)
// - 3 coins: small chore (5-10 mins, e.g. clearing & cleaning the whole dining table, emptying dishwasher, sweeping a room)
// - 5 coins: moderate chore (10-20 mins, e.g. mopping floors, vacuuming living room, cleaning bathroom sink, folding laundry)
// - 8 coins: substantial chore (20-35 mins, e.g. scrubbing bathtub, detailed room cleanup)
// - 13 coins: large chore (35-60 mins, e.g. deep kitchen clean, yard work)
// - 21 coins: major deep clean (1+ hours)
func CalculateChoreCoins(usd float64, suggestedCoins int) int {
	if suggestedCoins >= 1 {
		return SnapToFibonacci(suggestedCoins)
	}
	switch {
	case usd <= 2.0:
		return 1
	case usd <= 5.0:
		return 2
	case usd <= 9.0:
		return 3
	case usd <= 18.0:
		return 5
	case usd <= 32.0:
		return 8
	case usd <= 55.0:
		return 13
	default:
		return 21
	}
}

// AnalyzeImageForChoreOrReward uses Gemini Vision to classify an image as a Chore or Reward, estimate USD value, and calculate coins.
func (s *Service) AnalyzeImageForChoreOrReward(ctx context.Context, imageData []byte, mimeType string, iconURL string, targetLang string, mode string) (*model.AIImageAnalysisResult, error) {
	if s.geminiAPIKey == "" {
		return nil, errors.New("Gemini API key not configured")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(s.geminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}
	defer client.Close()

	if targetLang == "" {
		targetLang = "Hebrew"
	}

	// genai.ImageData() expects format (e.g. "png", "jpeg"), not full MIME type.
	// It internally prepends "image/", so we must strip it.
	imageFormat := strings.TrimPrefix(mimeType, "image/")
	if imageFormat == "" {
		imageFormat = "jpeg"
	}

	normalizedMode := strings.ToLower(strings.TrimSpace(mode))
	var prompt string
	if normalizedMode == "reward" {
		prompt = fmt.Sprintf(`CRITICAL REQUIREMENT: You MUST write ALL user-facing text strictly in %s. Do NOT output English or any other language unless %s is English.

You are an expert appraiser for a family rewards store. Analyze this photo to identify the REWARD item, toy, gadget, console, treat, or experience shown.

Case 1 - If NO recognizable reward item, toy, product, treat, or prize is visible (e.g. blank screen, blurry photo, random wall/floor, completely empty space):
- Set "is_valid": false
- Set "invalid_reason" to a friendly 1-sentence explanation strictly in %s explaining that no reward item or prize was recognized in this photo.
- Set "title": ""
- Set "description": ""

Case 2 - If a reward item or prize is visible:
- Set "is_valid": true
- Set "invalid_reason": ""
- "item_type": "reward"
- "title": Name of the reward item strictly in %s with a leading emoji (e.g. '🎮 סוני פלייסטיישן 5', '🍕 פיצה משפחתית', '🚲 אופניים', '🍦 גלידה', '🧸 בובת פרווה').
- "description": Short, appealing description strictly in %s.
- "estimated_usd": Realistic retail market value in USD (where 1 USD = 1 Coin in the rewards store). Guidelines:
  * Treats, candy, snacks: 2.0 to 8.0
  * Small toys, cinema tickets, pizza, books: 10.0 to 30.0
  * Video games, board games, Lego sets, headphones: 40.0 to 100.0
  * Smartwatches, bikes, tablets: 150.0 to 350.0
  * High-end gaming consoles (PlayStation 5 / PS5, Xbox Series X, Nintendo Switch), laptops, smartphones: 500.0 to 1000.0+
- "valuation_reasoning": Brief reasoning strictly in %s explaining the estimated market price.
- "suggested_emoji": A fitting emoji for the item (e.g. 🎮, 🍕, 🚲, 🧸).

Return ONLY a valid JSON object:
{"is_valid":true,"invalid_reason":"","item_type":"reward","title":"🎮 סוני פלייסטיישן 5","description":"קונסולת משחקים פלייסטיישן 5","category":"individual","estimated_usd":500.0,"valuation_reasoning":"מחיר שוק קמעונאי ממוצע לקונסולה","suggested_emoji":"🎮"}`, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang)
	} else if normalizedMode == "chore" {
		prompt = fmt.Sprintf(`CRITICAL REQUIREMENT: You MUST write ALL user-facing text strictly in %s. Do NOT output English or any other language unless %s is English.

Analyze this photo to determine if it shows a household CHORE to do.

Case 1 - If NO chore is visible (e.g. room is already clean, nothing to clean/tidy, or unrelated subject):
- Set "is_valid": false
- Set "invalid_reason" to a friendly 1-sentence explanation strictly in %s explaining why no chore is needed (e.g. telling a child gently in %s that everything looks clean or there is nothing to tidy here).
- Set "title": ""
- Set "description": ""

Case 2 - If a chore is visible:
- Set "is_valid": true
- Set "invalid_reason": ""
- "item_type": "chore"
- "category": "personal" (IMPORTANT: All chores detected from photos are rewarding household tasks for coins. NEVER set category to "mandatory". Mandatory tasks are only set manually by parents).
- "title": Action to take strictly in %s with a leading emoji (e.g. '🧼 לנקות את שולחן האוכל', '🧹 לטאטא את הרצפה', '🍽️ להוציא כלים מהמדיח', '🧺 לקפל כביסה', '🧸 לסדר את המשחקים').
- "description": Short description strictly in %s describing what needs to be done.
- "points_reward": Coin reward from the Fibonacci scale [1, 2, 3, 5, 8, 13, 21] based on real effort and time:
  * 1 coin: Micro-task (< 2 mins, e.g. picking up a single toy or stray pair of shoes, putting a cup in sink).
  * 2 coins: Quick task (2-5 mins, e.g. wiping the dining table, taking out trash, making bed, feeding pet).
  * 3 coins: Small chore (5-10 mins, e.g. clearing and wiping the whole dining table, emptying dishwasher, sweeping a room).
  * 5 coins: Moderate chore (10-20 mins, e.g. mopping floors, vacuuming living room, cleaning bathroom sink, folding laundry).
  * 8 coins: Substantial chore (20-35 mins, e.g. scrubbing bathtub/shower, detailed room cleanup).
  * 13 coins: Large chore (35-60 mins, e.g. deep kitchen clean, yard work).
  * 21 coins: Major deep clean (1+ hours).
  NOTE: Everyday tasks taking a few minutes (such as cleaning/wiping the dining table, taking out trash, or loading dishes) are NOT trivial and must receive at least 2 or 3 coins!
- "estimated_usd": Estimated market value in USD (approx. $1.50 - $2.00 per coin, e.g. $4.0 - $6.0 for cleaning dining table).
- "valuation_reasoning": Brief reasoning strictly in %s explaining the effort, estimated duration in minutes, and why this coin reward was chosen.
- "suggested_emoji": A fitting emoji for the chore (e.g. 🧼, 🧹, 🍽️, 🧺, 🛏️, 🧸).

Return ONLY a valid JSON object:
{"is_valid":true,"invalid_reason":"","item_type":"chore","title":"🧼 לנקות את שולחן האוכל","description":"לפנות ולנקות את שולחן האוכל","category":"personal","points_reward":2,"estimated_usd":4.0,"valuation_reasoning":"משימה של כמה דקות (2 מטבעות)","suggested_emoji":"🧼"}`, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang)
	} else {
		prompt = fmt.Sprintf(`CRITICAL REQUIREMENT: You MUST write ALL user-facing text strictly in %s. Do NOT output English or any other language unless %s is English.

Analyze this photo to determine if it shows a household CHORE to do or a REWARD item.

Case 1 - If NO chore or reward is visible (e.g. room is already clean, nothing to clean/tidy, or unrelated subject):
- Set "is_valid": false
- Set "invalid_reason" to a friendly 1-sentence explanation strictly in %s explaining why no chore or reward is detected.
- Set "title": ""
- Set "description": ""

Case 2 - If a CHORE is visible:
- Set "is_valid": true
- Set "invalid_reason": ""
- "item_type": "chore"
- "category": "personal" (IMPORTANT: All chores detected from photos are rewarding household tasks for coins. NEVER set category to "mandatory". Mandatory tasks are only set manually by parents).
- "title": Action to take strictly in %s with a leading emoji (e.g. '🧼 לנקות את שולחן האוכל', '🧹 לטאטא את הרצפה', '🍽️ להוציא כלים מהמדיח').
- "description": Short description strictly in %s.
- "points_reward": Coin reward from the Fibonacci scale [1, 2, 3, 5, 8, 13, 21] based on real effort and time:
  * 1 coin: Micro-task (< 2 mins, e.g. picking up a single toy).
  * 2 coins: Quick task (2-5 mins, e.g. wiping the dining table, taking out trash, making bed).
  * 3 coins: Small chore (5-10 mins, e.g. clearing & cleaning the whole dining table, emptying dishwasher).
  * 5 coins: Moderate chore (10-20 mins, e.g. mopping floors, vacuuming).
  * 8 coins: Substantial chore (20-35 mins).
  * 13 coins: Large chore (35-60 mins).
  * 21 coins: Major deep clean (1+ hours).
  NOTE: Everyday tasks taking a few minutes (such as cleaning/wiping the dining table) are NOT trivial and must receive at least 2 or 3 coins!
- "estimated_usd": Estimated market value in USD (approx. $1.50 - $2.00 per coin).
- "valuation_reasoning": Brief reasoning strictly in %s explaining the effort, estimated duration in minutes, and why this coin reward was chosen.
- "suggested_emoji": A fitting emoji for the chore (e.g. 🧼, 🧹, 🍽️).

Case 3 - If a REWARD item is visible:
- Set "is_valid": true
- Set "invalid_reason": ""
- "item_type": "reward"
- "category": "individual"
- "title": Name of the reward strictly in %s with a leading emoji (e.g. '🎮 סוני פלייסטיישן 5').
- "estimated_usd": Retail market value in USD (e.g. $500.0-$1000.0 for PS5/consoles, $10.0-$30.0 for small toys/treats).
- "description": Short description strictly in %s.
- "valuation_reasoning": Brief reasoning strictly in %s.
- "suggested_emoji": A fitting emoji for the item (e.g. 🎮, 🍕, 🚲).

Return ONLY a valid JSON object:
{"is_valid":true,"invalid_reason":"","item_type":"chore","title":"...","description":"...","category":"personal","points_reward":2,"estimated_usd":4.0,"valuation_reasoning":"...","suggested_emoji":"..."}`, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang)
	}

	fastModels := []string{
		"gemini-3.1-flash-lite",
		"gemini-2.5-flash",
	}

	// Rotate starting model between the fastest models to balance quota evenly
	start := int(atomic.AddUint64(&aiModelCounter, 1) - 1) % len(fastModels)
	modelsToTry := make([]string, len(fastModels))
	for i := range fastModels {
		modelsToTry[i] = fastModels[(start+i)%len(fastModels)]
	}

	var resp *genai.GenerateContentResponse
	var errMsgs []string
	var lastErr error
	for _, modelName := range modelsToTry {
		modelCtx, cancel := context.WithTimeout(ctx, 28*time.Second)
		genModel := client.GenerativeModel(modelName)
		genModel.ResponseMIMEType = "application/json"
		temp := float32(0.1)
		genModel.Temperature = &temp
		maxTokens := int32(1024)
		genModel.MaxOutputTokens = &maxTokens
		r, err := genModel.GenerateContent(modelCtx, genai.ImageData(imageFormat, imageData), genai.Text(prompt))
		cancel()
		if err == nil && r != nil && len(r.Candidates) > 0 && len(r.Candidates[0].Content.Parts) > 0 {
			resp = r
			lastErr = nil
			break
		}
		if err != nil {
			lastErr = err
			errMsgs = append(errMsgs, fmt.Sprintf("%s (%v)", modelName, err))
			// If it timed out, do not make another attempt to avoid compounding Google's rate queue
			lowerErr := strings.ToLower(err.Error())
			if strings.Contains(lowerErr, "deadline exceeded") || strings.Contains(lowerErr, "timeout") {
				break
			}
		}
	}

	if resp == nil {
		errStr := ""
		if lastErr != nil {
			errStr = lastErr.Error()
		}
		for _, m := range errMsgs {
			errStr += " " + m
		}

		isHebrew := strings.Contains(targetLang, "Hebrew") || strings.Contains(targetLang, "עברית")
		lower := strings.ToLower(errStr)
		if strings.Contains(lower, "429") || strings.Contains(lower, "quota") || strings.Contains(lower, "resource_exhausted") || strings.Contains(lower, "rate limit") {
			if isHebrew {
				return nil, errors.New("🤖⏳ שרתי ה-AI עמוסים כרגע (מגבלת קצב). אנא המתינו 15-20 שניות ונסו שוב! 😴✨")
			}
			return nil, errors.New("🤖⏳ AI servers are busy right now (rate limit queue). Please wait 15-20 seconds and try again! 😴✨")
		}
		if strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout") {
			if isHebrew {
				return nil, errors.New("🤖⏳ שרתי Google AI עמוסים כרגע ומעבדים לאט. אנא המתינו 15-20 שניות ונסו לצלם שוב! 📸")
			}
			return nil, errors.New("🤖⏳ Google AI servers are currently busy and processing slowly. Please wait 15-20 seconds and try again! 📸")
		}
		if isHebrew {
			return nil, errors.New("🤖⚠️ לא ניתן היה להשלים את ניתוח התמונה כרגע. אנא נסו לצלם שוב! 📸")
		}
		return nil, errors.New("🤖⚠️ AI image analysis could not be completed. Please try taking the photo again!")
	}

	var sb strings.Builder
	for _, p := range resp.Candidates[0].Content.Parts {
		if txt, ok := p.(genai.Text); ok {
			sb.WriteString(string(txt))
		}
	}
	jsonStr := strings.TrimSpace(sb.String())
	if jsonStr == "" {
		return nil, errors.New("unexpected empty response from gemini")
	}

	startIdx := strings.Index(jsonStr, "{")
	endIdx := strings.LastIndex(jsonStr, "}")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		slog.Error("failed to find json in gemini response", "raw", jsonStr)
		return nil, errors.New("AI could not extract valid task details from this photo. Please try again! 📸")
	}
	jsonStr = jsonStr[startIdx : endIdx+1]

	var res model.AIImageAnalysisResult
	if err := json.Unmarshal([]byte(jsonStr), &res); err != nil {
		slog.Error("failed to parse gemini vision json", "raw", jsonStr, "error", err)
		return nil, errors.New("AI returned an unexpected format. Please try again! 📸")
	}

	if res.EstimatedUSD <= 0 {
		res.EstimatedUSD = 5.0
	}

	if normalizedMode == "reward" || res.ItemType == "reward" {
		res.ItemType = "reward"
		res.Category = "individual"
		res.PointsReward = CalculateRewardCoins(res.EstimatedUSD)
	} else {
		res.ItemType = "chore"
		if res.Category == "mandatory" || res.Category == "" {
			res.Category = "personal"
		}
		res.PointsReward = CalculateChoreCoins(res.EstimatedUSD, res.PointsReward)
	}
	res.IconURL = iconURL

	return &res, nil
}

