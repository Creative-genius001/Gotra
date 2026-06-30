-- Auth & identity schema (Authentication & Identity Architecture Bible).
-- Identity (users) is kept separate from login methods (user_auth_providers);
-- a single user may link password, Google and GitHub providers.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email          VARCHAR(255) NOT NULL,
    name           VARCHAR(255) NOT NULL DEFAULT '',
    avatar_url     TEXT,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    last_login_at  TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_users_email ON users (lower(email));
CREATE INDEX idx_users_created_at ON users (created_at);

CREATE TABLE user_auth_providers (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          VARCHAR(50) NOT NULL,            -- password | google | github
    provider_user_id  VARCHAR(255) NOT NULL,
    provider_email    VARCHAR(255),
    password_hash     TEXT,                            -- only for provider = 'password' (Argon2id)
    provider_metadata JSONB NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_provider_identity UNIQUE (provider, provider_user_id)
);
CREATE INDEX idx_auth_providers_user_id ON user_auth_providers (user_id);

CREATE TABLE sessions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL,
    ip_address         TEXT,
    user_agent         TEXT,
    expires_at         TIMESTAMPTZ NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at         TIMESTAMPTZ
);
CREATE INDEX idx_sessions_user_id ON sessions (user_id);
CREATE UNIQUE INDEX idx_sessions_refresh_hash ON sessions (refresh_token_hash);

CREATE TABLE email_verifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ
);
CREATE INDEX idx_email_verifications_user_id ON email_verifications (user_id);

CREATE TABLE password_resets (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ
);
CREATE INDEX idx_password_resets_user_id ON password_resets (user_id);
