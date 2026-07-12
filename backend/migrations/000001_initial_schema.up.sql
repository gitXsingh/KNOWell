CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE user_status AS ENUM ('active', 'invited', 'disabled');
CREATE TYPE workspace_kind AS ENUM ('personal', 'team');
CREATE TYPE workspace_member_role AS ENUM ('owner', 'member');
CREATE TYPE invitation_scope_type AS ENUM ('workspace', 'project');
CREATE TYPE invitation_status AS ENUM ('pending', 'accepted', 'revoked', 'expired');
CREATE TYPE project_status AS ENUM ('active', 'archived', 'completed');
CREATE TYPE project_member_role AS ENUM ('owner', 'tech_lead', 'developer', 'viewer');
CREATE TYPE repository_provider AS ENUM ('github');
CREATE TYPE repository_status AS ENUM ('connected', 'disconnected');
CREATE TYPE pull_request_state AS ENUM ('opened', 'closed', 'merged');
CREATE TYPE ai_draft_source_type AS ENUM (
  'commit',
  'pull_request',
  'manual_note',
  'api_documentation',
  'architecture_decision',
  'meeting_note',
  'deployment_note',
  'database_schema'
);
CREATE TYPE ai_draft_status AS ENUM ('draft', 'in_review', 'approved', 'rejected', 'archived');
CREATE TYPE knowledge_item_status AS ENUM ('active', 'archived');
CREATE TYPE comment_target_type AS ENUM ('draft', 'knowledge_item');
CREATE TYPE webhook_processing_status AS ENUM ('received', 'processing', 'processed', 'failed', 'ignored');
CREATE TYPE activity_event_type AS ENUM (
  'commit',
  'pr_created',
  'pr_merged',
  'draft_generated',
  'draft_approved',
  'knowledge_edited',
  'member_joined',
  'repository_connected'
);

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL,
  password_hash text NOT NULL,
  full_name text NOT NULL,
  status user_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email));

CREATE TABLE workspaces (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  slug text NOT NULL,
  kind workspace_kind NOT NULL DEFAULT 'personal',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (slug)
);

CREATE TABLE workspace_members (
  workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role workspace_member_role NOT NULL,
  joined_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, user_id)
);

CREATE INDEX workspace_members_workspace_id_idx ON workspace_members (workspace_id);
CREATE INDEX workspace_members_user_id_idx ON workspace_members (user_id);

CREATE TABLE invitations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scope_type invitation_scope_type NOT NULL,
  scope_id uuid NOT NULL,
  invited_email text NOT NULL,
  role text NOT NULL,
  token_hash text NOT NULL UNIQUE,
  status invitation_status NOT NULL DEFAULT 'pending',
  invited_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  accepted_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX invitations_invited_email_idx ON invitations (invited_email);
CREATE INDEX invitations_scope_idx ON invitations (scope_type, scope_id);
CREATE INDEX invitations_status_idx ON invitations (status);

CREATE TABLE projects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  name text NOT NULL,
  slug text NOT NULL,
  description text NOT NULL DEFAULT '',
  status project_status NOT NULL DEFAULT 'active',
  created_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, slug)
);

CREATE INDEX projects_workspace_id_idx ON projects (workspace_id);
CREATE INDEX projects_status_idx ON projects (status);

CREATE TABLE project_members (
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role project_member_role NOT NULL,
  joined_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (project_id, user_id)
);

CREATE INDEX project_members_project_id_idx ON project_members (project_id);
CREATE INDEX project_members_user_id_idx ON project_members (user_id);

CREATE TABLE project_sources (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  source_key text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  config_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, source_key)
);

CREATE INDEX project_sources_project_id_idx ON project_sources (project_id);
CREATE INDEX project_sources_enabled_idx ON project_sources (enabled);

CREATE TABLE github_accounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  github_user_id bigint NOT NULL UNIQUE,
  access_token_encrypted text NOT NULL,
  token_scopes text NOT NULL DEFAULT '',
  connected_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz
);

CREATE TABLE repositories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
  provider repository_provider NOT NULL,
  owner text NOT NULL,
  repo_name text NOT NULL,
  default_branch text NOT NULL DEFAULT 'main',
  webhook_id text,
  connected_at timestamptz NOT NULL DEFAULT now(),
  status repository_status NOT NULL DEFAULT 'connected',
  UNIQUE (provider, owner, repo_name)
);

CREATE INDEX repositories_project_id_idx ON repositories (project_id);
CREATE INDEX repositories_provider_owner_repo_idx ON repositories (provider, owner, repo_name);

CREATE TABLE categories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  sort_order integer NOT NULL DEFAULT 0,
  active boolean NOT NULL DEFAULT true
);

CREATE INDEX categories_active_sort_idx ON categories (active, sort_order);

CREATE TABLE tags (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  name text NOT NULL
);

CREATE TABLE commits (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  sha text NOT NULL,
  message text NOT NULL,
  author_name text NOT NULL,
  author_email text NOT NULL,
  committed_at timestamptz NOT NULL,
  diff_summary_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  raw_payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE (repository_id, sha)
);

CREATE INDEX commits_project_id_idx ON commits (project_id);
CREATE INDEX commits_repository_id_idx ON commits (repository_id);
CREATE INDEX commits_committed_at_idx ON commits (committed_at DESC);

