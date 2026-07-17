export interface User {
  id: string;
  email: string;
  full_name: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface WorkspaceSummary {
  id: string;
  name: string;
  slug: string;
  kind: string;
  role: string;
}

export interface SessionResponse {
  user: User;
  workspaces: WorkspaceSummary[];
  token?: string;
}

export interface Workspace {
  id: string;
  owner_user_id: string;
  name: string;
  slug: string;
  kind: string;
  created_at: string;
  updated_at: string;
}

export interface Project {
  id: string;
  workspace_id: string;
  name: string;
  slug: string;
  description: string;
  status: string;
  created_by_user_id: string;
  created_at: string;
  updated_at: string;
  sources?: Source[];
}

export interface Source {
  id: string;
  project_id: string;
  source_key: string;
  enabled: boolean;
  config_json: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface Repository {
  id: string;
  project_id: string;
  provider: string;
  owner: string;
  repo_name: string;
  full_name: string;
  default_branch: string;
  webhook_id?: string;
  connected_at: string;
  status: string;
  webhook_url?: string;
}

export interface RepositorySummary {
  owner: string;
  repo_name: string;
  full_name: string;
  default_branch: string;
  private: boolean;
}

export interface KnowledgeItem {
  id: string;
  project_id: string;
  repository_id?: string;
  commit_id?: string;
  pull_request_id?: string;
  title: string;
  summary: string;
  body: string;
  decision_body: string;
  agents_md: string;
  importance: number;
  status: string;
  created_by_user_id?: string;
  approved_by_user_id?: string;
  approved_at?: string;
  created_at: string;
  updated_at: string;
}

export interface Draft {
  id: string;
  project_id: string;
  repository_id?: string;
  commit_id?: string;
  pull_request_id?: string;
  suggested_title: string;
  summary: string;
  source_type: string;
  decision_body: string;
  agents_md: string;
  importance: number;
  status: string;
  reason?: string;
  raw_input_json?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface TimelineEvent {
  id: string;
  workspace_id?: string;
  project_id?: string;
  actor_id?: string;
  event_type: string;
  entity_type: string;
  entity_id: string;
  payload: Record<string, unknown>;
  created_at: string;
}

export interface Member {
  user_id: string;
  email: string;
  full_name: string;
  role: string;
  joined_at: string;
}

export interface SearchResult {
  id: string;
  project_id: string;
  repository_id?: string;
  title: string;
  summary: string;
  importance: number;
  status: string;
  approved_at?: string;
  created_at: string;
}

export interface GitHubStatus {
  configured: boolean;
  connected: boolean;
  github_user_id?: number;
  token_scopes?: string[];
  connected_at?: string;
}

const API_BASE = import.meta.env?.VITE_API_BASE;
const BASE = (API_BASE ? API_BASE : "https://knowell-7pyh.onrender.com").replace(/\/+$/, "");

async function request<T = unknown>(path: string, opts: Omit<RequestInit, 'body'> & { body?: unknown } = {}): Promise<T> {
  const url = BASE ? `${BASE}${path.startsWith("/") ? path : `/${path}`}` : path;
  const { body, ...rest } = opts;
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(rest.headers as Record<string, string> || {}),
  };
  const res = await fetch(url, {
    credentials: "include",
    ...rest,
    headers,
    body: body && typeof body !== "string" ? JSON.stringify(body) : (body as BodyInit | undefined),
  });
  const text = await res.text();
  const data = text ? safeJson(text) : null;
  if (!res.ok) {
    const d = data as Record<string, unknown> | null;
    const err = d?.error;
    const msg = (typeof err === 'object' && err ? (err as Record<string, unknown>).message as string : err as string)
      || d?.message as string
      || res.statusText
      || "Request failed";
    throw new Error(msg);
  }
  return data as T;
}

function safeJson(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

function fmtDate(v: string | undefined | null): string {
  if (!v) return "—";
  try {
    return new Date(v).toLocaleString();
  } catch {
    return v;
  }
}

const EVENT_LABELS: Record<string, string> = {
  repository_connected: "Repository connected",
  repository_disconnected: "Repository disconnected",
  webhook_received: "Push received",
  pull_request_merged: "Pull request merged",
  pull_request_opened: "Pull request opened",
  draft_generated: "AI draft generated",
  knowledge_approved: "Knowledge approved",
  knowledge_edited: "Knowledge edited",
  member_joined: "Team member joined",
  member_invited: "Team member invited",
  draft_approved: "Knowledge approved",
  project_created: "Project created",
};

function humanEvent(event_type: string, payload?: Record<string, unknown>): string {
  const label = EVENT_LABELS[event_type];
  if (label) return label;
  if (payload?.title && typeof payload.title === "string") return payload.title;
  return event_type.replace(/_/g, " ");
}

function getSourceLabel(item: KnowledgeItem): string {
  if (item.repository_id) return "GitHub";
  return "Manual";
}

function getCategoryLabel(item: KnowledgeItem): string {
  if (item.commit_id) return "Commit";
  if (item.pull_request_id) return "Pull Request";
  return "General";
}

export { request, fmtDate, humanEvent, getSourceLabel, getCategoryLabel };
