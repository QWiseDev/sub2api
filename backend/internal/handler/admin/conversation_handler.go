package admin

import (
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ConversationHandler struct {
	service *service.ConversationRecordService
}

func NewConversationHandler(recordService *service.ConversationRecordService) *ConversationHandler {
	return &ConversationHandler{service: recordService}
}

func (h *ConversationHandler) GetConfig(c *gin.Context) {
	config, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

func (h *ConversationHandler) UpdateConfig(c *gin.Context) {
	var input service.ConversationRecordConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	config, err := h.service.UpdateConfig(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

func (h *ConversationHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.ConversationSessionFilter{
		Pagination: pagination.PaginationParams{
			Page:      page,
			PageSize:  pageSize,
			SortOrder: pagination.SortOrderDesc,
		},
		Search: strings.TrimSpace(c.Query("search")),
		Model:  strings.TrimSpace(c.Query("model")),
	}
	if raw := strings.TrimSpace(c.Query("user_id")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		filter.UserID = &value
	}
	if raw := strings.TrimSpace(c.Query("api_key_id")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}
		filter.APIKeyID = &value
	}
	if raw := strings.TrimSpace(c.Query("from")); raw != "" {
		value, _, err := parseContentModerationDate(raw)
		if err != nil {
			response.BadRequest(c, "Invalid from")
			return
		}
		filter.From = &value
	}
	if raw := strings.TrimSpace(c.Query("to")); raw != "" {
		value, dateOnly, err := parseContentModerationDate(raw)
		if err != nil {
			response.BadRequest(c, "Invalid to")
			return
		}
		if dateOnly {
			value = value.Add(24*time.Hour - time.Nanosecond)
		}
		filter.To = &value
	}

	items, result, err := h.service.ListSessions(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *ConversationHandler) Get(c *gin.Context) {
	conversationKey := strings.TrimSpace(c.Param("conversation_key"))
	if len(conversationKey) != 64 {
		response.BadRequest(c, "Invalid conversation_key")
		return
	}
	if _, err := hex.DecodeString(conversationKey); err != nil {
		response.BadRequest(c, "Invalid conversation_key")
		return
	}
	items, err := h.service.ListTurns(c.Request.Context(), conversationKey)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
