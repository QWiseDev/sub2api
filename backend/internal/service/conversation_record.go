package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/tidwall/gjson"
)

const (
	ConversationRecordGroupID       int64 = 20
	ConversationRecordBodyMaxBytes        = 1 << 20
	conversationRecordRetentionDays       = 7
	conversationCleanupInitialDelay       = time.Minute
	conversationCleanupInterval           = 24 * time.Hour
	conversationCleanupTimeout            = 30 * time.Second
)

var ErrConversationRecordConfigInvalid = infraerrors.BadRequest(
	"CONVERSATION_RECORD_CONFIG_INVALID",
	"conversation record group_ids and api_key_ids must contain positive IDs",
)

type ConversationRecordConfig struct {
	Enabled   bool    `json:"enabled"`
	GroupIDs  []int64 `json:"group_ids"`
	APIKeyIDs []int64 `json:"api_key_ids"`
}

type ConversationCaptureInput struct {
	RequestID        string
	UserID           int64
	Username         string
	UserEmail        string
	APIKeyID         int64
	APIKeyName       string
	GroupID          int64
	GroupName        string
	Provider         string
	Endpoint         string
	Protocol         string
	Model            string
	SessionID        string
	PromptCacheKey   string
	Stream           bool
	Body             []byte
	RequestText      string
	RequestTruncated bool
	StartedAt        time.Time
}

type ConversationResponseCapture struct {
	Body        []byte
	Truncated   bool
	StatusCode  int
	ContentType string
	CompletedAt time.Time
}

type ConversationTurn struct {
	ID                int64     `json:"id"`
	ConversationKey   string    `json:"conversation_key"`
	SessionID         string    `json:"session_id"`
	RequestID         string    `json:"request_id"`
	UserID            int64     `json:"user_id"`
	UsernameSnapshot  string    `json:"username"`
	UserEmailSnapshot string    `json:"user_email"`
	APIKeyID          int64     `json:"api_key_id"`
	APIKeyName        string    `json:"api_key_name"`
	GroupID           int64     `json:"group_id"`
	GroupName         string    `json:"group_name"`
	Provider          string    `json:"provider"`
	Endpoint          string    `json:"endpoint"`
	Protocol          string    `json:"protocol"`
	Model             string    `json:"model"`
	Stream            bool      `json:"stream"`
	StatusCode        int       `json:"status_code"`
	ContentType       string    `json:"content_type"`
	RequestText       string    `json:"request_text"`
	RequestBody       string    `json:"request_body"`
	ResponseBody      string    `json:"response_body"`
	RequestTruncated  bool      `json:"request_truncated"`
	ResponseTruncated bool      `json:"response_truncated"`
	CreatedAt         time.Time `json:"created_at"`
	CompletedAt       time.Time `json:"completed_at"`
}

type ConversationSessionSummary struct {
	ConversationKey string    `json:"conversation_key"`
	SessionID       string    `json:"session_id"`
	UserID          int64     `json:"user_id"`
	Username        string    `json:"username"`
	UserEmail       string    `json:"user_email"`
	APIKeyID        int64     `json:"api_key_id"`
	APIKeyName      string    `json:"api_key_name"`
	GroupID         int64     `json:"group_id"`
	GroupName       string    `json:"group_name"`
	Provider        string    `json:"provider"`
	Endpoint        string    `json:"endpoint"`
	Protocol        string    `json:"protocol"`
	Model           string    `json:"model"`
	TurnCount       int64     `json:"turn_count"`
	LastRequestText string    `json:"last_request_text"`
	FirstActivityAt time.Time `json:"first_activity_at"`
	LastActivityAt  time.Time `json:"last_activity_at"`
}

type ConversationSessionFilter struct {
	Pagination pagination.PaginationParams
	Search     string
	UserID     *int64
	APIKeyID   *int64
	Model      string
	From       *time.Time
	To         *time.Time
}

type ConversationRecordRepository interface {
	CreateTurn(ctx context.Context, turn *ConversationTurn) error
	ListSessions(ctx context.Context, filter ConversationSessionFilter) ([]ConversationSessionSummary, *pagination.PaginationResult, error)
	ListTurns(ctx context.Context, conversationKey string) ([]ConversationTurn, error)
	CleanupExpired(ctx context.Context, before time.Time) (int64, error)
}

type ConversationRecordService struct {
	repo        ConversationRecordRepository
	settingRepo SettingRepository
	config      atomic.Pointer[ConversationRecordConfig]
	configMu    sync.Mutex
	startOnce   sync.Once
}

func NewConversationRecordService(repo ConversationRecordRepository, settingRepo SettingRepository) *ConversationRecordService {
	return &ConversationRecordService{repo: repo, settingRepo: settingRepo}
}

func ProvideConversationRecordService(repo ConversationRecordRepository, settingRepo SettingRepository) *ConversationRecordService {
	svc := NewConversationRecordService(repo, settingRepo)
	svc.StartCleanup()
	return svc
}

