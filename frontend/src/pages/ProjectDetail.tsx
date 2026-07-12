import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Check, X, ExternalLink, RefreshCw } from "lucide-react";
import { request, fmtDate } from "../lib/api";
import type { Project, Repository, KnowledgeItem, Draft, TimelineEvent, Member } from "../lib/api";
import NotFound from "./NotFound";
import { useToast } from "../lib/toast";

const TABS = ["Overview", "Repository", "Knowledge", "Drafts", "Timeline", "Members"];

export default function ProjectDetail() {
  const { wid, pid, tab = "Overview" } = useParams();
  const nav = useNavigate();
  const [project, setProject] = useState<Project | null>(null);

  useEffect(() => {
    if (!wid || !pid) return;
    request<Project>(`/workspaces/${wid}/projects/${pid}`).then(setProject).catch(() => setProject(null));
  }, [wid, pid]);

  if (!project) return <NotFound />;

  return (
    <div className="page-content">
      <div className="page-content__header">
        <h1>{project.name}</h1>
        <p>{project.description || "No description"}</p>
      </div>
      <div className="tabs">
        {TABS.map((t) => (
          <button key={t} className={`tab ${tab === t ? "active" : ""}`} onClick={() => nav(`/workspaces/${wid}/projects/${pid}/${t}`)}>{t}</button>
        ))}
      </div>
      {tab === "Overview" && <OverviewTab project={project} />}
      {tab === "Repository" && <RepositoryTab wid={wid!} pid={pid!} />}
      {tab === "Knowledge" && <KnowledgeTab wid={wid!} pid={pid!} />}
      {tab === "Drafts" && <DraftsTab wid={wid!} pid={pid!} />}
      {tab === "Timeline" && <TimelineTab wid={wid!} pid={pid!} />}
      {tab === "Members" && <MembersTab wid={wid!} pid={pid!} />}
    </div>
  );
}

function OverviewTab({ project }: { project: Project }) {
  return (
    <div className="grid-2">
      <div className="card">
        <h3 style={{ marginBottom: "var(--space-3)" }}>Details</h3>
        <div className="col gap-2">
          {[
            ["Status", <span className="pill pill--success">{project.status}</span>],
            ["Created", fmtDate(project.created_at)],
            ["Updated", fmtDate(project.updated_at)],
          ].map(([l, v]) => (
            <div key={l as string} className="row justify-between">
              <span className="text-sm text-muted">{l as string}</span>
              <span className="text-sm text-strong">{v as React.ReactNode}</span>
            </div>
          ))}
        </div>
      </div>
      <div className="card">
        <h3 style={{ marginBottom: "var(--space-3)" }}>Sources</h3>
        <div className="col gap-2">
          {project.sources?.length ? project.sources.map((s) => (
            <div key={s.source_key} className="row justify-between">
              <span className="text-sm text-muted">{s.source_key.replace(/_/g, " ")}</span>
              {s.enabled ? <span className="pill pill--success">Enabled</span> : <span className="pill pill--default">Disabled</span>}
            </div>
          )) : <span className="text-dim text-sm">None configured</span>}
        </div>
      </div>
    </div>
  );
}

