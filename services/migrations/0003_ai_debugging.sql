-- AI Debugging Service schema (AI Debugging Service Architecture Bible).
-- Stores analyses, incidents, user feedback, token/cost usage, raw provider
-- logs and versioned prompts. Every analysis carries a 0–100 confidence score.

CREATE TABLE ai_analyses (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    request_id       UUID REFERENCES requests(id) ON DELETE SET NULL,
    analysis_type    VARCHAR(50) NOT NULL,   -- explain_error|explain_logs|analyze_request|analyze_replay
    provider         VARCHAR(50) NOT NULL,   -- gemini|claude
    confidence_score INT NOT NULL DEFAULT 0 CHECK (confidence_score BETWEEN 0 AND 100),
    severity         VARCHAR(50),
    result_json      JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_analyses_project ON ai_analyses (project_id);
CREATE INDEX idx_ai_analyses_request ON ai_analyses (request_id);
CREATE INDEX idx_ai_analyses_created_at ON ai_analyses (created_at);

CREATE TABLE ai_incidents (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    summary          TEXT NOT NULL,
    root_cause       TEXT,
    confidence_score INT NOT NULL DEFAULT 0 CHECK (confidence_score BETWEEN 0 AND 100),
    status           VARCHAR(50) NOT NULL DEFAULT 'detected', -- detected|investigating|confirmed|resolved|archived
    report_json      JSONB NOT NULL DEFAULT '{}',
    assigned_to      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_incidents_project ON ai_incidents (project_id);
CREATE INDEX idx_ai_incidents_status ON ai_incidents (status);

CREATE TABLE ai_feedback (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_id UUID NOT NULL REFERENCES ai_analyses(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating      INT CHECK (rating BETWEEN 1 AND 5),
    feedback    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_feedback_analysis ON ai_feedback (analysis_id);

CREATE TABLE ai_usage (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider   VARCHAR(50) NOT NULL,
    tokens_in  INT NOT NULL DEFAULT 0,
    tokens_out INT NOT NULL DEFAULT 0,
    cost_usd   NUMERIC(12, 6) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_usage_project ON ai_usage (project_id);
CREATE INDEX idx_ai_usage_created_at ON ai_usage (created_at);

CREATE TABLE ai_provider_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_id   UUID REFERENCES ai_analyses(id) ON DELETE SET NULL,
    provider      VARCHAR(50) NOT NULL,
    request_json  JSONB,
    response_json JSONB,
    latency_ms    INT,
    error         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_provider_logs_analysis ON ai_provider_logs (analysis_id);

CREATE TABLE ai_prompt_versions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    version    INT NOT NULL,
    template   TEXT NOT NULL,
    is_active  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_prompt_name_version UNIQUE (name, version)
);
