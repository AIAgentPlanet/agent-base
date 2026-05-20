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
