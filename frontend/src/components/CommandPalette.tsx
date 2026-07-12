import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Folder, LayoutDashboard, Search, Settings } from "lucide-react";
import { useAuth } from "../lib/auth";
import { request } from "../lib/api";
import type { Workspace, Project } from "../lib/api";

type Props = { onClose: () => void };

export default function CommandPalette({ onClose }: Props) {
  const [q, setQ] = useState("");
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [allProjects, setAllProjects] = useState<Project[]>([]);
  const nav = useNavigate();
  const { workspaceId } = useAuth();

  useEffect(() => {
    request<Workspace[]>("/workspaces").then((data) => {
      setWorkspaces(data ?? []);
      const ws = data ?? [];
      Promise.all(ws.map((w) =>
        request<Project[]>(`/workspaces/${w.id}/projects`).then((p) => p ?? []).catch(() => [] as Project[])
      )).then((results) => {
        setAllProjects(results.flat());
      });
    }).catch(() => {});
  }, []);

  const items = useMemo(() => {
    const base = [
      { id: "d", label: "Go to Dashboard", to: "/dashboard", Icon: LayoutDashboard },
      { id: "w", label: "Browse Workspaces", to: "/workspaces", Icon: Folder },
      { id: "s", label: "Search", to: "/search", Icon: Search },
      { id: "st", label: "Settings", to: "/settings", Icon: Settings },
    ];
    const projects = allProjects.map((p) => ({
      id: p.id,
      label: `${p.name}`,
      to: `/workspaces/${p.workspace_id}/projects/${p.id}`,
      Icon: Folder,
    }));
    const all = [...base, ...projects];
    if (!q.trim()) return all;
    return all.filter((i) => i.label.toLowerCase().includes(q.toLowerCase()));
  }, [q, allProjects]);

  return (
    <div className="palette-backdrop" onClick={onClose}>
      <div className="palette" onClick={(e) => e.stopPropagation()}>
        <input
          autoFocus
          className="palette__input"
          placeholder="Type a command or search..."
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <div className="palette__list">
          {items.length === 0 && (
            <div style={{ padding: 24, textAlign: "center", color: "var(--stone)" }}>
              No results
            </div>
          )}
          {items.map(({ id, label, to, Icon }) => (
            <button
              key={id}
              className="palette__item"
              onClick={() => {
                nav(to);
                onClose();
              }}
            >
              <Icon size={16} />
              <span>{label}</span>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
