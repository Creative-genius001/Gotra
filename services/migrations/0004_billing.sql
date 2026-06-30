-- Billing: ensure one subscription per workspace so plan upserts can use
-- ON CONFLICT, and seed a free subscription for every existing workspace.

CREATE UNIQUE INDEX IF NOT EXISTS uq_subscriptions_workspace ON subscriptions (workspace_id);

INSERT INTO subscriptions (workspace_id, plan, status)
SELECT w.id, 'free', 'active'
FROM workspaces w
LEFT JOIN subscriptions s ON s.workspace_id = w.id
WHERE s.id IS NULL;
