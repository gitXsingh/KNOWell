import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Plus, Layout } from "lucide-react";
import { useAuth } from "../lib/auth";
import { request } from "../lib/api";
import type { Workspace, Project } from "../lib/api";
import { useToast } from "../lib/toast";

export default function Workspaces() {
  const { refresh } = useAuth();
  const { toast } = useToast();
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [projectsMap, setProjectsMap] = useState<Record<string, Project[]>>({});
  const [newName, setNewName] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function load() {
    const data = await request<Workspace[]>("/workspaces");
    setWorkspaces(data ?? []);
    const ws = data ?? [];
    const map: Record<string, Project[]> = {};
    await Promise.all(
      ws.map((w) =>
        request<Project[]>(`/workspaces/${w.id}/projects`).then((p) => { map[w.id] = p ?? []; }).catch(() => { map[w.id] = []; })
      )
    );
    setProjectsMap(map);
  }

  useEffect(() => { load(); }, []);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    if (!newName.trim()) return;
    setBusy(true);
    setErr("");
    try {
      await request("/workspaces", { method: "POST", body: { name: newName } });
      setNewName("");
      toast("Workspace created", "success");
      await refresh();
      await load();
    } catch (e) {
      const msg = (e as Error).message || "Failed";
      setErr(msg);
      toast(msg, "error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="page-content">
      <div className="page-content__header">
        <h1>Workspaces</h1>
        <p>Group your projects, team, and knowledge.</p>
      </div>

      <form onSubmit={create} className="row gap-2" style={{ marginBottom: "var(--space-6)" }}>
        <input
          className="input"
          placeholder="New workspace name"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          style={{ maxWidth: 300 }}
        />
        <button type="submit" className="btn btn--primary btn--sm" disabled={busy}>
          <Plus size={13} /> Create
        </button>
      </form>
      {err && <p className="text-sm" style={{ color: "var(--red-500)", marginBottom: "var(--space-3)" }}>{err}</p>}

      {workspaces.length === 0 ? (
        <div className="empty-state">
          <div className="empty-state__icon"><Layout size={18} /></div>
          <h3>No workspaces yet</h3>
          <p>Create one above to get started.</p>
        </div>
      ) : (
        <div className="grid-3">
          {workspaces.map((w) => (
            <Link key={w.id} to={`/workspaces/${w.id}`} className="card" style={{ textDecoration: "none" }}>
              <div className="row gap-3" style={{ marginBottom: "var(--space-2)" }}>
                <div className="avatar" style={{ borderRadius: 8, width: 38, height: 38 }}>
                  {w.name.split(" ").map((s) => s[0]).join("").slice(0, 2)}
                </div>
                <div>
                  <div className="proj-card__title">{w.name}</div>
                  <div className="text-dim text-xs">{(projectsMap[w.id] || []).length} projects</div>
                </div>
              </div>
              <span className="pill pill--default">{w.kind}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
