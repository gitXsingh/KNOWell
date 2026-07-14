import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../lib/auth";
import { Sun, Moon, Monitor, ExternalLink, Palette, Github as GhIcon } from "lucide-react";

const KNOWLEDGE_SOURCES = [
  { key: "github_repository", label: "GitHub Repository", available: true },
  { key: "manual_notes", label: "Manual Notes", available: false },
  { key: "architecture_decisions", label: "Architecture Decisions", available: false },
  { key: "api_documentation", label: "API Documentation", available: false },
  { key: "meeting_notes", label: "Meeting Notes", available: false },
  { key: "deployment_notes", label: "Deployment Notes", available: false },
  { key: "database_schema", label: "Database Schema", available: false },
];

const TABS = [
  { id: "profile", label: "Profile" },
  { id: "sources", label: "Knowledge Sources" },
  { id: "integrations", label: "Integrations" },
  { id: "appearance", label: "Appearance" },
];

export default function Settings() {
  const { session, logout } = useAuth();
  const nav = useNavigate();
  const user = session?.user;
  const [tab, setTab] = useState("profile");

  return (
    <div className="page-content" style={{ maxWidth: 700 }}>
      <div className="page-content__header">
        <h1>Settings</h1>
      </div>

      <div className="tabs" style={{ marginBottom: "var(--space-6)" }}>
        {TABS.map((t) => (
          <button key={t.id} className={`tab ${tab === t.id ? "active" : ""}`} onClick={() => setTab(t.id)}>
            {t.label}
          </button>
        ))}
      </div>

      {tab === "profile" && (
        <div className="col gap-4">
          <div className="card">
            <div className="row gap-4" style={{ marginBottom: "var(--space-5)" }}>
              <div className="avatar" style={{ width: 40, height: 40, fontSize: 14 }}>
                {user?.full_name?.split(" ").map(s => s[0]).join("").slice(0, 2) || "?"}
              </div>
              <div>
                <div className="proj-card__title" style={{ fontSize: 15 }}>{user?.full_name}</div>
                <div className="text-dim text-sm">{user?.email}</div>
              </div>
            </div>
            <div className="field">
              <label className="label">Full Name</label>
              <input className="input" value={user?.full_name || ""} readOnly />
            </div>
            <div className="field">
              <label className="label">Email</label>
              <input className="input" value={user?.email || ""} readOnly />
            </div>
          </div>

          <div className="card card--compact">
            <div className="row justify-between">
              <div className="col">
                <span className="text-strong text-sm">Sign out</span>
                <span className="text-dim text-xs">End your current session</span>
              </div>
              <button className="btn btn--outline btn--sm" onClick={logout}>Sign Out</button>
            </div>
          </div>
        </div>
      )}

      {tab === "sources" && (
        <div className="card">
          <div className="row gap-2" style={{ marginBottom: "var(--space-3)" }}>
            <ExternalLink size={14} />
            <h3>Knowledge Sources</h3>
          </div>
          <div className="col">
            {KNOWLEDGE_SOURCES.map((s) => (
              <div key={s.key} className="knowledge-source-row">
                <div className="row gap-2">
                  <span className="text-sm text-strong">{s.label}</span>
                  {!s.available && <span className="badge-coming-soon">Coming Soon</span>}
                </div>
                {s.available ? (
                  <span className="pill pill--success">Available</span>
                ) : (
                  <span className="pill pill--default" style={{ opacity: 0.5 }}>Unavailable</span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {tab === "integrations" && (
        <div className="card">
          <div className="row gap-2" style={{ marginBottom: "var(--space-3)" }}>
            <GhIcon size={14} />
            <h3>Integrations</h3>
          </div>
          <div className="knowledge-source-row">
            <div className="row gap-2">
              <GhIcon size={15} />
              <span className="text-sm text-strong">GitHub</span>
            </div>
            <button className="btn btn--primary btn--sm" onClick={() => nav("/github")}>
              Manage
            </button>
          </div>
        </div>
      )}

      {tab === "appearance" && (
        <div className="card">
          <div className="row gap-2" style={{ marginBottom: "var(--space-3)" }}>
            <Palette size={14} />
            <h3>Appearance</h3>
          </div>
          <div className="col gap-2">
            <div className="appearance-option appearance-option--active">
              <div className="row gap-2">
                <Sun size={15} />
                <span className="text-sm text-strong">Light</span>
              </div>
              <span className="pill pill--success">Enabled</span>
            </div>
            <div className="appearance-option" style={{ opacity: 0.5 }}>
              <div className="row gap-2">
                <Moon size={15} />
                <span className="text-sm">Dark</span>
              </div>
              <span className="badge-coming-soon">Coming Soon</span>
            </div>
            <div className="appearance-option" style={{ opacity: 0.5 }}>
              <div className="row gap-2">
                <Monitor size={15} />
                <span className="text-sm">System</span>
              </div>
              <span className="badge-coming-soon">Coming Soon</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
