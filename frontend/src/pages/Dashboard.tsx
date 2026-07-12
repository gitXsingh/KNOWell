import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { FolderKanban, FolderGit2 } from "lucide-react";
import { useAuth } from "../lib/auth";
import { request, fmtDate } from "../lib/api";
import type { Project, Workspace } from "../lib/api";

export default function Dashboard() {
  const { session } = useAuth();
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [projectsMap, setProjectsMap] = useState<Record<string, Project[]>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    request<Workspace[]>("/workspaces").then((data) => {
      const ws = data ?? [];
      setWorkspaces(ws);
      return Promise.all(
        ws.map((w) =>
          request<Project[]>(`/workspaces/${w.id}/projects`).then((p) => ({ wid: w.id, projects: p ?? [] }))
        )
      );
    }).then((results) => {
      if (!results) return;
      const map: Record<string, Project[]> = {};
      for (const r of results) map[r.wid] = r.projects;
      setProjectsMap(map);
    }).catch(() => {}).finally(() => setLoading(false));
  }, []);

  const user = session?.user;
  const allProjects = Object.values(projectsMap).flat();

  if (loading) {
    return (
      <div className="page-content">
        <div className="page-content__header">
          <div className="skeleton" style={{ height: 22, width: "40%", marginBottom: 6 }} />
          <div className="skeleton" style={{ height: 14, width: "25%" }} />
        </div>
        <div className="grid-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="card card--compact">
              <div className="skeleton" style={{ height: 14, width: "50%", marginBottom: 8 }} />
              <div className="skeleton" style={{ height: 10, width: "30%" }} />
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="page-content">
      <div className="page-content__header">
        <h1>Welcome back{user ? `, ${user.full_name.split(" ")[0]}` : ""}</h1>
        <p>Your workspaces and recent projects at a glance.</p>
      </div>

      <h2 className="page-bar" style={{ marginBottom: "var(--space-4)" }}>Workspaces</h2>

      {workspaces.length === 0 ? (
        <div className="empty-state">
          <div className="empty-state__icon"><FolderKanban size={18} /></div>
          <h3>No workspaces yet</h3>
          <p>Go to Workspaces to create your first one.</p>
          <Link to="/workspaces" className="btn btn--primary btn--sm" style={{ marginTop: "var(--space-4)" }}>
            Create Workspace
          </Link>
        </div>
      ) : (
        <div className="grid-3" style={{ marginBottom: "var(--space-8)" }}>
          {workspaces.map((w) => (
            <Link key={w.id} to={`/workspaces/${w.id}`} className="card" style={{ textDecoration: "none" }}>
              <div className="row gap-3" style={{ marginBottom: "var(--space-2)" }}>
                <div className="avatar" style={{ borderRadius: 8, width: 34, height: 34 }}>
                  {w.name.split(" ").map((s) => s[0]).join("").slice(0, 2)}
                </div>
                <div className="col" style={{ gap: 1, minWidth: 0 }}>
                  <div className="proj-card__title">{w.name}</div>
                  <div className="text-dim text-xs">{w.kind}</div>
                </div>
              </div>
              <div className="text-sm text-muted">{(projectsMap[w.id] || []).length} projects</div>
            </Link>
          ))}
        </div>
      )}

      <h2 className="page-bar" style={{ marginBottom: "var(--space-4)" }}>Recent Projects</h2>

      {allProjects.length === 0 ? (
        <div className="empty-state">
          <div className="empty-state__icon"><FolderGit2 size={18} /></div>
          <h3>No projects yet</h3>
          <p>Create your first project inside a workspace.</p>
        </div>
      ) : (
        <div className="grid-2">
          {allProjects.slice(0, 6).map((p) => (
            <Link
              key={p.id}
              to={`/workspaces/${p.workspace_id}/projects/${p.id}`}
              className="card"
              style={{ textDecoration: "none" }}
            >
              <div className="proj-card__title">{p.name}</div>
              <div className="proj-card__desc" style={{ marginTop: 2 }}>{p.description || "No description"}</div>
              <div className="text-dim text-xs" style={{ marginTop: "var(--space-3)" }}>
                Updated {fmtDate(p.updated_at)}
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
