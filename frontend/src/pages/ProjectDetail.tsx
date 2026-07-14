import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Check, X, ExternalLink, RefreshCw, Sparkles, Download, FileText, Edit, BookOpen, GitBranch, Activity, Search, Loader, AlertCircle, CheckCircle, XCircle } from "lucide-react";
import { request, fmtDate, humanEvent, getSourceLabel, getCategoryLabel } from "../lib/api";
import type { Project, Repository, RepositorySummary, KnowledgeItem, Draft, TimelineEvent, GitHubStatus } from "../lib/api";
import NotFound from "./NotFound";
import { useToast } from "../lib/toast";

const TABS = ["Overview", "Repository", "Knowledge", "Drafts", "Timeline"];

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
        {project.description && <p>{project.description}</p>}
      </div>
      <div className="tabs">
        {TABS.map((t) => (
          <button key={t} className={`tab ${tab === t ? "active" : ""}`} onClick={() => nav(`/workspaces/${wid}/projects/${pid}/${t}`)}>{t}</button>
        ))}
      </div>
      {tab === "Overview" && <OverviewTab wid={wid!} pid={pid!} project={project} />}
      {tab === "Repository" && <RepositoryTab wid={wid!} pid={pid!} />}
      {tab === "Knowledge" && <KnowledgeTab wid={wid!} pid={pid!} />}
      {tab === "Drafts" && <DraftsTab wid={wid!} pid={pid!} />}
      {tab === "Timeline" && <TimelineTab wid={wid!} pid={pid!} />}
    </div>
  );
}

