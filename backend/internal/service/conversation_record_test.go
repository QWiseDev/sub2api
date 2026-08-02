package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type conversationRecordRepoStub struct {
	turns []ConversationTurn
}

type conversationRecordSettingRepoStub struct {
	values map[string]string
	setErr error
}

func (r *conversationRecordSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *conversationRecordSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (r *conversationRecordSettingRepoStub) Set(_ context.Context, key, value string) error {
	if r.setErr != nil {
		return r.setErr
	}
	if r.values == nil {
		r.values = make(map[string]string)
	}
	r.values[key] = value
	return nil
}

func (r *conversationRecordSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}

func (r *conversationRecordSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (r *conversationRecordSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return nil, nil
}

func (r *conversationRecordSettingRepoStub) Delete(context.Context, string) error {
	return nil
}

func (r *conversationRecordRepoStub) CreateTurn(_ context.Context, turn *ConversationTurn) error {
	r.turns = append(r.turns, *turn)
	return nil
}

func (r *conversationRecordRepoStub) ListSessions(context.Context, ConversationSessionFilter) ([]ConversationSessionSummary, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *conversationRecordRepoStub) ListTurns(context.Context, string) ([]ConversationTurn, error) {
	return nil, nil
}

func (r *conversationRecordRepoStub) CleanupExpired(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func TestBuildConversationKeyUsesExplicitSession(t *testing.T) {
	base := ConversationCaptureInput{
		UserID:         1,
		APIKeyID:       2,
		GroupID:        ConversationRecordGroupID,
		SessionID:      "session-1",
		PromptCacheKey: "prompt-cache-1",
	}
	first := BuildConversationKey(base)
	base.RequestID = "another-request"
	require.Equal(t, first, BuildConversationKey(base))
	base.PromptCacheKey = "another-prompt-cache"
	require.Equal(t, first, BuildConversationKey(base))
	base.APIKeyID = 99
	require.Equal(t, first, BuildConversationKey(base))

	base.SessionID = ""
	base.PromptCacheKey = "prompt-cache-1"
	base.RequestID = "request-1"
	require.NotEqual(t, first, BuildConversationKey(base))
	promptCacheKey := BuildConversationKey(base)
	base.RequestID = "request-2"
	require.Equal(t, promptCacheKey, BuildConversationKey(base))
	base.PromptCacheKey = "prompt-cache-2"
	require.NotEqual(t, promptCacheKey, BuildConversationKey(base))

	base.PromptCacheKey = ""
	base.RequestID = "request-1"
	requestKey := BuildConversationKey(base)
	base.RequestID = "request-2"
	require.NotEqual(t, requestKey, BuildConversationKey(base))
}

func TestBuildConversationKeyIsolatesPromptCacheKeyByUserAndGroup(t *testing.T) {
	base := ConversationCaptureInput{UserID: 1, GroupID: 20, PromptCacheKey: "codex-session"}
	key := BuildConversationKey(base)

	base.UserID = 2
	require.NotEqual(t, key, BuildConversationKey(base))
	base.UserID = 1
	base.GroupID = 21
	require.NotEqual(t, key, BuildConversationKey(base))
}

func TestExtractConversationPromptCacheKey(t *testing.T) {
	body := []byte(`{"model":"gpt-5","prompt_cache_key":" codex-session ","input":[]}`)

	require.Equal(t, "codex-session", ExtractConversationPromptCacheKey(ContentModerationProtocolOpenAIResponses, body))
	require.Equal(t, "codex-session", ExtractConversationPromptCacheKey(ContentModerationProtocolOpenAIChat, body))
	require.Empty(t, ExtractConversationPromptCacheKey(ContentModerationProtocolAnthropicMessages, body))
	require.Empty(t, ExtractConversationPromptCacheKey(ContentModerationProtocolOpenAIResponses, []byte(`{"model":"gpt-5"}`)))
}

func TestConversationRecordServiceRecordsAndTruncatesTurn(t *testing.T) {
	repo := &conversationRecordRepoStub{}
	svc := NewConversationRecordService(repo, nil)
	requestBody := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	responseBody := []byte(strings.Repeat("x", ConversationRecordBodyMaxBytes+10))

	err := svc.RecordTurn(context.Background(), ConversationCaptureInput{
		RequestID: "request-1",
		UserID:    1,
		APIKeyID:  2,
		GroupID:   ConversationRecordGroupID,
		Protocol:  ContentModerationProtocolOpenAIChat,
		SessionID: "session-1",
		Body:      requestBody,
		StartedAt: time.Now(),
	}, ConversationResponseCapture{Body: responseBody, StatusCode: 200})
	require.NoError(t, err)
	require.Len(t, repo.turns, 1)
	require.Equal(t, "hello", repo.turns[0].RequestText)
	require.Len(t, repo.turns[0].ResponseBody, ConversationRecordBodyMaxBytes)
	require.True(t, repo.turns[0].ResponseTruncated)
}

func TestConversationRecordServiceGroupsWithoutPersistingPromptCacheKey(t *testing.T) {
	repo := &conversationRecordRepoStub{}
	svc := NewConversationRecordService(repo, nil)
	input := ConversationCaptureInput{
		RequestID:      "request-1",
		UserID:         1,
		APIKeyID:       2,
		GroupID:        ConversationRecordGroupID,
		PromptCacheKey: "codex-session",
	}

	require.NoError(t, svc.RecordTurn(context.Background(), input, ConversationResponseCapture{}))
	input.RequestID = "request-2"
	require.NoError(t, svc.RecordTurn(context.Background(), input, ConversationResponseCapture{}))

	require.Len(t, repo.turns, 2)
	require.Empty(t, repo.turns[0].SessionID)
	require.Empty(t, repo.turns[1].SessionID)
	require.Equal(t, repo.turns[0].ConversationKey, repo.turns[1].ConversationKey)
}

func TestConversationRecordServiceSkipsOtherGroups(t *testing.T) {
	repo := &conversationRecordRepoStub{}
	svc := NewConversationRecordService(repo, nil)
	require.NoError(t, svc.RecordTurn(context.Background(), ConversationCaptureInput{UserID: 1, APIKeyID: 2, GroupID: 21}, ConversationResponseCapture{}))
	require.Empty(t, repo.turns)
}

func TestConversationRecordConfigDefaultsToLegacyGroup(t *testing.T) {
	svc := NewConversationRecordService(&conversationRecordRepoStub{}, &conversationRecordSettingRepoStub{})
	config, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.True(t, config.Enabled)
	require.Equal(t, []int64{ConversationRecordGroupID}, config.GroupIDs)
	require.Empty(t, config.APIKeyIDs)
	require.True(t, svc.ShouldCapture(context.Background(), ConversationRecordGroupID, 99))
	require.False(t, svc.ShouldCapture(context.Background(), 21, 99))
}

func TestConversationRecordConfigLoadsPersistedScope(t *testing.T) {
	settings := &conversationRecordSettingRepoStub{values: map[string]string{
		SettingKeyConversationRecordConfig: `{"enabled":true,"group_ids":[31,30,31],"api_key_ids":[8,7]}`,
	}}
	svc := NewConversationRecordService(&conversationRecordRepoStub{}, settings)

	config, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, []int64{30, 31}, config.GroupIDs)
	require.Equal(t, []int64{7, 8}, config.APIKeyIDs)
	require.True(t, svc.ShouldCapture(context.Background(), 99, 7))
}

func TestConversationRecordConfigInvalidStoredValueFailsClosed(t *testing.T) {
	settings := &conversationRecordSettingRepoStub{values: map[string]string{
		SettingKeyConversationRecordConfig: `{invalid`,
	}}
	svc := NewConversationRecordService(&conversationRecordRepoStub{}, settings)

	config, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.False(t, config.Enabled)
	require.Empty(t, config.GroupIDs)
	require.Empty(t, config.APIKeyIDs)
	require.False(t, svc.ShouldCapture(context.Background(), ConversationRecordGroupID, 1))
}

func TestConversationRecordConfigMatchesGroupsOrAPIKeys(t *testing.T) {
	settings := &conversationRecordSettingRepoStub{}
	svc := NewConversationRecordService(&conversationRecordRepoStub{}, settings)

	config, err := svc.UpdateConfig(context.Background(), ConversationRecordConfig{
		Enabled:   true,
		GroupIDs:  []int64{30, 20, 30},
		APIKeyIDs: []int64{9, 7, 9},
	})
	require.NoError(t, err)
	require.Equal(t, []int64{20, 30}, config.GroupIDs)
	require.Equal(t, []int64{7, 9}, config.APIKeyIDs)
	require.True(t, svc.ShouldCapture(context.Background(), 20, 1))
	require.True(t, svc.ShouldCapture(context.Background(), 99, 7))
	require.False(t, svc.ShouldCapture(context.Background(), 99, 8))
	require.Contains(t, settings.values[SettingKeyConversationRecordConfig], `"api_key_ids":[7,9]`)
}

func TestConversationRecordConfigDisabledOrEmptyScopeSkipsCapture(t *testing.T) {
	settings := &conversationRecordSettingRepoStub{}
	svc := NewConversationRecordService(&conversationRecordRepoStub{}, settings)

	_, err := svc.UpdateConfig(context.Background(), ConversationRecordConfig{Enabled: false, GroupIDs: []int64{20}})
	require.NoError(t, err)
	require.False(t, svc.ShouldCapture(context.Background(), 20, 1))

	_, err = svc.UpdateConfig(context.Background(), ConversationRecordConfig{Enabled: true})
	require.NoError(t, err)
	require.False(t, svc.ShouldCapture(context.Background(), 20, 1))
}

func TestConversationRecordConfigSerializesEmptyScopeAsArrays(t *testing.T) {
	svc := NewConversationRecordService(&conversationRecordRepoStub{}, &conversationRecordSettingRepoStub{})

	config, err := svc.UpdateConfig(context.Background(), ConversationRecordConfig{Enabled: false})
	require.NoError(t, err)
	require.NotNil(t, config.GroupIDs)
	require.NotNil(t, config.APIKeyIDs)

	raw, err := json.Marshal(config)
	require.NoError(t, err)
	require.JSONEq(t, `{"enabled":false,"group_ids":[],"api_key_ids":[]}`, string(raw))
}

func TestConversationRecordConfigRejectsInvalidIDsAndWriteErrors(t *testing.T) {
	settings := &conversationRecordSettingRepoStub{}
	svc := NewConversationRecordService(&conversationRecordRepoStub{}, settings)

	_, err := svc.UpdateConfig(context.Background(), ConversationRecordConfig{Enabled: true, GroupIDs: []int64{0}})
	require.ErrorIs(t, err, ErrConversationRecordConfigInvalid)

	settings.setErr = errors.New("write failed")
	_, err = svc.UpdateConfig(context.Background(), ConversationRecordConfig{Enabled: true, GroupIDs: []int64{20}})
	require.ErrorContains(t, err, "save conversation record config")
}