func (s *ConversationRecordService) RecordTurn(ctx context.Context, input ConversationCaptureInput, response ConversationResponseCapture) error {
	if s == nil || s.repo == nil || !s.ShouldCapture(ctx, input.GroupID, input.APIKeyID) {
		return nil
	}
	if input.UserID <= 0 || input.APIKeyID <= 0 {
		return errors.New("conversation record identity is incomplete")
	}
	requestBody, requestTruncated := conversationPersistentText(input.Body, ConversationRecordBodyMaxBytes)
	requestTruncated = requestTruncated || input.RequestTruncated
	requestText, requestTextTruncated := conversationPersistentText([]byte(conversationRequestText(input)), ConversationRecordBodyMaxBytes)
	requestTruncated = requestTruncated || requestTextTruncated
	responseBody, responseTruncated := conversationPersistentText(response.Body, ConversationRecordBodyMaxBytes)
	startedAt := input.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	completedAt := response.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	turn := &ConversationTurn{
		ConversationKey:   BuildConversationKey(input),
		SessionID:         sanitizeSessionID(input.SessionID),
		RequestID:         strings.TrimSpace(input.RequestID),
		UserID:            input.UserID,
		UsernameSnapshot:  strings.TrimSpace(input.Username),
		UserEmailSnapshot: strings.TrimSpace(input.UserEmail),
		APIKeyID:          input.APIKeyID,
		APIKeyName:        strings.TrimSpace(input.APIKeyName),
		GroupID:           input.GroupID,
		GroupName:         strings.TrimSpace(input.GroupName),
		Provider:          strings.TrimSpace(input.Provider),
		Endpoint:          strings.TrimSpace(input.Endpoint),
		Protocol:          strings.TrimSpace(input.Protocol),
		Model:             strings.TrimSpace(input.Model),
		Stream:            input.Stream,
		StatusCode:        response.StatusCode,
		ContentType:       strings.TrimSpace(response.ContentType),
		RequestText:       requestText,
		RequestBody:       requestBody,
		ResponseBody:      responseBody,
		RequestTruncated:  requestTruncated,
		ResponseTruncated: response.Truncated || responseTruncated,
		CreatedAt:         startedAt,
		CompletedAt:       completedAt,
	}
	return s.repo.CreateTurn(ctx, turn)
}

func (s *ConversationRecordService) GetConfig(ctx context.Context) (*ConversationRecordConfig, error) {
	config, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return cloneConversationRecordConfig(config), nil
}

func (s *ConversationRecordService) UpdateConfig(ctx context.Context, input ConversationRecordConfig) (*ConversationRecordConfig, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("conversation record setting repository unavailable")
	}
	config, err := normalizeConversationRecordConfig(input)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal conversation record config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyConversationRecordConfig, string(raw)); err != nil {
		return nil, fmt.Errorf("save conversation record config: %w", err)
	}
	s.config.Store(config)
	return cloneConversationRecordConfig(config), nil
}

func (s *ConversationRecordService) ShouldCapture(ctx context.Context, groupID, apiKeyID int64) bool {
	if s == nil || apiKeyID <= 0 {
		return false
	}
	config, err := s.loadConfig(ctx)
	if err != nil {
		slog.Warn("conversation_record.config_load_failed", "error", err)
		return false
	}
	if !config.Enabled {
		return false
	}
	return conversationRecordContainsID(config.GroupIDs, groupID) ||
		conversationRecordContainsID(config.APIKeyIDs, apiKeyID)
}

func (s *ConversationRecordService) loadConfig(ctx context.Context) (*ConversationRecordConfig, error) {
	if s == nil {
		return nil, errors.New("conversation record service unavailable")
	}
	if cached := s.config.Load(); cached != nil {
		return cached, nil
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()
	if cached := s.config.Load(); cached != nil {
		return cached, nil
	}

	config := defaultConversationRecordConfig()
	if s.settingRepo == nil {
		s.config.Store(config)
		return config, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyConversationRecordConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			s.config.Store(config)
			return config, nil
		}
		return nil, fmt.Errorf("load conversation record config: %w", err)
	}
	if err := json.Unmarshal([]byte(raw), config); err != nil {
		slog.Warn("conversation_record.config_invalid_json", "error", err)
		config = &ConversationRecordConfig{}
		s.config.Store(config)
		return config, nil
	}
	normalized, err := normalizeConversationRecordConfig(*config)
	if err != nil {
		slog.Warn("conversation_record.config_invalid_scope", "error", err)
		config = &ConversationRecordConfig{}
		s.config.Store(config)
		return config, nil
	}
	s.config.Store(normalized)
	return normalized, nil
}

func defaultConversationRecordConfig() *ConversationRecordConfig {
	return &ConversationRecordConfig{
		Enabled:  true,
		GroupIDs: []int64{ConversationRecordGroupID},
	}
}

