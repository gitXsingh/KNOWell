import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Plus, Copy, Check, UserCheck, UserX, Users, Key, FolderGit2 } from "lucide-react";
import { request, fmtDate } from "../lib/api";
import type { Workspace, Project, Member } from "../lib/api";
import NotFound from "./NotFound";
import { useToast } from "../lib/toast";

interface JoinRequest {
  id: string;
  user_id: string;
  user_email: string;
  user_full_name: string;
  status: string;
  created_at: string;
}

type PageStatus = "loading" | "loaded" | "not-found";

export default function WorkspaceDetail() {
  const { wid } = useParams();
  const { toast } = useToast();
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [members, setMembers] = useState<Member[]>([]);
  const [joinKey, setJoinKey] = useState("");
  const [copied, setCopied] = useState(false);
  const [joinRequests, setJoinRequests] = useState<JoinRequest[]>([]);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [status, setStatus] = useState<PageStatus>("loading");
  const [showForm, setShowForm] = useState(false);

  function loadAll() {
    if (!wid) return;
    setStatus("loading");
    request<Workspace>(`/workspaces/${wid}`).then((d) => { setWorkspace(d); setStatus("loaded"); }).catch(() => { setWorkspace(null); setStatus("not-found"); });
    request<Project[]>(`/workspaces/${wid}/projects`).then((d) => setProjects(d ?? [])).catch(() => setProjects([]));
    request<Member[]>(`/workspaces/${wid}/members`).then((d) => setMembers(d ?? [])).catch(() => setMembers([]));
    request<{ join_key: string }>(`/workspaces/${wid}/join-key`).then((d) => setJoinKey(d.join_key)).catch(() => setJoinKey(""));
    request<JoinRequest[]>(`/workspaces/${wid}/join-requests`).then((d) => setJoinRequests(d ?? [])).catch(() => setJoinRequests([]));
  }

  useEffect(() => { loadAll(); }, [wid]);

  if (status !== "loaded") {
    if (status === "not-found") return <NotFound />;
    return (
      <div className="page-content">
        <div className="empty-state"><h3>Loading workspace...</h3></div>
      </div>
    );
  }
  if (!workspace) return null;

  async function createProject(e: React.FormEvent) {
    e.preventDefault();
    if (!newName.trim()) return;
    setBusy(true);
    setErr("");
    try {
      await request(`/workspaces/${wid}/projects`, { method: "POST", body: { name: newName, description: newDesc } });
      setNewName(""); setNewDesc("");
      setShowForm(false);
      toast("Project created", "success");
      const data = await request<Project[]>(`/workspaces/${wid}/projects`);
      setProjects(data ?? []);
    } catch (e) {
      const msg = (e as Error).message || "Failed";
      setErr(msg); toast(msg, "error");
    } finally { setBusy(false); }
  }

  const pending = joinRequests.filter((r) => r.status === "pending");

  return (
    <div className="page-content">
      <div className="page-content__header">
        <h1>{workspace.name}</h1>
        <p>{workspace.kind} workspace</p>
      </div>

      <div className="page-bar">
        <h2>Projects</h2>
        <button className="btn btn--primary btn--sm" onClick={() => setShowForm(!showForm)}>
          <Plus size={13} /> New Project
        </button>
      </div>

      {showForm && (
        <form onSubmit={createProject} className="row gap-2" style={{ marginBottom: "var(--space-4)" }}>
          <input className="input" placeholder="Project name" value={newName} onChange={(e) => setNewName(e.target.value)} required style={{ flex: 1 }} aria-label="Project name" />
          <input className="input" placeholder="Description" value={newDesc} onChange={(e) => setNewDesc(e.target.value)} style={{ flex: 1 }} aria-label="Project description" />
          <button type="submit" className="btn btn--primary btn--sm" disabled={busy || !newName.trim()}>Create</button>
        </form>
      )}
      {err && <p className="text-sm" style={{ color: "var(--red-500)", marginBottom: "var(--space-2)" }}>{err}</p>}

      {projects.length === 0 ? (
        <div className="empty-state" style={{ padding: "var(--space-8)" }}>
          <div className="empty-state__icon"><FolderGit2 size={18} /></div>
          <h3>No projects yet</h3>
          <p>Create one to connect a repository.</p>
        </div>
      ) : (
        <div className="grid-2" style={{ marginBottom: "var(--space-8)" }}>
          {projects.map((p) => (
            <Link key={p.id} to={`/workspaces/${wid}/projects/${p.id}`} className="card" style={{ textDecoration: "none" }}>
              <div className="proj-card__title">{p.name}</div>
              <div className="proj-card__desc" style={{ marginTop: 2 }}>{p.description || "No description"}</div>
              <div className="text-dim text-xs" style={{ marginTop: "var(--space-2)" }}>Updated {fmtDate(p.updated_at)}</div>
            </Link>
          ))}
        </div>
      )}

      <div className="grid-2" style={{ gridTemplateColumns: "1fr 1fr", gap: "var(--space-4)" }}>
        {workspace.kind === "team" && joinKey && (
          <div className="card card--compact">
            <div className="row gap-2" style={{ marginBottom: "var(--space-2)" }}>
              <Key size={14} />
              <h3>Invite Key</h3>
            </div>
            <div className="row gap-2" style={{ padding: "8px 10px", background: "var(--gray-50)", borderRadius: "var(--radius)" }}>
              <code className="text-mono" style={{ flex: 1, fontSize: 12, letterSpacing: "0.06em", fontWeight: 600 }}>{joinKey}</code>
              <button className="btn btn--ghost btn--sm" onClick={() => { navigator.clipboard.writeText(joinKey); setCopied(true); toast("Copied", "success"); setTimeout(() => setCopied(false), 2000); }}>
                {copied ? <Check size={13} /> : <Copy size={13} />}
              </button>
            </div>
            <div className="text-dim text-xs" style={{ marginTop: "var(--space-1)" }}>Share this key with teammates to let them join.</div>
          </div>
        )}

        <div className="card card--compact">
          <div className="row gap-2" style={{ marginBottom: "var(--space-2)" }}>
            <Users size={14} />
            <h3>Members ({members.length})</h3>
          </div>
          <div className="col">
            {members.map((m) => (
              <div key={m.user_id} className="row gap-2" style={{ padding: "5px 0" }}>
                <div className="avatar avatar--sm">{m.full_name.split(" ").map(s => s[0]).join("").slice(0, 2)}</div>
                <div className="flex-1" style={{ minWidth: 0 }}>
                  <span className="text-strong text-sm">{m.full_name}</span>
                  <span className="pill pill--default" style={{ marginLeft: 4, fontSize: 10, height: 18 }}>{m.role}</span>
                </div>
              </div>
            ))}
            {members.length === 0 && <div className="text-dim text-sm">No members</div>}
          </div>
        </div>

        {workspace.kind === "team" && (
          <div className="card card--compact">
            <div className="row gap-2" style={{ marginBottom: "var(--space-2)" }}>
              <UserCheck size={14} />
              <h3>Join Requests</h3>
              {pending.length > 0 && <span className="pill pill--warning" style={{ height: 18, fontSize: 10 }}>{pending.length}</span>}
            </div>
            {pending.length === 0 ? (
              <div className="text-dim text-sm">No pending requests</div>
            ) : (
              <div className="col">
                {pending.map((r) => (
                  <div key={r.id} className="row gap-2" style={{ padding: "5px 0" }}>
                    <div className="avatar avatar--sm">{r.user_full_name.split(" ").map((s: string) => s[0]).join("").slice(0, 2)}</div>
                    <div className="flex-1" style={{ minWidth: 0 }}>
                      <div className="text-strong text-sm">{r.user_full_name}</div>
                      <div className="text-dim text-xs">{r.user_email}</div>
                    </div>
                    <div className="row gap-1">
                      <button className="btn btn--ghost btn--sm" style={{ color: "var(--green-500)" }} onClick={async () => { await request(`/workspaces/${wid}/join-requests/approve`, { method: "POST", body: { user_id: r.user_id, action: "approve" } }); toast(`${r.user_full_name} approved`, "success"); loadAll(); }}>
                        <UserCheck size={13} />
                      </button>
                      <button className="btn btn--ghost btn--sm" style={{ color: "var(--red-500)" }} onClick={async () => { await request(`/workspaces/${wid}/join-requests/approve`, { method: "POST", body: { user_id: r.user_id, action: "reject" } }); toast(`Rejected ${r.user_full_name}`, "info"); loadAll(); }}>
                        <UserX size={13} />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
