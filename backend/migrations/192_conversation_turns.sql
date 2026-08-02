-- 管理员会话记录：每次 HTTP 模型调用保存为一个 turn，按 conversation_key 聚合展示。
CREATE TABLE IF NOT EXISTS conversation_turns (
    id                  BIGSERIAL PRIMARY KEY,
    conversation_key    VARCHAR(64) NOT NULL,
    session_id          VARCHAR(255) NOT NULL DEFAULT '',
    request_id          VARCHAR(128) NOT NULL DEFAULT '',
    user_id             BIGINT NOT NULL,
    username_snapshot   VARCHAR(255) NOT NULL DEFAULT '',
    user_email_snapshot VARCHAR(255) NOT NULL DEFAULT '',
    api_key_id          BIGINT NOT NULL,
    api_key_name        VARCHAR(255) NOT NULL DEFAULT '',
    group_id            BIGINT NOT NULL,
    group_name          VARCHAR(255) NOT NULL DEFAULT '',
    provider            VARCHAR(64) NOT NULL DEFAULT '',
    endpoint            VARCHAR(255) NOT NULL DEFAULT '',
    protocol            VARCHAR(64) NOT NULL DEFAULT '',
    model               VARCHAR(255) NOT NULL DEFAULT '',
    stream              BOOLEAN NOT NULL DEFAULT FALSE,
    status_code         INT NOT NULL DEFAULT 0,
    content_type        VARCHAR(255) NOT NULL DEFAULT '',
    request_text        TEXT NOT NULL DEFAULT '',
    request_body        TEXT NOT NULL DEFAULT '',
    response_body       TEXT NOT NULL DEFAULT '',
    request_truncated   BOOLEAN NOT NULL DEFAULT FALSE,
    response_truncated  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversation_turns_conversation_time
    ON conversation_turns(conversation_key, created_at, id);
CREATE INDEX IF NOT EXISTS idx_conversation_turns_group_created
    ON conversation_turns(group_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_turns_user_created
    ON conversation_turns(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_turns_completed
    ON conversation_turns(completed_at);