function RepositoryTab({ wid, pid }: { wid: string; pid: string }) {
  const { toast } = useToast();
  const [repo, setRepo] = useState<Repository | null>(null);
  const [loading, setLoading] = useState(true);
  const [owner, setOwner] = useState("");
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const base = `/workspaces/${wid}/projects/${pid}/repository`;

  async function load() {
    setLoading(true);
    try { setRepo(await request<Repository>(base)); }
    catch { setRepo(null); }
    finally { setLoading(false); }
  }

  useEffect(() => { load(); }, [pid]);

  async function connect(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true); setErr("");
    try {
      await request(base, { method: "PUT", body: { owner, repo_name: name } });
      setOwner(""); setName("");
      toast("Repository connected", "success");
      await load();
    } catch (e) { const msg = (e as Error).message; setErr(msg); toast(msg, "error"); }
    finally { setBusy(false); }
  }

  if (loading) return <div className="empty-state" style={{ marginTop: "var(--space-5)" }}><h3>Loading...</h3></div>;

  if (!repo) return (
    <div className="card" style={{ maxWidth: 520 }}>
      <h3 style={{ marginBottom: "var(--space-2)" }}>Connect a GitHub repository</h3>
      <p className="text-muted text-sm" style={{ marginBottom: "var(--space-4)" }}>Requires a linked GitHub account.</p>
      <form onSubmit={connect} className="row gap-2">
        <input className="input" placeholder="owner" value={owner} onChange={(e) => setOwner(e.target.value)} style={{ width: 120 }} />
        <span className="text-dim">/</span>
        <input className="input" placeholder="repo-name" value={name} onChange={(e) => setName(e.target.value)} style={{ flex: 1 }} />
        <button type="submit" className="btn btn--primary btn--sm" disabled={busy}>Connect</button>
      </form>
      {err && <p className="text-sm" style={{ color: "var(--red-500)", marginTop: "var(--space-2)" }}>{err}</p>}
    </div>
  );

  return (
    <div className="card" style={{ maxWidth: 520 }}>
      <div className="row justify-between" style={{ marginBottom: "var(--space-3)" }}>
        <div>
          <div className="text-strong">{repo.full_name || `${repo.owner}/${repo.repo_name}`}</div>
          <div className="text-dim text-sm">Branch: {repo.default_branch || "main"}</div>
        </div>
        <div className="row gap-1">
          <button className="btn btn--ghost btn--sm" onClick={async () => { setBusy(true); try { await request(`${base}/webhook/sync`, { method: "POST" }); toast("Synced", "success"); } catch (e) { toast((e as Error).message, "error"); } finally { setBusy(false); } }} disabled={busy}>
            <RefreshCw size={12} /> Sync
          </button>
          <button className="btn btn--ghost btn--sm" style={{ color: "var(--red-500)" }} onClick={async () => { await request(base, { method: "DELETE" }); toast("Disconnected", "info"); await load(); }}>Disconnect</button>
        </div>
      </div>
      <span className="pill pill--success">{repo.status}</span>
    </div>
  );
}