function OverviewTab({ wid, pid, project }: { wid: string; pid: string; project: Project }) {
  const [repo, setRepo] = useState<Repository | null>(null);
  const [knowledge, setKnowledge] = useState<KnowledgeItem[]>([]);
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const base = `/workspaces/${wid}/projects/${pid}`;

  useEffect(() => {
    setLoading(true);
    Promise.all([
      request<Repository>(`${base}/repository`).then(setRepo).catch(() => setRepo(null)),
      request<KnowledgeItem[]>(`${base}/knowledge-items`).then((d) => setKnowledge(d ?? [])).catch(() => setKnowledge([])),
      request<Draft[]>(`${base}/drafts`).then((d) => setDrafts(d ?? [])).catch(() => setDrafts([])),
      request<TimelineEvent[]>(`${base}/timeline`).then((d) => setEvents(d ?? [])).catch(() => setEvents([])),
    ]).finally(() => setLoading(false));
  }, [pid]);

  if (loading) return <div className="grid-3" style={{ marginTop: "var(--space-5)" }}>{[1,2,3,4].map(i => <div key={i} className="card card--compact"><div className="skeleton" style={{height:38}} /></div>)}</div>;

  const draftCount = drafts.filter((d) => d.status === "draft").length;
  const approved = knowledge.filter((k) => k.status === "active");
  const recentApproved = approved.slice(0, 3);
  const recentEvents = events.slice(0, 5);

  return (
    <div className="col gap-5">
      <div className="stat-grid">
        <div className="stat-card">
          <div className="stat-card__value">{repo ? 1 : 0}</div>
          <div className="stat-card__label"><GitBranch size={12} /> Repository</div>
        </div>
        <div className="stat-card">
          <div className="stat-card__value">{approved.length}</div>
          <div className="stat-card__label"><BookOpen size={12} /> Knowledge Items</div>
        </div>
        <div className="stat-card">
          <div className="stat-card__value">{draftCount}</div>
          <div className="stat-card__label"><FileText size={12} /> Drafts to Review</div>
        </div>
        <div className="stat-card">
          <div className="stat-card__value">{project.sources?.length || 0}</div>
          <div className="stat-card__label"><Activity size={12} /> Sources</div>
        </div>
      </div>

      <div className="grid-2">
        <div className="card">
          <div className="row gap-2" style={{ marginBottom: "var(--space-3)" }}>
            <GitBranch size={14} />
            <h3>Repository</h3>
          </div>
          {repo ? (
            <div className="col gap-2">
              <div className="row justify-between">
                <span className="text-sm text-muted">Repository</span>
                <span className="text-sm text-strong">{repo.full_name || `${repo.owner}/${repo.repo_name}`}</span>
              </div>
              <div className="row justify-between">
                <span className="text-sm text-muted">Branch</span>
                <span className="text-sm text-strong">{repo.default_branch || "main"}</span>
              </div>
              <div className="row justify-between">
                <span className="text-sm text-muted">Status</span>
                <span className="pill pill--success">{repo.status}</span>
              </div>
            </div>
          ) : (
            <div className="empty-state" style={{ padding: "var(--space-6)" }}>
              <p>No repository connected.</p>
            </div>
          )}
        </div>

        <div className="card">
          <div className="row gap-2" style={{ marginBottom: "var(--space-3)" }}>
            <Activity size={14} />
            <h3>Knowledge Sources</h3>
          </div>
          <div className="col">
            {(project.sources?.length ?? 0) > 0 ? (
              project.sources!.map((s) => (
                <div key={s.source_key} className="row justify-between" style={{ padding: "var(--space-1) 0" }}>
                  <span className="text-sm text-muted">{s.source_key.replace(/_/g, " ")}</span>
                  {s.enabled ? <span className="pill pill--success">Enabled</span> : <span className="pill pill--default">Disabled</span>}
                </div>
              ))
            ) : (
              <div className="text-dim text-sm">No sources configured</div>
            )}
          </div>
        </div>
      </div>

      <div className="grid-2">
        {recentEvents.length > 0 && (
          <div className="card">
            <div className="row gap-2" style={{ marginBottom: "var(--space-3)" }}>
              <Activity size={14} />
              <h3>Recent Activity</h3>
            </div>
            <div className="col gap-2">
              {recentEvents.map((e) => (
                <div key={e.id} className="row justify-between">
                  <span className="text-sm">{humanEvent(e.event_type, e.payload)}</span>
                  <span className="text-dim text-xs">{fmtDate(e.created_at)}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {recentApproved.length > 0 && (
          <div className="card">
            <div className="row gap-2" style={{ marginBottom: "var(--space-3)" }}>
              <BookOpen size={14} />
              <h3>Recent Knowledge</h3>
            </div>
            <div className="col gap-2">
              {recentApproved.map((k) => (
                <div key={k.id} className="col">
                  <span className="text-sm text-strong">{k.title}</span>
                  <span className="text-dim text-xs">{fmtDate(k.approved_at || k.created_at)}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function RepositoryTab({ wid, pid }: { wid: string; pid: string }) {
  const { toast } = useToast();
  const [repo, setRepo] = useState<Repository | null>(null);
  const [gh, setGh] = useState<GitHubStatus | null>(null);
  const [ghLoading, setGhLoading] = useState(true);
  const [repos, setRepos] = useState<RepositorySummary[]>([]);
  const [reposLoading, setReposLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [connecting, setConnecting] = useState(false);
  const [err, setErr] = useState("");
  const base = `/workspaces/${wid}/projects/${pid}/repository`;

  async function load() {
    setGhLoading(true);
    try { setRepo(await request<Repository>(base)); }
    catch { setRepo(null); }
    try { setGh(await request<GitHubStatus>("/github/account")); }
    catch { setGh(null); }
    finally { setGhLoading(false); }
  }

  useEffect(() => { load(); }, [pid]);

  async function loadRepos(query = "") {
    setReposLoading(true);
    setErr("");
    try {
      const params = query ? `?query=${encodeURIComponent(query)}` : "";
      const data = await request<RepositorySummary[]>(`${base}/options${params}`);
      setRepos(data ?? []);
    } catch (e) {
      setRepos([]);
      if (!query) setErr("Could not load repositories. Make sure your GitHub account is connected.");
    } finally {
      setReposLoading(false);
    }
  }

  useEffect(() => {
    if (gh?.connected && !repo) {
      loadRepos();
    }
  }, [gh?.connected, repo?.id]);

  async function connectRepo(summary: RepositorySummary) {
    setConnecting(true);
    setErr("");
    try {
      await request(base, { method: "PUT", body: { owner: summary.owner, repo_name: summary.repo_name } });
      toast("Repository connected", "success");
      await load();
    } catch (e) {
      const msg = (e as Error).message || "Failed to connect repository";
      setErr(msg);
      toast(msg, "error");
    } finally {
      setConnecting(false);
    }
  }

  async function disconnectRepo() {
    setConnecting(true);
    try {
      await request(base, { method: "DELETE" });
      toast("Repository disconnected", "info");
      setRepo(null);
      loadRepos();
    } catch (e) {
      toast((e as Error).message || "Failed to disconnect", "error");
    } finally {
      setConnecting(false);
    }
  }

  if (ghLoading) return <div className="col gap-3" style={{ marginTop: "var(--space-5)", maxWidth: 600 }}>{[1,2].map(i => <div key={i} className="card card--compact"><div className="skeleton" style={{height:48}} /></div>)}</div>;

  return (
    <div className="col gap-4" style={{ maxWidth: 600 }}>
      <div className="card">
        <div className="row gap-2" style={{ marginBottom: "var(--space-3)" }}>
          <ExternalLink size={14} />
          <h3>GitHub Account</h3>
        </div>
        {gh === null ? (
          <div className="row gap-2" style={{ color: "var(--red-500)" }}>
            <XCircle size={14} />
            <span className="text-sm">Could not check GitHub connection</span>
          </div>
        ) : gh.connected ? (
          <div className="row justify-between">
            <div className="row gap-2">
              <CheckCircle size={14} style={{ color: "var(--green-500)" }} />
              <div>
                <span className="text-sm text-strong">Connected</span>
                <span className="text-dim text-xs" style={{ marginLeft: 8 }}>GitHub account linked</span>
              </div>
            </div>
            <span className="pill pill--success">Active</span>
          </div>
        ) : (
          <div className="col gap-3">
            <div className="row gap-2">
              <AlertCircle size={14} style={{ color: "var(--amber-500)" }} />
              <span className="text-sm text-strong">Not connected</span>
            </div>
            <p className="text-sm text-muted">Connect a GitHub account to browse and link repositories.</p>
            <button className="btn btn--primary btn--sm" style={{ alignSelf: "flex-start" }} onClick={async () => {
              try {
                const res = await request<{ authorization_url: string }>("/github/connect");
                if (res?.authorization_url) window.location.href = res.authorization_url;
              } catch (e) { toast((e as Error).message || "Failed to start GitHub OAuth", "error"); }
            }}>
              Connect GitHub
            </button>
          </div>
        )}
      </div>

      {gh?.connected && !repo && (
        <div className="card">
          <h3 style={{ marginBottom: "var(--space-2)" }}>Select a Repository</h3>
          <div className="row gap-2" style={{ marginBottom: "var(--space-3)" }}>
            <div className="input" style={{ display: "flex", alignItems: "center", gap: "var(--space-2)", flex: 1 }}>
              <Search size={14} style={{ color: "var(--gray-400)", flexShrink: 0 }} />
              <input
                style={{ border: "none", background: "none", outline: "none", flex: 1, padding: 0, fontSize: 13 }}
                placeholder="Search repositories..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") loadRepos(searchQuery); }}
                aria-label="Search repositories"
              />
            </div>
            <button className="btn btn--ghost btn--sm" onClick={() => loadRepos(searchQuery)} disabled={reposLoading}>
              {reposLoading ? <Loader size={12} /> : <Search size={12} />}
            </button>
          </div>

          {err && <p className="text-sm" style={{ color: "var(--red-500)", marginBottom: "var(--space-2)" }}>{err}</p>}

          {reposLoading ? (
            <div className="col gap-2">
              {[1, 2, 3].map((i) => (
                <div key={i} className="skeleton" style={{ height: 36, width: "100%" }} />
              ))}
            </div>
          ) : repos.length === 0 ? (
            <div className="text-dim text-sm" style={{ padding: "var(--space-4) 0", textAlign: "center" }}>
              {searchQuery ? "No repositories match your search." : "No repositories found. Make sure your GitHub account has access to repositories."}
            </div>
          ) : (
            <div className="col gap-1" style={{ maxHeight: 300, overflowY: "auto" }}>
              {repos.map((r) => (
                <button
                  key={r.full_name}
                  className="row gap-2"
                  style={{ padding: "var(--space-2) var(--space-3)", borderRadius: "var(--radius)", width: "100%", textAlign: "left", cursor: "pointer", background: "var(--gray-50)", border: "none" }}
                  onClick={() => connectRepo(r)}
                  disabled={connecting}
                  onMouseEnter={(e) => (e.currentTarget.style.background = "var(--gray-100)")}
                  onMouseLeave={(e) => (e.currentTarget.style.background = "var(--gray-50)")}
                >
                  <GitBranch size={14} style={{ color: "var(--gray-400)", flexShrink: 0 }} />
                  <div className="col" style={{ minWidth: 0, flex: 1 }}>
                    <span className="text-sm text-strong">{r.full_name}</span>
                    <span className="text-dim text-xs">{r.default_branch || "main"}{r.private ? " · Private" : ""}</span>
                  </div>
                  <span className="btn btn--primary btn--sm" onClick={(e) => { e.stopPropagation(); connectRepo(r); }} style={{ pointerEvents: connecting ? "none" : "auto" }}>
                    {connecting ? <Loader size={12} /> : "Connect"}
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {gh?.connected && repo && (
        <div className="card">
          <div className="row justify-between" style={{ marginBottom: "var(--space-3)" }}>
            <div>
              <span className="text-strong">{repo.full_name || `${repo.owner}/${repo.repo_name}`}</span>
              <span className="text-dim text-sm" style={{ marginLeft: 8 }}>Branch: {repo.default_branch || "main"}</span>
            </div>
            <span className="pill pill--success">{repo.status}</span>
          </div>
          <div className="col gap-2">
            <div className="row justify-between">
              <span className="text-sm text-muted">Default Branch</span>
              <span className="text-sm text-strong">{repo.default_branch || "main"}</span>
            </div>
            <div className="row justify-between">
              <span className="text-sm text-muted">Webhook</span>
              <span className={`pill ${repo.webhook_id ? "pill--success" : "pill--default"}`}>
                {repo.webhook_id ? "Active" : "Inactive"}
              </span>
            </div>
            <div className="row justify-between">
              <span className="text-sm text-muted">Connected</span>
              <span className="text-sm text-strong">{fmtDate(repo.connected_at)}</span>
            </div>
          </div>
          {err && <p className="text-sm" style={{ color: "var(--red-500)", marginTop: "var(--space-2)" }}>{err}</p>}
          <div className="row gap-1" style={{ marginTop: "var(--space-3)" }}>
            <button className="btn btn--ghost btn--sm" onClick={async () => {
              setConnecting(true);
              try { await request(`${base}/webhook/sync`, { method: "POST" }); toast("Webhook synced", "success"); await load(); }
              catch (e) { toast((e as Error).message, "error"); }
              finally { setConnecting(false); }
            }} disabled={connecting}>
              <RefreshCw size={12} /> Sync Webhook
            </button>
            <button className="btn btn--outline btn--sm" onClick={disconnectRepo} disabled={connecting}>
              Change Repository
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function downloadMd(content: string, filename: string) {
  const blob = new Blob([content], { type: "text/markdown" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url; a.download = filename; a.click();
  URL.revokeObjectURL(url);
}

function printPdf(title: string, content: string) {
  const win = window.open("", "_blank");
  if (!win) return;
  win.document.write(`<!DOCTYPE html><html><head><title>${title}</title><style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; font-size: 14px; line-height: 1.6; padding: 40px; max-width: 800px; margin: 0 auto; color: #17171c; }
    h1 { font-size: 22px; font-weight: 650; margin-bottom: 4px; }
    h2 { font-size: 18px; font-weight: 650; margin-top: 24px; margin-bottom: 8px; }
    p { margin: 0 0 8px 0; color: #555561; }
    hr { border: none; border-top: 1px solid #e9e9ec; margin: 24px 0; }
    .meta { font-size: 12px; color: #94949e; margin-bottom: 16px; }
    @media print { @page { margin: 20mm; } }
  </style></head><body>${content}</body></html>`);
  win.document.close();
  win.focus();
  setTimeout(() => { win.print(); }, 500);
}

function exportDeveloperGuide(items: KnowledgeItem[]): string {
  return items.filter((k) => k.status === "active").map((k) => {
    const date = k.approved_at || k.created_at;
    return `# ${k.title}\n\n${k.summary}\n\n${k.body || k.decision_body || ""}\n\n---\n*Source: ${getSourceLabel(k)} | ${fmtDate(date)}*\n`;
  }).join("\n\n");
}

function exportAgentContext(items: KnowledgeItem[]): string {
  return items.filter((k) => k.status === "active").map((k) => {
    return `## ${k.title}\n\n**Summary:** ${k.summary}\n\n**Source:** ${getSourceLabel(k)}\n**Category:** ${getCategoryLabel(k)}\n\n${k.agents_md || k.decision_body || k.body || ""}\n`;
  }).join("\n\n");
}

function KnowledgeTab({ wid, pid }: { wid: string; pid: string }) {
  const [items, setItems] = useState<KnowledgeItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<KnowledgeItem | null>(null);
  const base = `/workspaces/${wid}/projects/${pid}/knowledge-items`;

  async function load() { setLoading(true); try { setItems(await request<KnowledgeItem[]>(base) ?? []); } finally { setLoading(false); } }
  useEffect(() => { load(); }, [pid]);

  const approved = items.filter((k) => k.status === "active");

  if (loading) return <div className="grid-2" style={{ marginTop: "var(--space-5)" }}>{[1,2].map(i => <div key={i} className="card card--compact"><div className="skeleton" style={{height:48}} /><div className="skeleton" style={{height:12,marginTop:8,width:"60%"}} /></div>)}</div>;

  if (approved.length === 0) return (
    <div className="empty-state">
      <div className="empty-state__icon"><BookOpen size={18} /></div>
      <h3>No knowledge yet</h3>
      <p>Approve AI-generated drafts to build your project knowledge base.</p>
    </div>
  );

  const sorted = [...approved].sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());

  function handleExport(type: "devguide" | "agentctx", format: "md" | "pdf") {
    const content = type === "devguide" ? exportDeveloperGuide(sorted) : exportAgentContext(sorted);
    const label = type === "devguide" ? "developer-guide" : "agent-context";
    const filename = `${label}.${format}`;
    if (format === "md") {
      downloadMd(content, filename);
    } else {
      const htmlTitle = `KNOWell - ${type === "devguide" ? "Developer Guide" : "Agent Context"}`;
      const htmlContent = content.split("\n").map((line) => {
        if (line.startsWith("# ")) return `<h1>${line.slice(2)}</h1>`;
        if (line.startsWith("## ")) return `<h2>${line.slice(3)}</h2>`;
        if (line.startsWith("---")) return `<hr>`;
        if (line.startsWith("*")) return `<p class="meta">${line.slice(1, -1)}</p>`;
        if (line.trim() === "") return "";
        return `<p>${line}</p>`;
      }).filter(Boolean).join("\n");
      printPdf(htmlTitle, htmlContent);
    }
  }

  return (
    <div className="col gap-4">
      <div className="row justify-between" style={{ flexWrap: "wrap", gap: "var(--space-2)" }}>
        <span className="text-sm text-muted">{sorted.length} knowledge item{sorted.length !== 1 ? "s" : ""}</span>
        <div className="export-group">
          <button className="btn btn--ghost btn--sm" onClick={() => handleExport("devguide", "md")}>
            <Download size={12} /> Developer Guide (.md)
          </button>
          <button className="btn btn--ghost btn--sm" onClick={() => handleExport("devguide", "pdf")}>
            <FileText size={12} /> Developer Guide (.pdf)
          </button>
          <button className="btn btn--ghost btn--sm" onClick={() => handleExport("agentctx", "md")}>
            <Download size={12} /> Agent Context (.md)
          </button>
          <button className="btn btn--ghost btn--sm" onClick={() => handleExport("agentctx", "pdf")}>
            <FileText size={12} /> Agent Context (.pdf)
          </button>
        </div>
      </div>

      <div className="grid-2" style={{ gridTemplateColumns: selected ? "1fr 1.5fr" : "1fr" }}>
        <div className="col gap-2">
          {sorted.map((k) => (
            <button
              key={k.id}
              className="card card--compact"
              onClick={() => setSelected(selected?.id === k.id ? null : k)}
              style={{ textAlign: "left", width: "100%", borderColor: selected?.id === k.id ? "var(--gray-900)" : undefined, cursor: "pointer" }}
            >
              <div className="proj-card__title">{k.title}</div>
              <div className="proj-card__desc" style={{ marginTop: "var(--space-1)" }}>{k.summary}</div>
              <div className="row gap-2" style={{ marginTop: "var(--space-2)" }}>
                <span className="pill pill--default">{getCategoryLabel(k)}</span>
                <span className="pill pill--default">{getSourceLabel(k)}</span>
                <span className="text-dim text-xs" style={{ marginLeft: "auto" }}>{fmtDate(k.updated_at)}</span>
              </div>
            </button>
          ))}
        </div>

        {selected && (
          <div className="card">
            <h3>{selected.title}</h3>
            <p className="text-sm text-muted" style={{ marginTop: "var(--space-1)" }}>{selected.summary}</p>

            <div className="knowledge-content">
              {selected.body || selected.decision_body || "No additional content"}
            </div>

            <div className="knowledge-detail-grid">
              {selected.pull_request_id && (
                <div className="knowledge-detail-item">
                  <span className="knowledge-detail-item__label">Related PR</span>
                  <span className="knowledge-detail-item__value">{selected.pull_request_id}</span>
                </div>
              )}
              {selected.commit_id && (
                <div className="knowledge-detail-item">
                  <span className="knowledge-detail-item__label">Related Commit</span>
                  <span className="knowledge-detail-item__value">{selected.commit_id.slice(0, 8)}</span>
                </div>
              )}
              <div className="knowledge-detail-item">
                <span className="knowledge-detail-item__label">Source</span>
                <span className="knowledge-detail-item__value">{getSourceLabel(selected)}</span>
              </div>
              <div className="knowledge-detail-item">
                <span className="knowledge-detail-item__label">Created</span>
                <span className="knowledge-detail-item__value">{fmtDate(selected.created_at)}</span>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function DraftsTab({ wid, pid }: { wid: string; pid: string }) {
  const { toast } = useToast();
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<Draft | null>(null);
  const [aiDown, setAiDown] = useState(false);
  const base = `/workspaces/${wid}/projects/${pid}/drafts`;

  useEffect(() => {
    request<{ available: boolean }>("/ai/status").then((s) => setAiDown(!s.available)).catch(() => setAiDown(true));
  }, []);

  async function load() { setLoading(true); try { setDrafts(await request<Draft[]>(base) ?? []); } finally { setLoading(false); } }
  useEffect(() => { load(); }, [pid]);

  async function act(id: string, status: string) {
    try {
      await request(`${base}/${id}`, { method: "PATCH", body: { status } });
      if (status === "approved") toast("Draft approved — promote it to knowledge.", "success");
      else if (status === "rejected") toast("Draft rejected", "info");
      else if (status === "archived") toast("Draft archived", "info");
      else toast("Draft updated", "success");
      setSelected(null);
      await load();
    } catch (e) { toast((e as Error).message || "Failed to update draft", "error"); }
  }

  if (loading) return <div className="grid-2" style={{ marginTop: "var(--space-5)" }}>{[1,2].map(i => <div key={i} className="card card--compact"><div className="skeleton" style={{height:48}} /><div className="skeleton" style={{height:12,marginTop:8,width:"40%"}} /></div>)}</div>;

  const sorted = [...drafts].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());

  if (sorted.length === 0) return (
    <div className="col gap-3">
      <div className="empty-state">
        <div className="empty-state__icon"><FileText size={18} /></div>
        <h3>No drafts</h3>
        <p>Review and approve AI-generated drafts to build your knowledge base.</p>
      </div>
      {aiDown && (
        <div className="card card--compact" style={{ borderLeft: "3px solid var(--yellow-400)" }}>
          <div className="row gap-2">
            <Sparkles size={16} />
            <div className="text-sm"><strong>AI Generation</strong> is temporarily unavailable.</div>
          </div>
        </div>
      )}
    </div>
  );

  const getSourceLabel_draft = (d: Draft) => d.source_type === "pull_request" ? "Pull Request" : d.source_type === "commit" ? "Commit" : d.source_type;

  return (
    <div className="grid-2" style={{ gridTemplateColumns: "1fr 2fr" }}>
      <div className="col gap-2">
        {sorted.map((d) => (
          <button
            key={d.id}
            className="card card--compact"
            onClick={() => setSelected(d)}
            style={{ textAlign: "left", width: "100%", borderColor: selected?.id === d.id ? "var(--gray-900)" : undefined }}
          >
            <div className="proj-card__title">{d.suggested_title || "Untitled"}</div>
            <div className="row gap-1" style={{ marginTop: "var(--space-1)" }}>
              <span className={`pill ${d.status === "approved" ? "pill--success" : d.status === "rejected" ? "pill--warning" : d.status === "archived" ? "pill--default" : "pill--default"}`}>
                {d.status}
              </span>
              <span className="pill pill--default">{getSourceLabel_draft(d)}</span>
            </div>
            <div className="text-dim text-xs" style={{ marginTop: "var(--space-1)" }}>{fmtDate(d.created_at)}</div>
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
              <span className={`pill ${selected.status === "approved" ? "pill--success" : selected.status === "rejected" ? "pill--warning" : selected.status === "archived" ? "pill--default" : "pill--default"}`}>
                {selected.status}
              </span>
              <span className="pill pill--default">{getSourceLabel_draft(selected)}</span>
            </div>

            {selected.reason && (
              <div className="draft-detail-section">
                <div className="text-strong text-sm" style={{ marginBottom: "var(--space-1)" }}>Reason</div>
                <div>{selected.reason}</div>
              </div>
            )}

            <div className="draft-detail-section">
              <div className="text-strong text-sm" style={{ marginBottom: "var(--space-1)" }}>Summary</div>
              <div>{selected.summary || "No summary available"}</div>
            </div>

            {selected.raw_input_json && (
              <div className="draft-detail-section">
                <div className="text-strong text-sm" style={{ marginBottom: "var(--space-1)" }}>
                  {getSourceLabel_draft(selected) === "Commit" ? "Commit Summary" : "Files Changed"}
                </div>
                <div>
                  {selected.raw_input_json.message as string || selected.raw_input_json.title as string || "—"}
                </div>
              </div>
            )}

            <div className="row gap-1" style={{ flexWrap: "wrap" }}>
              {selected.status === "draft" && (
                <>
                  <button className="btn btn--primary btn--sm" onClick={() => act(selected.id, "approved")}>
                    <Check size={13} /> Approve
                  </button>
                  <button className="btn btn--ghost btn--sm" style={{ color: "var(--red-500)" }} onClick={() => act(selected.id, "rejected")}>
                    <X size={13} /> Reject
                  </button>
                  <button className="btn btn--ghost btn--sm" onClick={() => act(selected.id, "archived")}>
                    Archive
                  </button>
                </>
              )}
              {selected.status === "approved" && (
                <button className="btn btn--primary btn--sm" onClick={() => act(selected.id, "archived")}>
                  Archive
                </button>
              )}
              {selected.status === "rejected" && (
                <button className="btn btn--ghost btn--sm" onClick={() => act(selected.id, "archived")}>
                  Archive
                </button>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function TimelineTab({ wid, pid }: { wid: string; pid: string }) {
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    setLoading(true);
    request<TimelineEvent[]>(`/workspaces/${wid}/projects/${pid}/timeline`).then((d) => setEvents(d ?? [])).finally(() => setLoading(false));
  }, [pid]);

  if (loading) return <div className="col gap-2" style={{ marginTop: "var(--space-5)", maxWidth: 600 }}>{[1,2,3].map(i => <div key={i} className="card card--compact"><div className="skeleton" style={{height:28}} /></div>)}</div>;

  if (events.length === 0) return (
    <div className="empty-state">
      <div className="empty-state__icon"><Activity size={18} /></div>
      <h3>No activity yet</h3>
      <p>Events appear after connecting a repository and processing webhooks.</p>
    </div>
  );

  return (
    <div className="col gap-2" style={{ maxWidth: 600 }}>
      {events.map((e) => (
        <div key={e.id} className="card card--compact">
          <div className="row justify-between">
            <span className="text-strong text-sm">{humanEvent(e.event_type, e.payload)}</span>
            <span className="text-dim text-xs">{fmtDate(e.created_at)}</span>
          </div>
        </div>
      ))}
    </div>
  );
}
