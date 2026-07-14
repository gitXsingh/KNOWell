import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Search as SearchIcon } from "lucide-react";
import { useAuth } from "../lib/auth";
import { request, fmtDate } from "../lib/api";
import type { Project, Repository, SearchResult } from "../lib/api";

export default function Search() {
  const { workspaceId } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectId, setProjectId] = useState("");
  const [q, setQ] = useState("");
  const [repositoryId, setRepositoryId] = useState("");
  const [repos, setRepos] = useState<Repository[]>([]);
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");
  const [sort, setSort] = useState("newest");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState("");
  const [searched, setSearched] = useState(false);

  useEffect(() => {
    if (!workspaceId) return;
    request<Project[]>(`/workspaces/${workspaceId}/projects`).then((p) => {
      setProjects(p ?? []);
      if (p?.[0]?.id) setProjectId(p[0].id);
    }).catch(() => {});
  }, [workspaceId]);

  useEffect(() => {
    if (!workspaceId || !projectId) return;
    request<Repository>(`/workspaces/${workspaceId}/projects/${projectId}/repository`).then((r) => {
      if (r) setRepos([r]);
    }).catch(() => setRepos([]));
  }, [workspaceId, projectId]);

  async function run(e: React.FormEvent) {
    e.preventDefault();
    if (!projectId) {
      setErr("Select a project to search.");
      return;
    }
    setLoading(true);
    setErr("");
    setSearched(true);
    try {
      const params = new URLSearchParams();
      if (q.trim()) params.set("keyword", q.trim());
      if (repositoryId) params.set("repository_id", repositoryId);
      if (dateFrom) params.set("date_from", dateFrom);
      if (dateTo) params.set("date_to", dateTo);
      if (sort) params.set("sort", sort);
      const r = await request<SearchResult[]>(
        `/workspaces/${workspaceId}/projects/${projectId}/search?${params.toString()}`
      );
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
      </div>

      <form onSubmit={run}>
        <div className="filter-bar">
          <SearchIcon size={15} className="search-bar__icon" />
          <input
            className="input"
            style={{ flex: 1, minWidth: 180, border: "none", background: "transparent", padding: "4px 8px" }}
            placeholder="Keyword..."
            value={q}
            onChange={(e) => setQ(e.target.value)}
            aria-label="Search keyword"
          />
          <select
            className="input"
            style={{ width: "auto", minWidth: 130 }}
            value={projectId}
            onChange={(e) => { setProjectId(e.target.value); setRepositoryId(""); }}
            aria-label="Project"
          >
            {projects.length === 0 && <option value="">No projects</option>}
            {projects.map((p) => (<option key={p.id} value={p.id}>{p.name}</option>))}
          </select>
          {repos.length > 0 && (
            <select
              className="input"
              style={{ width: "auto", minWidth: 130 }}
              value={repositoryId}
              onChange={(e) => setRepositoryId(e.target.value)}
              aria-label="Repository"
            >
              <option value="">All repositories</option>
              {repos.map((r) => (<option key={r.id} value={r.id}>{r.full_name || r.repo_name}</option>))}
            </select>
          )}
          <input
            className="input"
            type="date"
            style={{ width: "auto", minWidth: 110 }}
            value={dateFrom}
            onChange={(e) => setDateFrom(e.target.value)}
            aria-label="From date"
          />
          <input
            className="input"
            type="date"
            style={{ width: "auto", minWidth: 110 }}
            value={dateTo}
            onChange={(e) => setDateTo(e.target.value)}
            aria-label="To date"
          />
          <select
            className="input"
            style={{ width: "auto", minWidth: 100 }}
            value={sort}
            onChange={(e) => setSort(e.target.value)}
            aria-label="Sort order"
          >
            <option value="newest">Newest</option>
            <option value="oldest">Oldest</option>
            <option value="importance">Importance</option>
            <option value="repository">Repository</option>
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
                <span className="pill pill--default">{fmtDate(r.created_at)}</span>
              </div>
            </Link>
          ))}
        </div>
      ) : searched ? (
        <div className="empty-state">
          <div className="empty-state__icon"><SearchIcon size={18} /></div>
          <h3>No results</h3>
          <p>Try different keywords or filters.</p>
        </div>
      ) : null}
    </div>
  );
}
