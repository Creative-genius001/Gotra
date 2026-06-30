-- Core platform schema (Backend Architecture Bible): workspaces/teams,
-- projects & membership (RBAC), tunnels & domains, captured requests/responses,
-- replays, API keys, subscriptions, audit logs and notifications.

CREATE TABLE workspaces (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       VARCHAR(255) NOT NULL,
    slug       VARCHAR(255) NOT NULL,
    is_personal BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_workspaces_slug ON workspaces (slug);
CREATE INDEX idx_workspaces_owner ON workspaces (owner_id);

CREATE TABLE projects (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    owner_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    slug         VARCHAR(255) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_projects_slug ON projects (slug);
CREATE INDEX idx_projects_workspace ON projects (workspace_id);
CREATE INDEX idx_projects_owner ON projects (owner_id);

-- RBAC roles per the bibles: owner | admin | developer | viewer.
CREATE TABLE project_members (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       VARCHAR(50) NOT NULL DEFAULT 'developer',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_project_member UNIQUE (project_id, user_id)
);
CREATE INDEX idx_project_members_user ON project_members (user_id);

CREATE TABLE api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL DEFAULT 'default',
    key_hash    TEXT NOT NULL,
    last_used_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_api_keys_hash ON api_keys (key_hash);
CREATE INDEX idx_api_keys_project ON api_keys (project_id);

CREATE TABLE tunnels (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    public_url TEXT NOT NULL,
    local_port INT NOT NULL,
    status     VARCHAR(50) NOT NULL DEFAULT 'created',  -- created|connecting|active|disconnected|expired|deleted
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tunnels_project ON tunnels (project_id);
CREATE INDEX idx_tunnels_status ON tunnels (status);
CREATE UNIQUE INDEX idx_tunnels_public_url ON tunnels (public_url);

CREATE TABLE tunnel_domains (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tunnel_id  UUID NOT NULL REFERENCES tunnels(id) ON DELETE CASCADE,
    domain     TEXT NOT NULL,
    is_custom  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_tunnel_domains_domain ON tunnel_domains (domain);

-- Captured traffic. (High-volume analytics will move to ClickHouse later;
-- Postgres holds the canonical capture for the first pass.)
CREATE TABLE requests (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tunnel_id   UUID NOT NULL REFERENCES tunnels(id) ON DELETE CASCADE,
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    method      VARCHAR(10) NOT NULL,
    path        TEXT NOT NULL,
    query       TEXT,
    headers     JSONB NOT NULL DEFAULT '{}',
    body        BYTEA,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_requests_tunnel ON requests (tunnel_id);
CREATE INDEX idx_requests_project ON requests (project_id);
CREATE INDEX idx_requests_received_at ON requests (received_at);

CREATE TABLE responses (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id   UUID NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    status_code  INT NOT NULL,
    headers      JSONB NOT NULL DEFAULT '{}',
    body         BYTEA,
    duration_ms  INT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_responses_request ON responses (request_id);

CREATE TABLE replays (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_request_id UUID NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    project_id          UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    modified_request    JSONB NOT NULL DEFAULT '{}',
    result_status_code  INT,
    result_body         BYTEA,
    duration_ms         INT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_replays_original ON replays (original_request_id);
CREATE INDEX idx_replays_project ON replays (project_id);

CREATE TABLE subscriptions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    plan         VARCHAR(50) NOT NULL DEFAULT 'free',
    status       VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_subscriptions_workspace ON subscriptions (workspace_id);

CREATE TABLE audit_logs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE SET NULL,
    user_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    action       VARCHAR(100) NOT NULL,
    target       TEXT,
    metadata     JSONB NOT NULL DEFAULT '{}',
    ip_address   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_workspace ON audit_logs (workspace_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at);

CREATE TABLE notifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       VARCHAR(100) NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT,
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notifications_user ON notifications (user_id);
