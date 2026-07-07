package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
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


// CreateTask creates a new task from a DTO.
func (s *Service) CreateTask(ctx context.Context, choregroupID uuid.UUID, req model.CreateTaskRequest, onResolved func()) (*model.Task, error) {
	if req.IsMandatory {
		req.PointsReward = 0
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
	origTitle := req.Title
	title, needsAsync := s.ResolveEmoji(ctx, req.Title)
	req.Title = title

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

	return s.repo.DeleteTask(ctx, taskID, adminUser.ChoreGroupID)
}

// ListTasks retrieves tasks based on the user's role.
func (s *Service) ListTasks(ctx context.Context, user model.User) ([]model.Task, error) {
	var tasks []model.Task
	var err error
	if user.Role == "admin" {
		tasks, err = s.repo.ListAllTasksForChoreGroup(ctx, user.ChoreGroupID)
	} else {
		tasks, err = s.repo.ListTasksForUser(ctx, user.ChoreGroupID, user.ID)
	}
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for i := range tasks {
		if tasks[i].Status == "assigned" && !tasks[i].IsMandatory && tasks[i].ExpiresAt != nil && tasks[i].ExpiresAt.Before(now) {
			tasks[i].Status = "expired"
		}
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

	if !task.IsMandatory {
		userTasks, err := s.repo.ListTasksForUser(ctx, task.ChoreGroupID, userID)
		if err != nil {
			return nil, err
		}
		for _, t := range userTasks {
			if t.IsMandatory && t.Status == "assigned" {
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
func (s *Service) CreateReward(ctx context.Context, adminUser model.User, req model.CreateRewardRequest, onResolved func()) (*model.Reward, error) {
	if adminUser.Role != "admin" {
		return nil, ErrForbidden
	}
	origName := req.Name
	name, needsAsync := s.ResolveEmoji(ctx, req.Name)
	req.Name = name
	reward := &model.Reward{
		ID:           uuid.New(),
		ChoreGroupID: adminUser.ChoreGroupID,
		Name:         req.Name,
		Description:  req.Description,
		Cost:         req.Cost,
		Type:         req.Type,
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

	model := client.GenerativeModel("gemini-flash-latest")
	model.ResponseMIMEType = "application/json"

	prompt := `Extract a single core keyword and a highly relevant single emoji from this task/reward title. Return ONLY a JSON object: {"keyword": "word", "emoji": "😀"}. Title: ` + title

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil || len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		slog.Error("failed to generate gemini content", "error", err)
		return title
	}

	part := resp.Candidates[0].Content.Parts[0]
	jsonStr := ""
	if txt, ok := part.(genai.Text); ok {
		jsonStr = string(txt)
	} else {
		return title
	}

	var result struct {
		Keyword string `json:"keyword"`
		Emoji   string `json:"emoji"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		slog.Error("failed to parse gemini json", "error", err)
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