func normalizeConversationRecordConfig(input ConversationRecordConfig) (*ConversationRecordConfig, error) {
	groupIDs, err := normalizeConversationRecordIDs(input.GroupIDs)
	if err != nil {
		return nil, err
	}
	apiKeyIDs, err := normalizeConversationRecordIDs(input.APIKeyIDs)
	if err != nil {
		return nil, err
	}
	return &ConversationRecordConfig{
		Enabled:   input.Enabled,
		GroupIDs:  groupIDs,
		APIKeyIDs: apiKeyIDs,
	}, nil
}

func normalizeConversationRecordIDs(values []int64) ([]int64, error) {
	unique := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return nil, ErrConversationRecordConfigInvalid
		}
		unique[value] = struct{}{}
	}
	result := make([]int64, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func cloneConversationRecordConfig(config *ConversationRecordConfig) *ConversationRecordConfig {
	if config == nil {
		return nil
	}
	return &ConversationRecordConfig{
		Enabled:   config.Enabled,
		GroupIDs:  append([]int64{}, config.GroupIDs...),
		APIKeyIDs: append([]int64{}, config.APIKeyIDs...),
	}
}

func conversationRecordContainsID(values []int64, target int64) bool {
	if target <= 0 {
		return false
	}
	index := sort.Search(len(values), func(i int) bool { return values[i] >= target })
	return index < len(values) && values[index] == target
}

func (s *ConversationRecordService) ListSessions(ctx context.Context, filter ConversationSessionFilter) ([]ConversationSessionSummary, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, errors.New("conversation record repository unavailable")
	}
	return s.repo.ListSessions(ctx, filter)
}

func (s *ConversationRecordService) ListTurns(ctx context.Context, conversationKey string) ([]ConversationTurn, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("conversation record repository unavailable")
	}
	conversationKey = strings.TrimSpace(conversationKey)
	if conversationKey == "" {
		return nil, errors.New("conversation key is required")
	}
	return s.repo.ListTurns(ctx, conversationKey)
}

func BuildConversationKey(input ConversationCaptureInput) string {
	sessionID := sanitizeSessionID(input.SessionID)
	seedKind := "session"
	seed := sessionID
	if seed == "" {
		seedKind = "prompt_cache_key"
		seed = strings.TrimSpace(input.PromptCacheKey)
	}
	if seed == "" {
		seedKind = "request"
		seed = strings.TrimSpace(input.RequestID)
	}
	if seed == "" {
		seed = strings.TrimSpace(input.Endpoint) + ":" + input.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"conversation:v1", seedKind, seed,
		formatConversationID(input.UserID), formatConversationID(input.GroupID),
	}, ":")))
	return hex.EncodeToString(digest[:])
}

// ExtractConversationPromptCacheKey returns the OpenAI request seed used only
// for conversation grouping. The raw value is never persisted as session_id.
func ExtractConversationPromptCacheKey(protocol string, body []byte) string {
	switch strings.TrimSpace(protocol) {
	case ContentModerationProtocolOpenAIChat, ContentModerationProtocolOpenAIResponses:
		return strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String())
	default:
		return ""
	}
}

func formatConversationID(value int64) string {
	return strconv.FormatInt(value, 10)
}

func conversationPersistentText(raw []byte, limit int) (string, bool) {
	truncated := len(raw) > limit
	if truncated {
		raw = raw[:limit]
	}
	return conversationCleanText(string(raw)), truncated
}

func conversationCleanText(value string) string {
	return strings.ToValidUTF8(strings.ReplaceAll(value, "\x00", ""), "")
}

func conversationRequestText(input ConversationCaptureInput) string {
	if value := strings.TrimSpace(input.RequestText); value != "" {
		return conversationCleanText(value)
	}
	return conversationCleanText(ExtractContentModerationText(input.Protocol, input.Body))
}

func (s *ConversationRecordService) StartCleanup() {
	if s == nil || s.repo == nil {
		return
	}
	s.startOnce.Do(func() {
		go s.cleanupWorker()
	})
}

func (s *ConversationRecordService) cleanupWorker() {
	timer := time.NewTimer(conversationCleanupInitialDelay)
	defer timer.Stop()
	for {
		<-timer.C
		s.runCleanupOnce()
		timer.Reset(conversationCleanupInterval)
	}
}

func (s *ConversationRecordService) runCleanupOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), conversationCleanupTimeout)
	defer cancel()
	before := time.Now().AddDate(0, 0, -conversationRecordRetentionDays)
	deleted, err := s.repo.CleanupExpired(ctx, before)
	if err != nil {
		slog.Warn("conversation_record.cleanup_failed", "error", err)
		return
	}
	if deleted > 0 {
		slog.Info("conversation_record.cleanup_completed", "deleted", deleted)
	}
}
