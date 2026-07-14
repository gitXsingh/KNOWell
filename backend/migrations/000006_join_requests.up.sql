CREATE TABLE join_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now(),
  reviewed_at timestamptz,
  reviewed_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  UNIQUE (workspace_id, user_id)
);

CREATE INDEX join_requests_workspace_id_idx ON join_requests (workspace_id);
CREATE INDEX join_requests_user_id_idx ON join_requests (user_id);
CREATE INDEX join_requests_status_idx ON join_requests (status);
