package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type conversationRecordRepository struct {
	db *sql.DB
}

func NewConversationRecordRepository(db *sql.DB) service.ConversationRecordRepository {
	return &conversationRecordRepository{db: db}
}

func (r *conversationRecordRepository) CreateTurn(ctx context.Context, turn *service.ConversationTurn) error {
	if turn == nil {
		return nil
	}
	err := r.db.QueryRowContext(ctx, `
INSERT INTO conversation_turns (
    conversation_key, session_id, request_id,
    user_id, username_snapshot, user_email_snapshot,
    api_key_id, api_key_name, group_id, group_name,
    provider, endpoint, protocol, model, stream,
    status_code, content_type, request_text, request_body, response_body,
    request_truncated, response_truncated, created_at, completed_at
) VALUES (
    $1, $2, $3,
    $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14, $15,
    $16, $17, $18, $19, $20,
    $21, $22, $23, $24
)
RETURNING id, created_at, completed_at`,
		turn.ConversationKey, turn.SessionID, turn.RequestID,
		turn.UserID, turn.UsernameSnapshot, turn.UserEmailSnapshot,
		turn.APIKeyID, turn.APIKeyName, turn.GroupID, turn.GroupName,
		turn.Provider, turn.Endpoint, turn.Protocol, turn.Model, turn.Stream,
		turn.StatusCode, turn.ContentType, turn.RequestText, turn.RequestBody, turn.ResponseBody,
		turn.RequestTruncated, turn.ResponseTruncated, turn.CreatedAt, turn.CompletedAt,
	).Scan(&turn.ID, &turn.CreatedAt, &turn.CompletedAt)
	if err != nil {
		return fmt.Errorf("insert conversation turn: %w", err)
	}
	return nil
}

func (r *conversationRecordRepository) ListSessions(ctx context.Context, filter service.ConversationSessionFilter) ([]service.ConversationSessionSummary, *pagination.PaginationResult, error) {
	where, args := buildConversationSessionWhere(filter)
	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT t.conversation_key) FROM conversation_turns t `+whereSQL, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count conversation sessions: %w", err)
	}

	params := filter.Pagination
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, params.Limit(), params.Offset())
	limitArg := len(queryArgs) - 1
	offsetArg := len(queryArgs)

	rows, err := r.db.QueryContext(ctx, `
WITH filtered AS (
    SELECT t.conversation_key, t.created_at, t.completed_at
    FROM conversation_turns t
    `+whereSQL+`
), grouped AS (
    SELECT
        conversation_key,
        COUNT(*) AS turn_count,
        MIN(created_at) AS first_activity_at,
        MAX(completed_at) AS last_activity_at
    FROM filtered
    GROUP BY conversation_key
    ORDER BY last_activity_at DESC, conversation_key DESC
    LIMIT $`+fmt.Sprint(limitArg)+` OFFSET $`+fmt.Sprint(offsetArg)+`
)
SELECT
    g.conversation_key,
    latest.session_id,
    latest.user_id,
    latest.username_snapshot,
    latest.user_email_snapshot,
    latest.api_key_id,
    latest.api_key_name,
    latest.group_id,
    latest.group_name,
    latest.provider,
    latest.endpoint,
    latest.protocol,
    latest.model,
    g.turn_count,
    LEFT(latest.request_text, 500),
    g.first_activity_at,
    g.last_activity_at
FROM grouped g
JOIN LATERAL (
    SELECT t.*
    FROM conversation_turns t
    WHERE t.conversation_key = g.conversation_key
    ORDER BY t.completed_at DESC, t.id DESC
    LIMIT 1
) latest ON TRUE
ORDER BY g.last_activity_at DESC, g.conversation_key DESC`, queryArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("list conversation sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.ConversationSessionSummary, 0)
	for rows.Next() {
		var item service.ConversationSessionSummary
		if err := rows.Scan(
			&item.ConversationKey,
			&item.SessionID,
			&item.UserID,
			&item.Username,
			&item.UserEmail,
			&item.APIKeyID,
			&item.APIKeyName,
			&item.GroupID,
			&item.GroupName,
			&item.Provider,
			&item.Endpoint,
			&item.Protocol,
			&item.Model,
			&item.TurnCount,
			&item.LastRequestText,
			&item.FirstActivityAt,
			&item.LastActivityAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan conversation session: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate conversation sessions: %w", err)
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *conversationRecordRepository) ListTurns(ctx context.Context, conversationKey string) ([]service.ConversationTurn, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    id, conversation_key, session_id, request_id,
    user_id, username_snapshot, user_email_snapshot,
    api_key_id, api_key_name, group_id, group_name,
    provider, endpoint, protocol, model, stream,
    status_code, content_type, request_text, request_body, response_body,
    request_truncated, response_truncated, created_at, completed_at
FROM conversation_turns
WHERE conversation_key = $1
ORDER BY created_at ASC, id ASC`, conversationKey)
	if err != nil {
		return nil, fmt.Errorf("list conversation turns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.ConversationTurn, 0)
	for rows.Next() {
		var item service.ConversationTurn
		if err := rows.Scan(
			&item.ID,
			&item.ConversationKey,
			&item.SessionID,
			&item.RequestID,
			&item.UserID,
			&item.UsernameSnapshot,
			&item.UserEmailSnapshot,
			&item.APIKeyID,
			&item.APIKeyName,
			&item.GroupID,
			&item.GroupName,
			&item.Provider,
			&item.Endpoint,
			&item.Protocol,
			&item.Model,
			&item.Stream,
			&item.StatusCode,
			&item.ContentType,
			&item.RequestText,
			&item.RequestBody,
			&item.ResponseBody,
			&item.RequestTruncated,
			&item.ResponseTruncated,
			&item.CreatedAt,
			&item.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan conversation turn: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversation turns: %w", err)
	}
	return items, nil
}

func (r *conversationRecordRepository) CleanupExpired(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM conversation_turns WHERE completed_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired conversation turns: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read deleted conversation turn count: %w", err)
	}
	return deleted, nil
}

func buildConversationSessionWhere(filter service.ConversationSessionFilter) ([]string, []any) {
	where := []string{"t.id IS NOT NULL"}
	args := make([]any, 0)
	add := func(expr string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(expr, len(args)))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + search + "%"
		args = append(args, like, like, like, like, like, like, like, like)
		first := len(args) - 7
		where = append(where, fmt.Sprintf(`(
            t.conversation_key ILIKE $%d OR t.session_id ILIKE $%d OR t.request_id ILIKE $%d OR
            t.username_snapshot ILIKE $%d OR t.user_email_snapshot ILIKE $%d OR
            t.api_key_name ILIKE $%d OR t.model ILIKE $%d OR t.request_text ILIKE $%d
        )`, first, first+1, first+2, first+3, first+4, first+5, first+6, first+7))
	}
	if filter.UserID != nil {
		add("t.user_id = $%d", *filter.UserID)
	}
	if filter.APIKeyID != nil {
		add("t.api_key_id = $%d", *filter.APIKeyID)
	}
	if model := strings.TrimSpace(filter.Model); model != "" {
		add("t.model = $%d", model)
	}
	if filter.From != nil && !filter.From.IsZero() {
		add("t.created_at >= $%d", *filter.From)
	}
	if filter.To != nil && !filter.To.IsZero() {
		add("t.created_at <= $%d", *filter.To)
	}
	return where, args
}
