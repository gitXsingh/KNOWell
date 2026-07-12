import { useEffect, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { request } from "../lib/api";
import type { Workspace, Project } from "../lib/api";

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export default function Topbar() {
  const loc = useLocation();
  const [labels, setLabels] = useState<Record<string, string>>({});

  const parts = loc.pathname.split("/").filter(Boolean);

  useEffect(() => {
    const toFetch: { segment: string; url: string }[] = [];
    for (let i = 0; i < parts.length; i++) {
      const seg = parts[i];
      if (!UUID_RE.test(seg) || labels[seg]) continue;
      const prev = parts[i - 1];
      if (seg && prev === "workspaces") {
        toFetch.push({ segment: seg, url: `/workspaces/${seg}` });
      }
      if (seg && prev === "projects") {
        const wsId = parts[i - 2];
        if (wsId && UUID_RE.test(wsId)) {
          toFetch.push({ segment: seg, url: `/workspaces/${wsId}/projects/${seg}` });
        }
      }
    }
    if (toFetch.length > 0) {
      Promise.all(
        toFetch.map(({ segment, url }) =>
          request<Workspace | Project>(url)
            .then((d) => ({ segment, name: (d as any).name || segment }))
            .catch(() => ({ segment, name: segment }))
        )
      ).then((results) => {
        const updates: Record<string, string> = {};
        for (const r of results) updates[r.segment] = r.name;
        setLabels((prev) => ({ ...prev, ...updates }));
      });
    }
  }, [loc.pathname]);

  const buildBreadcrumb = () => {
    const crumbs: { label: string; to?: string }[] = [{ label: "Home", to: "/dashboard" }];
    for (let i = 0; i < parts.length; i++) {
      const seg = parts[i];
      if (seg === "dashboard") continue;
      if (UUID_RE.test(seg) && parts[i - 1] === "projects") continue;
      const label = UUID_RE.test(seg) ? (labels[seg] || "...") : decodeURIComponent(seg);
      const to = "/" + parts.slice(0, i + 1).join("/");
      const isLast = i === parts.length - 1;
      crumbs.push({ label, to: isLast ? undefined : to });
    }
    return crumbs;
  };

  const crumbs = buildBreadcrumb();

  return (
    <div className="topbar">
      <div className="topbar__left">
        <div className="breadcrumb">
          {crumbs.map((c, i) => (
            <span key={i} className="row gap-1">
              {i > 0 && <span className="breadcrumb__sep">/</span>}
              {c.to ? (
                <Link to={c.to} className="breadcrumb__seg">{c.label}</Link>
              ) : (
                <span className="breadcrumb__seg">{c.label}</span>
              )}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}
