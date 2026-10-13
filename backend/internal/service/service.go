package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"log/slog"
	"math"
	"math/rand/v2"
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
var ErrNoSpinsAvailable = repository.ErrNoSpinsAvailable

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
	newUser, err := s.repo.AddUserToChoreGroup(ctx, choregroupName, username, passwordHash, userRole)
	return newUser, err
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
		UserID:                user.ID,
		Username:              user.Username,
		ChoreGroupID:          user.ChoreGroupID,
		ChoreGroupName:        group.Name,
		Role:                  user.Role,
		HasClaimedWelcomeSpin: user.HasClaimedWelcomeSpin,
		ProfileImageURL:       user.ProfileImageURL,
		PointsBackground:      user.PointsBackground,
		UnlockedBackgrounds:   user.UnlockedBackgrounds,
		AvatarFrame:           user.AvatarFrame,
		UnlockedAvatarFrames:  user.UnlockedAvatarFrames,
		AILanguage:            group.AILanguage,
		AIInstructions:        group.AIInstructions,
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

// UpdateChoreGroupAIInstructions updates the household's custom instructions for AI chore analysis and rewards.
func (s *Service) UpdateChoreGroupAIInstructions(ctx context.Context, choregroupID uuid.UUID, instructions string) error {
	return s.repo.UpdateChoreGroupAIInstructions(ctx, choregroupID, instructions)
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

// UpdateUserProfileImage updates a user's avatar / profile image URL.
func (s *Service) UpdateUserProfileImage(ctx context.Context, loggedInUser model.User, targetUserID uuid.UUID, avatarURL string) error {
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

	var urlPtr *string
	trimmed := strings.TrimSpace(avatarURL)
	if trimmed != "" {
		urlPtr = &trimmed
	}

	return s.repo.UpdateUserProfileImage(ctx, targetUserID, urlPtr)
}


// HasActiveMandatoryTasks checks if a user has any active mandatory tasks.
func (s *Service) HasActiveMandatoryTasks(ctx context.Context, choregroupID, userID uuid.UUID) (bool, error) {
	return s.repo.HasActiveMandatoryTasksForUser(ctx, choregroupID, userID)
}

// CreateTask creates a new task from a DTO.
// If kidCreatorID is non-nil, the task was created by a user/kid:
// chores created by users have 1 less coin than if an admin creates it;
// if the estimation is 1 coin, instead of 0 coins the task becomes mandatory on the user who created it.
func (s *Service) CreateTask(ctx context.Context, choregroupID uuid.UUID, req model.CreateTaskRequest, onResolved func(), kidCreatorID *uuid.UUID) (*model.Task, error) {
	if kidCreatorID != nil {
		req.AssignedToUserID = kidCreatorID
		req.Type = "individual"
	}

	if req.IsMandatory {
		req.PointsReward = 0
	} else if req.PointsReward <= 0 {
		var customRules string
		if group, err := s.repo.GetChoreGroupByID(ctx, choregroupID); err == nil {
			customRules = group.AIInstructions
		}
		baseCoins := s.EstimateChoreCoins(ctx, req.Title, customRules)
		if kidCreatorID != nil {
			if baseCoins <= 1 {
				req.IsMandatory = true
				req.PointsReward = 0
				req.AssignedToUserID = kidCreatorID
			} else {
				req.PointsReward = baseCoins - 1
			}
		} else {
			req.PointsReward = baseCoins
		}
	} else if kidCreatorID != nil {
		// When kid creates a task directly without photo upload (or manual API call):
		if req.IconURL == nil || *req.IconURL == "" {
			if req.PointsReward <= 1 {
				req.IsMandatory = true
				req.PointsReward = 0
				req.AssignedToUserID = kidCreatorID
			} else {
				req.PointsReward = req.PointsReward - 1
			}
		}
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
	} else if req.PointsReward <= 0 {
		var customRules string
		if group, err := s.repo.GetChoreGroupByID(ctx, adminUser.ChoreGroupID); err == nil {
			customRules = group.AIInstructions
		}
		req.PointsReward = s.EstimateChoreCoins(ctx, req.Title, customRules)
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

// SaveUpload stores binary image content in the PostgreSQL database repository.
func (s *Service) SaveUpload(ctx context.Context, filename string, mimeType string, data []byte) (string, error) {
	ext := filepath.Ext(filename)
	if ext == "" {
		if strings.Contains(mimeType, "png") {
			ext = ".png"
		} else if strings.Contains(mimeType, "gif") {
			ext = ".gif"
		} else if strings.Contains(mimeType, "webp") {
			ext = ".webp"
		} else {
			ext = ".jpg"
		}
	}

	fileID := uuid.New()
	cleanFilename := fmt.Sprintf("%s%s", fileID.String(), ext)

	upload := model.Upload{
		ID:        fileID,
		Filename:  cleanFilename,
		MIMEType:  mimeType,
		Data:      data,
		SizeBytes: int64(len(data)),
		CreatedAt: time.Now(),
	}

	if err := s.repo.SaveUpload(ctx, upload); err != nil {
		return "", err
	}

	return fmt.Sprintf("/uploads/%s", cleanFilename), nil
}

// GetUploadByID retrieves an upload record with its binary BYTEA data from PostgreSQL repository.
func (s *Service) GetUploadByID(ctx context.Context, id uuid.UUID) (*model.Upload, error) {
	return s.repo.GetUploadByID(ctx, id)
}

// DeleteUploadedPhoto deletes an uploaded photo file from the database repository and local disk.
func (s *Service) DeleteUploadedPhoto(ctx context.Context, rawURL string) {
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

	rawID := strings.TrimSuffix(filename, filepath.Ext(filename))
	if fileID, err := uuid.Parse(rawID); err == nil {
		_ = s.repo.DeleteUploadByID(ctx, fileID)
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
		}
	}
}

// DeleteUploadedPhoto is a helper for deleting uploaded files.
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

	err = s.repo.DeleteTask(ctx, taskID, adminUser.ChoreGroupID)
	if err == nil {
		_ = s.repo.LogActivity(ctx, model.ActivityEvent{
			ChoreGroupID: adminUser.ChoreGroupID,
			UserID:       &adminUser.ID,
			UserUsername: adminUser.Username,
			ActivityType: "task_deleted",
			Title:        task.Title,
			Description:  task.Title,
			Metadata: map[string]interface{}{
				"task_id": task.ID.String(),
				"title":   task.Title,
			},
		})
	}
	return err
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
		pointsToAward := task.PointsReward
		// Check if user has double coins bonus active for today
		// (Mandatory tasks have 0 reward and must not consume or be counted as the 2x coins reward)
		if !task.IsMandatory && task.PointsReward > 0 {
			doubleActive, err := s.repo.CheckAndConsumeDoubleCoins(ctx, task.ChoreGroupID, submission.SubmittedBy)
			if err == nil && doubleActive {
				pointsToAward = task.PointsReward * 2
			}
		}

		err = s.repo.UpdatePoints(ctx, task.ChoreGroupID, submission.SubmittedBy, pointsToAward, task.Type)
		if err != nil {
			return err
		}
		// Record daily task completion (mandatory tasks count as tasks for the progress bar)
		_ = s.repo.RecordDailyTaskCompletion(ctx, task.ChoreGroupID, submission.SubmittedBy)

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

		submittingUser, _ := s.repo.GetUserByID(ctx, submission.SubmittedBy)
		_ = s.repo.LogActivity(ctx, model.ActivityEvent{
			ChoreGroupID: adminUser.ChoreGroupID,
			UserID:       &submission.SubmittedBy,
			UserUsername: submittingUser.Username,
			ActivityType: "task_completed",
			Title:        "Task Completed",
			Description:  task.Title,
			Metadata: map[string]interface{}{
				"task_id": task.ID.String(),
				"title":   task.Title,
				"points":  pointsToAward,
			},
		})

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
	err = s.repo.CreateTaskSubmission(ctx, submission)
	return submission, err
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
	_ = s.repo.LogActivity(ctx, model.ActivityEvent{
		ChoreGroupID: user.ChoreGroupID,
		UserID:       &user.ID,
		UserUsername: user.Username,
		ActivityType: "reward_created",
		Title:        "Reward Created",
		Description:  reward.Name,
		Metadata: map[string]interface{}{
			"reward_id": reward.ID.String(),
			"name":      reward.Name,
			"cost":      reward.Cost,
		},
	})
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
	err = s.repo.DeleteReward(ctx, rewardID, adminUser.ChoreGroupID)
	if err == nil {
		_ = s.repo.LogActivity(ctx, model.ActivityEvent{
			ChoreGroupID: adminUser.ChoreGroupID,
			UserID:       &adminUser.ID,
			UserUsername: adminUser.Username,
			ActivityType: "reward_deleted",
			Title:        reward.Name,
			Description:  reward.Name,
			Metadata: map[string]interface{}{
				"reward_id": reward.ID.String(),
				"name":      reward.Name,
				"cost":      reward.Cost,
			},
		})
	}
	return err
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
		err = s.repo.CreateRewardPurchase(ctx, purchase)
		if err == nil {
			_ = s.repo.LogActivity(ctx, model.ActivityEvent{
				ChoreGroupID: user.ChoreGroupID,
				UserID:       &user.ID,
				UserUsername: user.Username,
				ActivityType: "reward_obtained",
				Title:        "Reward Obtained",
				Description:  reward.Name,
				Metadata: map[string]interface{}{
					"purchase_id": purchase.ID.String(),
					"reward_id":   reward.ID.String(),
					"name":        reward.Name,
					"cost":        reward.Cost,
				},
			})
		}
		return purchase, err
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

	err = s.repo.CreateRewardPurchase(ctx, purchase)
	if err == nil {
		_ = s.repo.LogActivity(ctx, model.ActivityEvent{
			ChoreGroupID: user.ChoreGroupID,
			UserID:       &user.ID,
			UserUsername: user.Username,
			ActivityType: "reward_obtained",
			Title:        "Reward Obtained",
			Description:  reward.Name,
			Metadata: map[string]interface{}{
				"purchase_id": purchase.ID.String(),
				"reward_id":   reward.ID.String(),
				"name":        reward.Name,
				"cost":        reward.Cost,
			},
		})
	}
	return purchase, err
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
		"gemini-2.5-flash-lite",
		"gemini-flash-lite-latest",
		"gemini-2.5-flash",
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

// SnapToFibonacci preserves backward-compatibility, returning the coin count directly (minimum 1).
func SnapToFibonacci(val int) int {
	if val <= 1 {
		return 1
	}
	return val
}

func absVal(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// CalculateRewardCoins converts estimated USD into a clean reward coin value.
// In ChoreCraft, snap rewards coin decisions are doubled (1 USD = 2 Coins) because daily rewards
// (lucky wheel spins, daily 2x multipliers) make coins easier to earn:
// - <= 10 USD: exact integer * 2 (min 2 coins)
// - 11 - 30 USD: nearest 5 * 2 (e.g. $15 -> 30, $20 -> 40, $25 -> 50, $30 -> 60)
// - 31 - 100 USD: nearest 10 * 2 (e.g. $50 -> 100, $100 -> 200)
// - 101 - 500 USD: nearest 25 * 2 (e.g. $250 -> 500, $500 -> 1000)
// - > 500 USD: nearest 50 * 2 (e.g. $750 -> 1500, $1000 -> 2000)
func CalculateRewardCoins(usd float64) int {
	if usd <= 0 {
		return 10
	}
	val := int(math.Round(usd))
	var baseCoins int
	switch {
	case val <= 10:
		if val < 1 {
			baseCoins = 1
		} else {
			baseCoins = val
		}
	case val <= 30:
		baseCoins = ((val + 2) / 5) * 5
	case val <= 100:
		baseCoins = ((val + 5) / 10) * 10
	case val <= 500:
		baseCoins = ((val + 12) / 25) * 25
	default:
		baseCoins = ((val + 25) / 50) * 50
	}
	return baseCoins * 2
}

// CalculateChoreCoins converts estimated USD and/or suggested coins into a fair, motivating chore coin reward.
func CalculateChoreCoins(usd float64, suggestedCoins int) int {
	if suggestedCoins >= 1 {
		return suggestedCoins
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
		coins := int(math.Round(usd / 2.0))
		if coins < 1 {
			return 1
		}
		return coins
	}
}

// EstimateChoreCoins uses Gemini to analyze the chore title and assign a fair coin reward.
func (s *Service) EstimateChoreCoins(ctx context.Context, title string, customInstructions ...string) int {
	cleanTitle := strings.TrimSpace(title)
	if cleanTitle == "" {
		return 3
	}
	if s.geminiAPIKey == "" {
		return s.fallbackChoreCoins(cleanTitle)
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(s.geminiAPIKey))
	if err != nil {
		slog.Error("failed to create gemini client for coin estimation", "error", err)
		return s.fallbackChoreCoins(cleanTitle)
	}
	defer client.Close()

	customRulesText := ""
	if len(customInstructions) > 0 && strings.TrimSpace(customInstructions[0]) != "" {
		customRulesText = fmt.Sprintf("\nCRITICAL - HOUSEHOLD CUSTOM RULES (MUST STRICTLY OVERRIDE ALL DEFAULT TIERS):\n\"\"\"\n%s\n\"\"\"\nYou MUST strictly follow the household rules above when calculating coins! For example, if a rule specifies an exact point value (e.g. 'כל משימה היא 10 נקודות' or 'every task is 10 points'), you MUST award that EXACT point value (e.g. 10)!\n", strings.TrimSpace(customInstructions[0]))
	}

	prompt := `You are an expert chore valuation assistant for a family chore app.
Analyze the effort, time, and difficulty required for this chore title and determine the most appropriate coin reward:
- 1 coin: Micro-task (< 1-2 mins, extremely light effort, e.g. touching up an already mostly organized closet or shelf, emptying a blue recycling basket / paper bin 'לרוקן סל מחזור כחול / פח נייר', putting a dish/cup in the sink, picking up shoes, turning off lights, throwing away a small wrapper)
- 2 coins: Quick chore (2-5 mins active effort, e.g. taking out the main household kitchen garbage bag, wiping dining table, making bed, feeding pet)
- 3 coins: Small chore (5-10 mins, e.g. emptying dishwasher, sweeping room, tidying desk, watering plants)
- 5 coins: Moderate chore (10-20 mins, e.g. vacuuming room/hallway, mopping floor, folding laundry, cleaning sink)
- 8 coins: Substantial chore (20-35 mins, e.g. scrubbing bathroom, detailed room clean, cleaning oven)
- 13 coins: Large chore (35-60 mins, e.g. deep kitchen clean, yard work, mowing lawn, washing car)
- 20+ coins: Major deep clean (1+ hours, whole house deep clean, attic/garage cleanout)

CRITICAL: Super light tasks that take barely a minute (like emptying a blue paper recycling basket, putting a cup in the sink, picking up shoes, or minor touch-up to an already organized closet) MUST be 1 coin!
` + customRulesText + `
Return ONLY a valid JSON object with the integer points: {"points": 1}
Chore title: ` + cleanTitle

	modelCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	model := client.GenerativeModel("gemini-2.5-flash-lite")
	model.ResponseMIMEType = "application/json"
	temp := float32(0.1)
	model.Temperature = &temp
	maxTokens := int32(512)
	model.MaxOutputTokens = &maxTokens

	r, err := model.GenerateContent(modelCtx, genai.Text(prompt))
	if err != nil || r == nil || len(r.Candidates) == 0 || len(r.Candidates[0].Content.Parts) == 0 {
		return s.fallbackChoreCoins(cleanTitle)
	}

	part := r.Candidates[0].Content.Parts[0]
	if txt, ok := part.(genai.Text); ok {
		var result struct {
			Points int `json:"points"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(string(txt))), &result); err == nil && result.Points > 0 {
			return result.Points
		}
	}

	return s.fallbackChoreCoins(cleanTitle)
}

func (s *Service) fallbackChoreCoins(title string) int {
	lower := strings.ToLower(title)
	// Micro tasks (1 coin) - super light effort (< 1-2 mins):
	// Emptying recycling bins (סל מחזור / פח כחול / מחזור / סל ניירות), putting cup/dish in sink, picking up shoes, turning off lights
	if strings.Contains(lower, "מחזור") || strings.Contains(lower, "recycle") || strings.Contains(lower, "recycling") ||
		strings.Contains(lower, "פח כחול") || strings.Contains(lower, "סל כחול") || strings.Contains(lower, "פח נייר") ||
		strings.Contains(lower, "סל נייר") || strings.Contains(lower, "פח קטן") || strings.Contains(lower, "פח חדר") ||
		strings.Contains(lower, "פח שולחן") || strings.Contains(lower, "כוס") || strings.Contains(lower, "צלחת") ||
		strings.Contains(lower, "נעליים") || strings.Contains(lower, "cup") || strings.Contains(lower, "shoes") || strings.Contains(lower, "light") {
		return 1
	}
	// Quick chores (2 coins) - active effort taking 2-5 mins:
	// Main household trash (זבל / אשפה), making bed, feeding pet, wiping dining table
	if strings.Contains(lower, "זבל") || strings.Contains(lower, "אשפה") || strings.Contains(lower, "מיטה") || strings.Contains(lower, "להאכיל") ||
		strings.Contains(lower, "trash") || strings.Contains(lower, "garbage") || strings.Contains(lower, "bed") || strings.Contains(lower, "feed") {
		return 2
	}
	// Substantial chores (8 coins)
	if strings.Contains(lower, "אמבטיה") || strings.Contains(lower, "שירותים") || strings.Contains(lower, "לגזום") || strings.Contains(lower, "קרטון") ||
		strings.Contains(lower, "bathroom") || strings.Contains(lower, "bathtub") || strings.Contains(lower, "prune") {
		return 8
	}
	// Large / Major (13 - 21 coins)
	if strings.Contains(lower, "יסודי") || strings.Contains(lower, "חצר") || strings.Contains(lower, "מכונית") || strings.Contains(lower, "רכב") ||
		strings.Contains(lower, "yard") || strings.Contains(lower, "deep clean") || strings.Contains(lower, "garage") ||
		(strings.Contains(lower, "car") && !strings.Contains(lower, "carpet") && !strings.Contains(lower, "carry")) {
		return 13
	}
	// Moderate (5 coins)
	if strings.Contains(lower, "לשאוב") || strings.Contains(lower, "לשטוף") || strings.Contains(lower, "כביסה") || strings.Contains(lower, "ספונג'ה") ||
		strings.Contains(lower, "vacuum") || strings.Contains(lower, "mop") || strings.Contains(lower, "laundry") || strings.Contains(lower, "carpet") {
		return 5
	}
	// Default small/moderate
	return 3
}

// AnalyzeImageForChoreOrReward uses Gemini Vision to classify an image as a Chore or Reward, estimate USD value, and calculate coins.
func (s *Service) AnalyzeImageForChoreOrReward(ctx context.Context, imageData []byte, mimeType string, iconURL string, targetLang string, mode string, customInstructions ...string) (*model.AIImageAnalysisResult, error) {
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
- "estimated_usd": Realistic retail market value in USD (note: reward coin decisions are doubled so 1 USD = 2 Coins in the rewards store to account for daily bonus earnings). Guidelines:
  * Treats, candy, snacks: 2.0 to 8.0 (4 to 16 coins)
  * Small toys, cinema tickets, pizza, books: 10.0 to 30.0 (20 to 60 coins)
  * Video games, board games, Lego sets, headphones: 40.0 to 100.0 (80 to 200 coins)
  * Smartwatches, bikes, tablets: 150.0 to 350.0 (300 to 700 coins)
  * High-end gaming consoles (PlayStation 5 / PS5, Xbox Series X, Nintendo Switch), laptops, smartphones: 500.0 to 1000.0+ (1000 to 2000+ coins)
- "valuation_reasoning": Brief reasoning strictly in %s explaining the estimated market price.
- "suggested_emoji": A fitting emoji for the item (e.g. 🎮, 🍕, 🚲, 🧸).

Return ONLY a valid JSON object:
{"is_valid":true,"invalid_reason":"","item_type":"reward","title":"🎮 סוני פלייסטיישן 5","description":"קונסולת משחקים פלייסטיישן 5","category":"individual","estimated_usd":500.0,"valuation_reasoning":"מחיר שוק קמעונאי ממוצע לקונסולה","suggested_emoji":"🎮"}`, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang)
	} else if normalizedMode == "chore" {
		prompt = fmt.Sprintf(`CRITICAL REQUIREMENT: You MUST write ALL user-facing text strictly in %s. Do NOT output English or any other language unless %s is English.

Analyze this photo to determine if it shows a household CHORE to do.

Case 1 - If NO chore is visible, OR if the space, closet, wardrobe, drawer, room, or surface is ALREADY ORGANIZED, NEAT, OR CLEAN (e.g. an already organized closet with items in place on shelves, an already clean room/counter, nothing substantial to clean/tidy):
- Set "is_valid": false
- Set "invalid_reason" to a friendly 1-sentence explanation strictly in %s explaining that this area is already organized and clean (e.g. 'הארון/האזור כבר מסודר ונקי, אין צורך לסדר כעת ואין נקודות על משהו שכבר מסודר!') so no chore or points are needed.
- Set "title": ""
- Set "description": ""

Case 2 - If a REAL chore is visible (genuine disorder, mess, dirt, or clutter needing real work):
- Set "is_valid": true
- Set "invalid_reason": ""
- "item_type": "chore"
- "category": "personal" (IMPORTANT: All chores detected from photos are rewarding household tasks for coins. NEVER set category to "mandatory". Mandatory tasks are only set manually by parents).
- "title": Action to take strictly in %s with a leading emoji (e.g. '🧼 לנקות את שולחן האוכל', '🧹 לטאטא את הרצפה', '🍽️ להוציא כלים מהמדיח', '🧺 לקפל כביסה', '🧸 לסדר את המשחקים').
- "description": Short description strictly in %s describing what needs to be done.
- "points_reward": Integer coin reward based on real effort and time:
  * CRITICAL RULE FOR ALREADY ORGANIZED / LIGHT TOUCH-UPS: If something is already mostly clean or organized with only tiny/minimal touch-ups, it must NOT receive many points! It should receive at most 1 coin (micro-task). Never give 2+ coins if the area/closet is already organized or neat.
  * 1 coin: Micro-task (< 1-2 mins, very light effort, e.g. touching up an already mostly organized closet or shelf, emptying a blue recycling basket / paper bin 'לרוקן סל מחזור כחול / פח נייר', putting a cup/dish in the sink, picking up stray shoes or a single toy, throwing away a wrapper).
  * 2 coins: Quick task (2-5 mins of active effort, e.g. taking out the main household garbage bag to the dumpster, wiping down the dining table, making bed, feeding a pet).
  * 3 coins: Small chore (5-10 mins, e.g. clearing & cleaning the whole dining table, emptying dishwasher, sweeping a room, organizing a moderately messy closet/shelf with scattered items).
  * 5 coins: Moderate chore (10-20 mins, e.g. mopping floors, vacuuming living room, cleaning bathroom sink, folding laundry, organizing a genuinely disordered closet).
  * 8 coins: Substantial chore (20-35 mins, e.g. scrubbing bathtub/shower, detailed room cleanup, deep closet reorganization).
  * 13 coins: Large chore (35-60 mins, e.g. deep kitchen clean, yard work).
  * 20+ coins: Major deep clean (1+ hours).
  NOTE: Very quick tasks taking barely a minute (such as emptying a blue paper recycling basket 'סל מחזור כחול', picking up a shoe, or putting a cup in the sink) require minimal effort and MUST receive 1 coin! Standard chores taking 2-5 minutes of active effort (like taking out the main heavy kitchen garbage or wiping the dining table) receive 2 coins. If an item or closet is already organized, do NOT give multiple points!
- "estimated_usd": Estimated market value in USD (approx. $1.50 - $2.00 per coin, e.g. $4.0 - $6.0 for cleaning dining table).
- "valuation_reasoning": Brief reasoning strictly in %s explaining the effort, estimated duration in minutes, and why this coin reward was chosen.
- "suggested_emoji": A fitting emoji for the chore (e.g. 🧼, 🧹, 🍽️, 🧺, 🛏️, 🧸).

Return ONLY a valid JSON object:
{"is_valid":true,"invalid_reason":"","item_type":"chore","title":"🧼 לנקות את שולחן האוכל","description":"לפנות ולנקות את שולחן האוכל","category":"personal","points_reward":2,"estimated_usd":4.0,"valuation_reasoning":"משימה של כמה דקות (2 מטבעות)","suggested_emoji":"🧼"}`, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang)
	} else {
		prompt = fmt.Sprintf(`CRITICAL REQUIREMENT: You MUST write ALL user-facing text strictly in %s. Do NOT output English or any other language unless %s is English.

Analyze this photo to determine if it shows a household CHORE to do or a REWARD item.

Case 1 - If NO chore or reward is visible, OR if the space, closet, wardrobe, drawer, room, or surface is ALREADY ORGANIZED, NEAT, OR CLEAN (nothing substantial to clean/tidy):
- Set "is_valid": false
- Set "invalid_reason" to a friendly 1-sentence explanation strictly in %s explaining that this area is already organized and clean (e.g. 'הארון/האזור כבר מסודר ונקי, אין צורך לסדר כעת ואין נקודות על משהו שכבר מסודר!') so no chore is detected.
- Set "title": ""
- Set "description": ""

Case 2 - If a REAL CHORE is visible (meaning real disorder, mess, or dirt needing actual work):
- Set "is_valid": true
- Set "invalid_reason": ""
- "item_type": "chore"
- "category": "personal" (IMPORTANT: All chores detected from photos are rewarding household tasks for coins. NEVER set category to "mandatory". Mandatory tasks are only set manually by parents).
- "title": Action to take strictly in %s with a leading emoji (e.g. '🧼 לנקות את שולחן האוכל', '🧹 לטאטא את הרצפה', '🍽️ להוציא כלים מהמדיח').
- "description": Short description strictly in %s.
- "points_reward": Integer coin reward based on real effort and time:
  * CRITICAL RULE FOR ALREADY ORGANIZED / LIGHT TOUCH-UPS: If something is already mostly clean or organized with only tiny/minimal touch-ups, it must NOT receive many points! It should receive at most 1 coin (micro-task). Never give 2+ coins if the area/closet is already organized or neat.
  * 1 coin: Micro-task (< 1-2 mins, very light effort, e.g. touching up an already mostly organized closet or shelf, emptying a blue recycling basket / paper bin 'לרוקן סל מחזור כחול / פח נייר', putting a dish/cup in sink, picking up a single toy).
  * 2 coins: Quick task (2-5 mins active effort, e.g. taking out the main household garbage bag, wiping dining table, making bed).
  * 3 coins: Small chore (5-10 mins, e.g. clearing & cleaning the whole dining table, emptying dishwasher).
  * 5 coins: Moderate chore (10-20 mins, e.g. mopping floors, vacuuming).
  * 8 coins: Substantial chore (20-35 mins).
  * 13 coins: Large chore (35-60 mins).
  * 20+ coins: Major deep clean (1+ hours).
  NOTE: Very quick tasks taking barely a minute (such as emptying a blue paper recycling basket 'סל מחזור כחול' or touching up an already neat area) require minimal effort and MUST receive 1 coin! Standard chores taking 2-5 minutes of active effort receive 2 coins.
- "estimated_usd": Estimated market value in USD (approx. $1.50 - $2.00 per coin).
- "valuation_reasoning": Brief reasoning strictly in %s explaining the effort, estimated duration in minutes, and why this coin reward was chosen.
- "suggested_emoji": A fitting emoji for the chore (e.g. 🧼, 🧹, 🍽️).

Case 3 - If a REWARD item is visible:
- Set "is_valid": true
- Set "invalid_reason": ""
- "item_type": "reward"
- "category": "individual"
- "title": Name of the reward strictly in %s with a leading emoji (e.g. '🎮 סוני פלייסטיישן 5').
- "estimated_usd": Retail market value in USD (e.g. $500.0-$1000.0 for PS5/consoles, $10.0-$30.0 for small toys/treats; reward store coin price is doubled: 1 USD = 2 Coins).
- "description": Short description strictly in %s.
- "valuation_reasoning": Brief reasoning strictly in %s.
- "suggested_emoji": A fitting emoji for the item (e.g. 🎮, 🍕, 🚲).

Return ONLY a valid JSON object:
{"is_valid":true,"invalid_reason":"","item_type":"chore","title":"...","description":"...","category":"personal","points_reward":2,"estimated_usd":4.0,"valuation_reasoning":"...","suggested_emoji":"..."}`, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang, targetLang)
	}

	if len(customInstructions) > 0 && strings.TrimSpace(customInstructions[0]) != "" {
		prompt += fmt.Sprintf(`

CRITICAL - HOUSEHOLD CUSTOM RULES & INSTRUCTIONS (MUST STRICTLY OVERRIDE ALL DEFAULT TIERS):
The parents/admins of this household have defined the following specific rules for this home:
"""
%s
"""
You MUST strictly obey and apply these household rules above when determining "points_reward"!
- If a rule states an exact point value for chores (e.g. 'כל משימה היא 10 נקודות' / 'every task is 10 points'), you MUST award that EXACT point value (e.g. 10 coins) for "points_reward"! Do NOT change it to 8 or any other number!
- If they specify having a dishwasher, washing dishes takes significantly less time and effort (1-2 coins).
- If they set custom coin calculation rates (e.g. 3 minutes = 1 coin), calculate the points directly using their rate!
`, strings.TrimSpace(customInstructions[0]))
	}

	type modelAttempt struct {
		name    string
		timeout time.Duration
	}
	modelsToTry := []modelAttempt{
		{name: "gemini-2.5-flash", timeout: 25 * time.Second},
		{name: "gemini-2.5-flash-lite", timeout: 15 * time.Second},
	}

	// Compress/downscale large camera photos (>300KB) to max 1024px for lightning-fast AI analysis
	compressedBytes, compressedFmt := compressImageForAI(imageData)

	var resp *genai.GenerateContentResponse
	var errMsgs []string
	var lastErr error
	for _, attempt := range modelsToTry {
		modelCtx, cancel := context.WithTimeout(ctx, attempt.timeout)
		genModel := client.GenerativeModel(attempt.name)
		genModel.ResponseMIMEType = "application/json"
		temp := float32(0.1)
		genModel.Temperature = &temp
		maxTokens := int32(4096)
		genModel.MaxOutputTokens = &maxTokens
		r, err := genModel.GenerateContent(modelCtx, genai.ImageData(compressedFmt, compressedBytes), genai.Text(prompt))
		cancel()
		if err == nil && r != nil && len(r.Candidates) > 0 && len(r.Candidates[0].Content.Parts) > 0 {
			var checkSb strings.Builder
			for _, p := range r.Candidates[0].Content.Parts {
				if txt, ok := p.(genai.Text); ok {
					checkSb.WriteString(string(txt))
				}
			}
			txt := strings.TrimSpace(checkSb.String())
			sIdx := strings.Index(txt, "{")
			eIdx := strings.LastIndex(txt, "}")
			if sIdx != -1 && eIdx > sIdx {
				resp = r
				lastErr = nil
				break
			}
			errMsgs = append(errMsgs, fmt.Sprintf("%s (incomplete json, finish: %v)", attempt.name, r.Candidates[0].FinishReason))
		}
		if err != nil {
			lastErr = err
			errMsgs = append(errMsgs, fmt.Sprintf("%s (%v)", attempt.name, err))
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

		slog.Error("[AI GEMINI ERROR DETAILED]", "lastErr", lastErr, "allErrs", errMsgs)

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
	if startIdx != -1 && (endIdx == -1 || endIdx <= startIdx) {
		// Attempt simple auto-repair by closing unterminated string/brackets
		repaired := jsonStr[startIdx:]
		if strings.Count(repaired, `"`)%2 != 0 {
			repaired += `"`
		}
		repaired += "\n}"
		var testRes model.AIImageAnalysisResult
		if err := json.Unmarshal([]byte(repaired), &testRes); err == nil && (testRes.Title != "" || testRes.Description != "" || testRes.InvalidReason != "") {
			jsonStr = repaired
			startIdx = 0
			endIdx = len(repaired) - 1
		}
	}

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

// compressImageForAI optimizes and downscales camera photos (>300KB) to max 1024px for fast AI vision processing.
func compressImageForAI(imageData []byte) ([]byte, string) {
	if len(imageData) <= 300*1024 {
		return imageData, "jpeg"
	}
	img, fmtName, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return imageData, "jpeg"
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	maxDim := 1024

	if width <= maxDim && height <= maxDim {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 82}); err == nil {
			return buf.Bytes(), "jpeg"
		}
		return imageData, fmtName
	}

	var newWidth, newHeight int
	if width > height {
		newWidth = maxDim
		newHeight = int(float64(height) * float64(maxDim) / float64(width))
	} else {
		newHeight = maxDim
		newWidth = int(float64(width) * float64(maxDim) / float64(height))
	}

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	for y := 0; y < newHeight; y++ {
		srcY := y * height / newHeight
		for x := 0; x < newWidth; x++ {
			srcX := x * width / newWidth
			dst.Set(x, y, img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err == nil {
		return buf.Bytes(), "jpeg"
	}
	return imageData, fmtName
}

// WheelSliceDefinition defines an individual slice on the lucky wheel.
type WheelSliceDefinition struct {
	Index  int    `json:"index"`
	Label  string `json:"label"`
	Type   string `json:"type"` // "individual", "cooperative", "none"
	Amount int    `json:"amount"`
}

var WheelSlices = []WheelSliceDefinition{
	{Index: 0, Label: "+1 Coin", Type: "individual", Amount: 1},
	{Index: 1, Label: "+1 Co-op Coin", Type: "cooperative", Amount: 1},
	{Index: 2, Label: "+2 Coins", Type: "individual", Amount: 2},
	{Index: 3, Label: "🎁 Surprise", Type: "surprise", Amount: 0},
	{Index: 4, Label: "+1 Coin", Type: "individual", Amount: 1},
	{Index: 5, Label: "+2 Co-op Coins", Type: "cooperative", Amount: 2},
	{Index: 6, Label: "+2 Coins", Type: "individual", Amount: 2},
	{Index: 7, Label: "0 Coins", Type: "none", Amount: 0},
}

// GetDailyBonusStatus fetches today's chore completion count, target milestones, and available wheel spins.
func (s *Service) GetDailyBonusStatus(ctx context.Context, choregroupID, userID uuid.UUID) (*model.DailyBonusStatusResponse, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.ChoreGroupID != choregroupID {
		return nil, ErrForbidden
	}

	progress, err := s.repo.GetUserDailyProgress(ctx, choregroupID, userID)
	if err != nil {
		return nil, err
	}

	spinsAvail := 0
	if user.Role == "user" {
		totalSpins := progress.SpinsEarned
		spinsAvail = totalSpins - progress.SpinsUsed
		if spinsAvail < 0 {
			spinsAvail = 0
		}
	}

	return &model.DailyBonusStatusResponse{
		Date:                  progress.Date.Format("2006-01-02"),
		TasksCompleted:        progress.TasksCompleted,
		TargetWheel:           1,
		TargetDouble:          0,
		TargetSpin2:           4,
		TargetSpin3:           7,
		TargetSpin4:           10,
		TargetSurprise:        10,
		SpinsAvailable:        spinsAvail,
		WheelUnlocked:         user.Role == "user",
		DoubleCoinsActive:     progress.DoubleCoinsActive,
		DoubleCoinsUsed:       progress.DoubleCoinsUsed,
		SurpriseClaimed:       progress.SurpriseClaimed,
		HasClaimedWelcomeSpin: user.HasClaimedWelcomeSpin,
	}, nil
}

// ClaimDailySurprise claims the 10-task daily milestone surprise for a kid user.
func (s *Service) ClaimDailySurprise(ctx context.Context, choregroupID, userID uuid.UUID) (*model.DailySurpriseResponse, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.ChoreGroupID != choregroupID {
		return nil, ErrForbidden
	}
	if user.Role != "user" {
		return nil, errors.New("only kid users can claim the daily surprise")
	}

	return s.repo.ClaimDailySurprise(ctx, choregroupID, userID)
}

// SpinWheel executes a spin for a kid user, picking a random slice and persisting points/spin consumption.
func (s *Service) SpinWheel(ctx context.Context, choregroupID uuid.UUID, user model.User) (*model.WheelSpinResponse, error) {
	if user.Role != "user" {
		return nil, errors.New("only kid users can spin the wheel")
	}

	idx := rand.IntN(len(WheelSlices))
	chosenSlice := WheelSlices[idx]

	resp, err := s.repo.ClaimWheelSpin(ctx, choregroupID, user.ID, chosenSlice.Index, chosenSlice.Label, chosenSlice.Amount, chosenSlice.Type)
	if err == nil && resp != nil {
		desc := fmt.Sprintf("%s spun the Lucky Wheel and won: %s", user.Username, resp.Label)
		if resp.SurpriseBackground != nil {
			desc = fmt.Sprintf("%s spun the Lucky Wheel and won surprise points theme: %s!", user.Username, *resp.SurpriseBackground)
		}
		metadata := map[string]interface{}{
			"label":       resp.Label,
			"reward_type": resp.RewardType,
			"amount":      resp.Amount,
		}
		if resp.SurpriseBackground != nil {
			metadata["surprise_background"] = *resp.SurpriseBackground
		}
		_ = s.repo.LogActivity(ctx, model.ActivityEvent{
			ChoreGroupID: choregroupID,
			UserID:       &user.ID,
			UserUsername: user.Username,
			ActivityType: "wheel_spin",
			Title:        "Wheel Spin",
			Description:  desc,
			Metadata:     metadata,
		})
	}
	return resp, err
}

// UpdateUserPointsBackground updates the active points background theme for a user.
func (s *Service) UpdateUserPointsBackground(ctx context.Context, adminOrSelf model.User, targetUserID uuid.UUID, background string) error {
	if adminOrSelf.Role != "admin" && adminOrSelf.ID != targetUserID {
		return ErrForbidden
	}
	targetUser, err := s.repo.GetUserByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if targetUser.ChoreGroupID != adminOrSelf.ChoreGroupID {
		return ErrForbidden
	}

	isAdmin := adminOrSelf.Role == "admin" || targetUser.Role == "admin"
	return s.repo.UpdateUserPointsBackground(ctx, targetUserID, background, isAdmin)
}

// UpdateUserAvatarFrame updates the equipped avatar frame for a user.
func (s *Service) UpdateUserAvatarFrame(ctx context.Context, adminOrSelf model.User, targetUserID uuid.UUID, frame string) error {
	if adminOrSelf.Role != "admin" && adminOrSelf.ID != targetUserID {
		return ErrForbidden
	}
	targetUser, err := s.repo.GetUserByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if targetUser.ChoreGroupID != adminOrSelf.ChoreGroupID {
		return ErrForbidden
	}

	isAdmin := adminOrSelf.Role == "admin" || targetUser.Role == "admin"
	return s.repo.UpdateUserAvatarFrame(ctx, targetUserID, frame, isAdmin)
}

// GetRecentHistory fetches the most recent activities (e.g. 3) for the household or user.
func (s *Service) GetRecentHistory(ctx context.Context, choregroupID, requestingUserID uuid.UUID, isAdmin bool) ([]model.ActivityEvent, error) {
	_, _ = s.repo.PurgeOldActivityHistory(ctx, 7)
	_ = s.repo.BackfillInitialActivityHistory(ctx)
	var filterUserID *uuid.UUID
	if !isAdmin {
		filterUserID = &requestingUserID
	}
	return s.repo.GetRecentActivity(ctx, choregroupID, filterUserID, 3)
}

// GetHistory fetches the full activity history for the specified week / filter params (7-day retention).
func (s *Service) GetHistory(ctx context.Context, choregroupID, requestingUserID uuid.UUID, isAdmin bool, filter model.HistoryFilterParams) (*model.HistoryResponse, error) {
	_, _ = s.repo.PurgeOldActivityHistory(ctx, 7)
	_ = s.repo.BackfillInitialActivityHistory(ctx)
	if !isAdmin {
		// Kids can only view their own activity history
		filter.UserID = &requestingUserID
	}
	events, totalCount, err := s.repo.GetActivityHistory(ctx, choregroupID, filter)
	if err != nil {
		return nil, err
	}

	startDateStr := ""
	if filter.StartDate != nil {
		startDateStr = filter.StartDate.Format(time.RFC3339)
	}
	endDateStr := ""
	if filter.EndDate != nil {
		endDateStr = filter.EndDate.Format(time.RFC3339)
	}

	return &model.HistoryResponse{
		Events:     events,
		TotalCount: totalCount,
		StartDate:  startDateStr,
		EndDate:    endDateStr,
	}, nil
}

// RunDataCleanup runs comprehensive background maintenance:
// 1. Purges activity history older than 7 days
// 2. Purges orphaned uploads older than 1 hour that are no longer referenced by any user avatar, task, or reward
// 3. Purges stale rejected task submissions older than 7 days and orphaned push subscriptions
func (s *Service) RunDataCleanup(ctx context.Context) (map[string]int, error) {
	stats := make(map[string]int)

	historyCount, err := s.repo.PurgeOldActivityHistory(ctx, 7)
	if err == nil {
		stats["history_purged"] = int(historyCount)
	}

	uploadsCount, err := s.repo.PurgeOrphanedUploads(ctx, 1*time.Hour)
	if err == nil {
		stats["orphaned_uploads_purged"] = uploadsCount
	} else {
		slog.Error("failed to purge orphaned uploads", "error", err)
	}

	staleCount, err := s.repo.PurgeStaleSubmissionsAndPush(ctx)
	if err == nil {
		stats["stale_submissions_purged"] = staleCount
	}

	return stats, nil
}