CREATE TABLE pull_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  number integer NOT NULL,
  title text NOT NULL,
  description text NOT NULL DEFAULT '',
  state pull_request_state NOT NULL,
  merged_at timestamptz,
  head_sha text NOT NULL DEFAULT '',
  base_branch text NOT NULL DEFAULT 'main',
  merged_by_name text NOT NULL DEFAULT '',
  raw_payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE (repository_id, number)
);

CREATE INDEX pull_requests_project_id_idx ON pull_requests (project_id);
CREATE INDEX pull_requests_repository_id_idx ON pull_requests (repository_id);
CREATE INDEX pull_requests_state_idx ON pull_requests (state);

CREATE TABLE webhook_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  github_delivery_id text NOT NULL,
  event_type text NOT NULL,
  action text NOT NULL DEFAULT '',
  signature_valid boolean NOT NULL DEFAULT false,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  received_at timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz,
  processing_status webhook_processing_status NOT NULL DEFAULT 'received',
  error_message text NOT NULL DEFAULT '',
  UNIQUE (repository_id, github_delivery_id)
);

CREATE INDEX webhook_events_repository_id_idx ON webhook_events (repository_id);
CREATE INDEX webhook_events_event_type_idx ON webhook_events (event_type);
CREATE INDEX webhook_events_processed_at_idx ON webhook_events (processed_at DESC);

CREATE TABLE ai_drafts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  repository_id uuid REFERENCES repositories(id) ON DELETE SET NULL,
  commit_id uuid REFERENCES commits(id) ON DELETE SET NULL,
  pull_request_id uuid REFERENCES pull_requests(id) ON DELETE SET NULL,
  source_type ai_draft_source_type NOT NULL,
  status ai_draft_status NOT NULL DEFAULT 'draft',
  suggested_title text NOT NULL,
  summary text NOT NULL,
  category_id uuid REFERENCES categories(id) ON DELETE SET NULL,
  importance integer NOT NULL DEFAULT 0,
  reason text NOT NULL DEFAULT '',
  raw_input_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  ai_provider text NOT NULL DEFAULT 'ollama',
  version integer NOT NULL DEFAULT 1,
  created_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  reviewed_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  reviewed_at timestamptz
);

CREATE INDEX ai_drafts_project_id_idx ON ai_drafts (project_id);
CREATE INDEX ai_drafts_status_idx ON ai_drafts (status);
CREATE INDEX ai_drafts_category_id_idx ON ai_drafts (category_id);
CREATE INDEX ai_drafts_repository_id_idx ON ai_drafts (repository_id);

CREATE TABLE knowledge_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  repository_id uuid REFERENCES repositories(id) ON DELETE SET NULL,
  commit_id uuid REFERENCES commits(id) ON DELETE SET NULL,
  pull_request_id uuid REFERENCES pull_requests(id) ON DELETE SET NULL,
  author_id uuid REFERENCES users(id) ON DELETE SET NULL,
  category_id uuid REFERENCES categories(id) ON DELETE SET NULL,
  title text NOT NULL,
  summary text NOT NULL,
  body text NOT NULL DEFAULT '',
  importance integer NOT NULL DEFAULT 0,
  status knowledge_item_status NOT NULL DEFAULT 'active',
  created_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  approved_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  approved_at timestamptz,
  archived_at timestamptz,
  search_vector tsvector NOT NULL DEFAULT ''::tsvector,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX knowledge_items_project_id_idx ON knowledge_items (project_id);
CREATE INDEX knowledge_items_category_id_idx ON knowledge_items (category_id);
CREATE INDEX knowledge_items_repository_id_idx ON knowledge_items (repository_id);
CREATE INDEX knowledge_items_author_id_idx ON knowledge_items (author_id);
CREATE INDEX knowledge_items_created_at_idx ON knowledge_items (created_at DESC);
CREATE INDEX knowledge_items_search_vector_idx ON knowledge_items USING GIN (search_vector);

CREATE TABLE knowledge_item_tags (
  knowledge_item_id uuid NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
  tag_id uuid NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (knowledge_item_id, tag_id)
);

CREATE INDEX knowledge_item_tags_tag_id_idx ON knowledge_item_tags (tag_id);

CREATE TABLE comments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  target_type comment_target_type NOT NULL,
  target_id uuid NOT NULL,
  author_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  body text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX comments_target_idx ON comments (target_type, target_id);
CREATE INDEX comments_author_id_idx ON comments (author_id);

CREATE TABLE activity_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid REFERENCES workspaces(id) ON DELETE CASCADE,
  project_id uuid REFERENCES projects(id) ON DELETE CASCADE,
  actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
  event_type activity_event_type NOT NULL,
  entity_type text NOT NULL,
  entity_id uuid NOT NULL,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  dedupe_key text NOT NULL UNIQUE
);

CREATE INDEX activity_logs_workspace_id_idx ON activity_logs (workspace_id);
CREATE INDEX activity_logs_project_id_idx ON activity_logs (project_id);
CREATE INDEX activity_logs_event_type_idx ON activity_logs (event_type);
CREATE INDEX activity_logs_created_at_idx ON activity_logs (created_at DESC);