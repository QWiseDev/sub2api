package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type conversationCaptureRepoStub struct {
	turns []service.ConversationTurn
}

func (r *conversationCaptureRepoStub) CreateTurn(_ context.Context, turn *service.ConversationTurn) error {
	r.turns = append(r.turns, *turn)
	return nil
}

func (r *conversationCaptureRepoStub) ListSessions(context.Context, service.ConversationSessionFilter) ([]service.ConversationSessionSummary, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *conversationCaptureRepoStub) ListTurns(context.Context, string) ([]service.ConversationTurn, error) {
	return nil, nil
}

func (r *conversationCaptureRepoStub) CleanupExpired(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func TestOpsErrorLoggerMiddlewareRecordsConversationResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &conversationCaptureRepoStub{}
	recordService := service.NewConversationRecordService(repo, nil)
	router := gin.New()
	router.Use(OpsErrorLoggerMiddleware(nil))
	router.GET("/test", func(c *gin.Context) {
		c.Set(conversationCaptureContextKey, &conversationCaptureState{
			service: recordService,
			input: service.ConversationCaptureInput{
				RequestID: "request-1",
				UserID:    1,
				APIKeyID:  2,
				GroupID:   service.ConversationRecordGroupID,
				Protocol:  service.ContentModerationProtocolOpenAIChat,
				Body:      []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
			},
		})
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, `{"choices":[{"message":{"content":"world"}}]}`)
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Len(t, repo.turns, 1)
	require.Contains(t, repo.turns[0].ResponseBody, "world")
	require.Equal(t, "application/json", repo.turns[0].ContentType)
}

func TestEnableConversationCaptureUsesPromptCacheKeyForGrouping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	groupID := service.ConversationRecordGroupID
	body := []byte(`{"model":"gpt-5","prompt_cache_key":"codex-session","input":[]}`)

	enableConversationCapture(
		c,
		service.NewConversationRecordService(&conversationCaptureRepoStub{}, nil),
		&service.APIKey{ID: 2, UserID: 1, GroupID: &groupID},
		middleware2.AuthSubject{UserID: 1},
		service.ContentModerationProtocolOpenAIResponses,
		"gpt-5",
		body,
	)

	state, ok := getConversationCapture(c)
	require.True(t, ok)
	require.Empty(t, state.input.SessionID)
	require.Equal(t, "codex-session", state.input.PromptCacheKey)
}
