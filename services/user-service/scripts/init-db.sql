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