function KnowledgeTab({ wid, pid }: { wid: string; pid: string }) {
  const [items, setItems] = useState<KnowledgeItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [view, setView] = useState<"team" | "agent">("team");
  const [expanded, setExpanded] = useState<string | null>(null);
  useEffect(() => { setLoading(true); request<KnowledgeItem[]>(`/workspaces/${wid}/projects/${pid}/knowledge-items`).then((d) => setItems(d ?? [])).finally(() => setLoading(false)); }, [pid]);
  if (loading) return <div className="empty-state" style={{ marginTop: "var(--space-5)" }}><h3>Loading...</h3></div>;
  if (items.length === 0) return <div className="empty-state"><div className="empty-state__icon">·</div><h3>No knowledge yet</h3><p>Approve drafts to promote them here.</p></div>;

  const downloadMd = (content: string, filename: string) => {
    const blob = new Blob([content], { type: "text/markdown" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url; a.download = filename; a.click();
    URL.revokeObjectURL(url);
  };

  return <div className="col gap-3">
    <div className="row gap-1">
      <button className={`btn btn--sm ${view === "team" ? "btn--primary" : "btn--ghost"}`} onClick={() => setView("team")}>Team View</button>
      <button className={`btn btn--sm ${view === "agent" ? "btn--primary" : "btn--ghost"}`} onClick={() => setView("agent")}>Agent View</button>
    </div>
    {view === "team" ? (
      <div className="grid-2">{items.map((k) => (
        <div key={k.id} className="card">
          <div className="row justify-between">
            <div className="proj-card__title" style={{ cursor: "pointer", flex: 1 }} onClick={() => setExpanded(expanded === k.id ? null : k.id)}>{k.title}</div>
            <button className="btn btn--ghost btn--sm" onClick={() => downloadMd(k.decision_body || `# ${k.title}\n\n${k.summary}`, `${k.title.replace(/[^a-z0-9]/gi, "_").toLowerCase()}_decision.md`)}>Download .md</button>
          </div>
          <div className="proj-card__desc" style={{ marginTop: "var(--space-1)", cursor: "pointer" }} onClick={() => setExpanded(expanded === k.id ? null : k.id)}>{k.summary}</div>
          <div className="row gap-2" style={{ marginTop: "var(--space-2)" }}>
            <span className="pill pill--default">Importance: {k.importance}</span>
            <span className="pill pill--default">{fmtDate(k.created_at)}</span>
          </div>
          {expanded === k.id && k.decision_body && (
            <div className="markdown-content" style={{ marginTop: "var(--space-3)", paddingTop: "var(--space-2)", borderTop: "1px solid var(--gray-200)" }}>
              {k.decision_body.split("\n").map((line, i) => <p key={i} className="text-sm" style={{ margin: "0.25em 0", lineHeight: 1.6 }}>{line}</p>)}
            </div>
          )}
        </div>
      ))}</div>
    ) : (
      <div className="col gap-2">{items.map((k) => (
        <div key={k.id} className="card">
          <div className="row justify-between" style={{ marginBottom: "var(--space-1)" }}>
            <div className="proj-card__title" style={{ fontSize: 14 }}>{k.title}</div>
            <button className="btn btn--ghost btn--sm" onClick={() => downloadMd(k.agents_md || `# ${k.title}\n\n${k.summary}`, `${k.title.replace(/[^a-z0-9]/gi, "_").toLowerCase()}_agents.md`)}>Download .md</button>
          </div>
          <pre className="text-sm" style={{ background: "var(--gray-50)", padding: 12, borderRadius: "var(--radius)", overflow: "auto", fontSize: 12, lineHeight: 1.5, whiteSpace: "pre-wrap" }}>{(k.agents_md || `# ${k.title}\n\n**Summary:** ${k.summary}\n\n**Importance:** ${k.importance}/4`).trim()}</pre>
        </div>
      ))}</div>
    )}
  </div>;
}

function DraftsTab({ wid, pid }: { wid: string; pid: string }) {
  const { toast } = useToast();
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<Draft | null>(null);
  const base = `/workspaces/${wid}/projects/${pid}/drafts`;

  async function load() { setLoading(true); try { setDrafts(await request<Draft[]>(base) ?? []); } finally { setLoading(false); } }
  useEffect(() => { load(); }, [pid]);

  async function act(id: string, action: string) {
    try {
      await request(`${base}/${id}`, { method: "PATCH", body: { status: action === "approve" ? "approved" : "rejected" } });
      setSelected(null);
      toast(action === "approve" ? "Approved! Promote to knowledge" : "Rejected", "success");
      await load();
    } catch (e) { toast((e as Error).message || "Failed", "error"); }
  }

  if (loading) return <div className="empty-state" style={{ marginTop: "var(--space-5)" }}><h3>Loading drafts...</h3></div>;
  if (drafts.length === 0) return <div className="empty-state"><div className="empty-state__icon">·</div><h3>No drafts</h3><p>Drafts appear after webhook events are processed.</p></div>;

  return (
    <div className="grid-2" style={{ gridTemplateColumns: "1fr 2fr" }}>
      <div className="col gap-2">
        {drafts.map((d) => (
          <button key={d.id} className="card card--compact" onClick={() => setSelected(d)} style={{ textAlign: "left", width: "100%", borderColor: selected?.id === d.id ? "var(--gray-900)" : undefined }}>
            <div className="proj-card__title">{d.suggested_title || "Untitled"}</div>
            <span className={`pill ${d.status === "approved" ? "pill--success" : d.status === "rejected" ? "pill--warning" : "pill--default"}`} style={{ marginTop: "var(--space-1)" }}>{d.status}</span>
          </button>
        ))}
      </div>
      <div className="card">
        {!selected ? (
          <div className="text-dim text-sm" style={{ padding: 20, textAlign: "center" }}>Select a draft to review</div>
        ) : (
          <div className="col gap-3">
            <h3>{selected.suggested_title}</h3>
            <div className="row gap-1">
              <span className={`pill ${selected.status === "approved" ? "pill--success" : selected.status === "rejected" ? "pill--warning" : "pill--default"}`}>{selected.status}</span>
              <span className="pill pill--default">Importance: {selected.importance}</span>
            </div>
            <div className="text-sm" style={{ lineHeight: 1.6, color: "var(--gray-600)", background: "var(--gray-50)", padding: 12, borderRadius: "var(--radius)" }}>
              {selected.summary || selected.content || "(no body)"}
            </div>
            {selected.status === "draft" && (
              <div className="row gap-1">
                <button className="btn btn--primary btn--sm" onClick={() => act(selected.id, "approve")}><Check size={13} /> Approve</button>
                <button className="btn btn--ghost btn--sm" style={{ color: "var(--red-500)" }} onClick={() => act(selected.id, "reject")}><X size={13} /> Reject</button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function TimelineTab({ wid, pid }: { wid: string; pid: string }) {
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => { setLoading(true); request<TimelineEvent[]>(`/workspaces/${wid}/projects/${pid}/timeline`).then((d) => setEvents(d ?? [])).finally(() => setLoading(false)); }, [pid]);
  if (loading) return <div className="empty-state" style={{ marginTop: "var(--space-5)" }}><h3>Loading...</h3></div>;
  if (events.length === 0) return <div className="empty-state"><div className="empty-state__icon">·</div><h3>No activity yet</h3></div>;
  return <div className="col gap-2">{events.map((e) => (
    <div key={e.id} className="card card--compact">
      <div className="row justify-between">
        <div className="text-strong text-sm">{(e.payload?.title as string) || e.event_type.replace(/_/g, " ")}</div>
        <div className="text-dim text-xs">{fmtDate(e.created_at)}</div>
      </div>
      <div className="row gap-1" style={{ marginTop: "var(--space-1)" }}>
        <span className="pill pill--default">{e.event_type}</span>
        <span className="pill pill--default">{e.entity_type}</span>
      </div>
    </div>
  ))}</div>;
}

function MembersTab({ wid, pid }: { wid: string; pid: string }) {
  const { toast } = useToast();
  const [members, setMembers] = useState<Member[]>([]);
  const [loading, setLoading] = useState(true);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("developer");
  const [err, setErr] = useState("");

  async function load() { setLoading(true); try { setMembers(await request<Member[]>(`/workspaces/${wid}/projects/${pid}/members`) ?? []); } catch { setMembers([]); } finally { setLoading(false); } }
  useEffect(() => { load(); }, [pid]);

  async function invite(e: React.FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await request(`/workspaces/${wid}/projects/${pid}/members/invitations`, { method: "POST", body: { email, role } });
      setEmail("");
      toast(`Invitation sent to ${email}`, "success");
      await load();
    } catch (e) { const msg = (e as Error).message; setErr(msg); toast(msg, "error"); }
  }

  if (loading) return <div className="empty-state" style={{ marginTop: "var(--space-5)" }}><h3>Loading...</h3></div>;

  return (
    <div className="col gap-3" style={{ maxWidth: 520 }}>
      <form onSubmit={invite} className="card card--compact row gap-2">
        <input className="input" type="email" placeholder="colleague@example.com" value={email} onChange={(e) => setEmail(e.target.value)} style={{ flex: 1 }} />
        <select className="input" style={{ width: 110 }} value={role} onChange={(e) => setRole(e.target.value)}>
          <option value="owner">Owner</option>
          <option value="tech_lead">Tech Lead</option>
          <option value="developer">Developer</option>
          <option value="viewer">Viewer</option>
        </select>
        <button type="submit" className="btn btn--primary btn--sm"><ExternalLink size={12} /> Invite</button>
      </form>
      {err && <p className="text-sm" style={{ color: "var(--red-500)" }}>{err}</p>}
      {members.length === 0 ? (
        <div className="empty-state"><div className="empty-state__icon">·</div><h3>No members yet</h3></div>
      ) : (
        <div className="card card--compact">
          {members.map((m) => (
            <div key={m.user_id} className="row gap-2" style={{ padding: "var(--space-1) 0" }}>
              <div className="avatar avatar--sm">{m.full_name.split(" ").map(s => s[0]).join("").slice(0, 2)}</div>
              <div className="flex-1" style={{ minWidth: 0 }}>
                <span className="text-strong text-sm">{m.full_name}</span>
                <span className="pill pill--default" style={{ marginLeft: 4, fontSize: 10, height: 18 }}>{m.role}</span>
              </div>
              <div className="text-dim text-xs">{m.email}</div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
