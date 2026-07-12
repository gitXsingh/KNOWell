import { useEffect, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { ChevronRight, Folder } from "lucide-react";
import { useAuth } from "../lib/auth";
import { request } from "../lib/api";
import type { Workspace, Project } from "../lib/api";

export default function ProjectTree() {
  const { workspaceId } = useAuth();
  const loc = useLocation();
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [projectsMap, setProjectsMap] = useState<Record<string, Project[]>>({});
  const [open, setOpen] = useState<Record<string, boolean>>({});

  useEffect(() => {
    request<Workspace[]>("/workspaces").then((data) => {
      setWorkspaces(data ?? []);
      const init: Record<string, boolean> = {};
      for (const w of data ?? []) init[w.id] = true;
      setOpen(init);
    }).catch(() => {});
  }, []);

  useEffect(() => {
    for (const w of workspaces) {
      request<Project[]>(`/workspaces/${w.id}/projects`).then((data) => {
        setProjectsMap((prev) => ({ ...prev, [w.id]: data ?? [] }));
      }).catch(() => {});
    }
  }, [workspaces.length]);

  if (workspaces.length === 0) {
    return <div className="dim" style={{ padding: "8px 10px", fontSize: 12 }}>No workspaces</div>;
  }

  return (
    <div className="tree">
      {workspaces.map((w) => {
        const projects = projectsMap[w.id] || [];
        return (
          <div key={w.id}>
            <div
              className="tree__row"
              onClick={() => setOpen((o) => ({ ...o, [w.id]: !o[w.id] }))}
            >
              <span
                className="tree__chevron"
                style={{ transform: open[w.id] ? "rotate(90deg)" : "none", transition: "transform 0.15s" }}
              >
                <ChevronRight size={14} />
              </span>
              <span className="tree__label" style={{ fontWeight: 600 }}>{w.name}</span>
            </div>
            {open[w.id] &&
              projects.map((p) => {
                const to = `/workspaces/${w.id}/projects/${p.id}`;
                const active = loc.pathname === to;
                return (
                  <Link
                    key={p.id}
                    to={to}
                    className={`tree__row ${active ? "active" : ""}`}
                    style={{ paddingLeft: 26 }}
                  >
                    <Folder size={13} />
                    <span className="tree__label">{p.name}</span>
                  </Link>
                );
              })}
          </div>
        );
      })}
    </div>
  );
}
