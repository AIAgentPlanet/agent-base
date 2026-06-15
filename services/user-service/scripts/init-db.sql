-- user_service database initialization script
-- Create users table with indexes

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    deleted_at BIGINT DEFAULT 0,
    username VARCHAR(64) NOT NULL UNIQUE,
    password TEXT NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    phone VARCHAR(32) NOT NULL UNIQUE,
    nickname VARCHAR(128) NOT NULL DEFAULT '',
    avatar TEXT NOT NULL DEFAULT '',
    status INT NOT NULL DEFAULT 1,
    login_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

CREATE TABLE IF NOT EXISTS oauth_clients (
    id BIGSERIAL PRIMARY KEY,
    client_id VARCHAR(128) NOT NULL UNIQUE,
    client_secret VARCHAR(256) NOT NULL,
    name VARCHAR(256) NOT NULL,
    redirect_uris TEXT NOT NULL,
    allowed_grants TEXT NOT NULL DEFAULT '["authorization_code"]',
    allowed_scopes TEXT NOT NULL DEFAULT '["profile"]',
    user_id BIGINT NOT NULL DEFAULT 0,
    status INT NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_oauth_clients_user_id ON oauth_clients(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_clients_status ON oauth_clients(status);

CREATE TABLE IF NOT EXISTS ath_audit_records (
    id BIGSERIAL PRIMARY KEY,
    sequence BIGINT NOT NULL UNIQUE,
    event_id VARCHAR(64) NOT NULL UNIQUE,
    event_type VARCHAR(64) NOT NULL,
    client_id VARCHAR(128),
    agent_id VARCHAR(256),
    handshake_id VARCHAR(128),
    token_id VARCHAR(128),
    payload TEXT NOT NULL,
    payload_hash VARCHAR(64) NOT NULL,
    previous_hash VARCHAR(64) NOT NULL,
    record_hash VARCHAR(64) NOT NULL UNIQUE,
    signature TEXT NOT NULL,
    signing_key_id VARCHAR(256) NOT NULL,
    signing_public_key TEXT NOT NULL,
    created_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ath_audit_event_type ON ath_audit_records(event_type);
CREATE INDEX IF NOT EXISTS idx_ath_audit_client_id ON ath_audit_records(client_id);
CREATE INDEX IF NOT EXISTS idx_ath_audit_agent_id ON ath_audit_records(agent_id);
CREATE INDEX IF NOT EXISTS idx_ath_audit_handshake_id ON ath_audit_records(handshake_id);
CREATE INDEX IF NOT EXISTS idx_ath_audit_token_id ON ath_audit_records(token_id);
CREATE INDEX IF NOT EXISTS idx_ath_audit_created_at ON ath_audit_records(created_at);

CREATE OR REPLACE FUNCTION prevent_ath_audit_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'ath_audit_records is append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS ath_audit_records_immutable ON ath_audit_records;
CREATE TRIGGER ath_audit_records_immutable
BEFORE UPDATE OR DELETE ON ath_audit_records
FOR EACH ROW EXECUTE FUNCTION prevent_ath_audit_mutation();

CREATE TABLE IF NOT EXISTS ath_audit_outboxes (
    id BIGSERIAL PRIMARY KEY,
    audit_record_id BIGINT NOT NULL UNIQUE,
    event_id VARCHAR(64) NOT NULL UNIQUE,
    sequence BIGINT NOT NULL,
    record_hash VARCHAR(64) NOT NULL,
    payload TEXT NOT NULL,
    status VARCHAR(16) NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at BIGINT NOT NULL,
    locked_until BIGINT NOT NULL DEFAULT 0,
    last_error TEXT,
    delivered_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ath_outbox_sequence ON ath_audit_outboxes(sequence);
CREATE INDEX IF NOT EXISTS idx_ath_outbox_status ON ath_audit_outboxes(status);
CREATE INDEX IF NOT EXISTS idx_ath_outbox_next_attempt ON ath_audit_outboxes(next_attempt_at);
