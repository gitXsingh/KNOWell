import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Search as SearchIcon } from "lucide-react";
import { useAuth } from "../lib/auth";
import { request } from "../lib/api";
import type { Project, SearchResult } from "../lib/api";

export default function Search() {
  const { workspaceId } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectId, setProjectId] = useState("");
  const [q, setQ] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState("");

  useEffect(() => {
    if (!workspaceId) return;
    request<Project[]>(`/workspaces/${workspaceId}/projects`).then((p) => {
      setProjects(p ?? []);
      if (p?.[0]?.id) setProjectId(p[0].id);
    }).catch(() => {});
  }, [workspaceId]);

  async function run(e: React.FormEvent) {
    e.preventDefault();
    if (!projectId || !q.trim()) return;
    setLoading(true);
    setErr("");
    try {
      const r = await request<SearchResult[]>(`/workspaces/${workspaceId}/projects/${projectId}/search?keyword=${encodeURIComponent(q)}`);
      setResults(r ?? []);
    } catch (e) {
      setErr((e as Error).message || "Search failed");
      setResults([]);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="page-content">
      <div className="page-content__header">
        <h1>Search Knowledge</h1>
        <p>Full-text search across approved knowledge items.</p>
      </div>

      <form onSubmit={run}>
        <div className="search-bar">
          <SearchIcon size={16} className="search-bar__icon" />
          <input
            placeholder="Search knowledge..."
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
          <select
            className="input"
            style={{ width: "auto", minWidth: 140, border: "none", background: "transparent", fontSize: 13, padding: "4px 8px" }}
            value={projectId}
            onChange={(e) => setProjectId(e.target.value)}
          >
            {projects.length === 0 && <option value="">No projects</option>}
            {projects.map((p) => (<option key={p.id} value={p.id}>{p.name}</option>))}
          </select>
          <button type="submit" className="btn btn--primary btn--sm" disabled={loading}>Search</button>
        </div>
      </form>
      {err && <p className="text-sm" style={{ color: "var(--red-500)", marginBottom: "var(--space-3)" }}>{err}</p>}

      {loading ? (
        <div className="grid-2">
          {[1, 2].map((i) => (
            <div key={i} className="card card--compact">
              <div className="skeleton" style={{ height: 14, width: "60%", marginBottom: 8 }} />
              <div className="skeleton" style={{ height: 10, width: "80%" }} />
            </div>
          ))}
        </div>
      ) : results.length > 0 ? (
        <div className="grid-2">
          {results.map((r) => (
            <Link key={r.id} to={`/workspaces/${workspaceId}/projects/${r.project_id}`} className="card" style={{ textDecoration: "none" }}>
              <div className="proj-card__title">{r.title}</div>
              <div className="proj-card__desc" style={{ marginTop: "var(--space-1)" }}>{r.summary}</div>
              <div className="row gap-2" style={{ marginTop: "var(--space-2)" }}>
                <span className="pill pill--default">Importance: {r.importance}</span>
                <span className="pill pill--default">{r.status}</span>
              </div>
            </Link>
          ))}
        </div>
      ) : q.trim() ? (
        <div className="empty-state">
          <div className="empty-state__icon"><SearchIcon size={18} /></div>
          <h3>No results</h3>
          <p>Try another query or project.</p>
        </div>
      ) : null}
    </div>
  );
}
