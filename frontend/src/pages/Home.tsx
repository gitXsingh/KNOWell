import { useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { useAuth } from "../lib/auth";
import { request } from "../lib/api";
import { useToast } from "../lib/toast";

export default function Home() {
  const { session } = useAuth();
  const nav = useNavigate();

  if (session) return <Navigate to="/dashboard" replace />;

  return (
    <div className="auth-page">
      <div className="auth-card" style={{ maxWidth: 420 }}>
        <img src="/logo.png" alt="KNOWell" className="auth-brand-img" />
        <div className="auth-title">Capture &rarr; Review &rarr; Store &rarr; Search</div>
        <p className="text-muted" style={{ textAlign: "center", marginBottom: 24, fontSize: 13, lineHeight: 1.5 }}>
          A developer-first platform that continuously captures project knowledge from day-to-day development and transforms it into structured, searchable documentation.
        </p>

        <div className="stack" style={{ gap: 10 }}>
          <button className="card" style={{ textAlign: "left", width: "100%", padding: 14, cursor: "pointer", borderRadius: "var(--radius-md)" }} onClick={() => nav("/login")}>
            <div className="flex items-center gap-3">
              <div className="avatar avatar--lg" style={{ background: "linear-gradient(135deg, var(--blue-500), #7aa7f5)" }}>P</div>
              <div>
                <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 1 }}>Personal Space</div>
                <div className="text-sm text-muted">Work on your own projects. Sign in or create an account.</div>
              </div>
            </div>
          </button>

          <button className="card card" style={{ textAlign: "left", width: "100%", padding: 14, cursor: "pointer", borderRadius: "var(--radius-md)" }} onClick={() => nav("/team/create")}>
            <div className="flex items-center gap-3">
              <div className="avatar avatar--lg" style={{ background: "linear-gradient(135deg, var(--green-500), #34b85f)" }}>T</div>
              <div>
                <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 1 }}>Create Team</div>
                <div className="text-sm text-muted">Start a team workspace with an invite key to share.</div>
              </div>
            </div>
          </button>

          <button className="card card" style={{ textAlign: "left", width: "100%", padding: 14, cursor: "pointer", borderRadius: "var(--radius-md)" }} onClick={() => nav("/team/join")}>
            <div className="flex items-center gap-3">
              <div className="avatar avatar--lg" style={{ background: "linear-gradient(135deg, var(--amber-500), #f0b84d)" }}>J</div>
              <div>
                <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 1 }}>Join Team</div>
                <div className="text-sm text-muted">Enter an invite key to request access to a team space.</div>
              </div>
            </div>
          </button>
        </div>
      </div>
    </div>
  );
}

export function CreateTeam() {
  const { session } = useAuth();
  const nav = useNavigate();
  const { toast } = useToast();
  if (!session) return <Navigate to="/login?redirect=/team/create" replace />;

  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setBusy(true);
    setErr("");
    try {
      const ws = await request<{ id: string; join_key: string }>("/workspaces", {
        method: "POST",
        body: { name: name.trim(), kind: "team" },
      });
      toast("Team space created", "success");
      nav(`/workspaces/${ws.id}`);
    } catch (e) {
      const msg = (e as Error).message || "Failed to create workspace";
      setErr(msg);
      toast(msg, "error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-card">
        <img src="/logo.png" alt="KNOWell" className="auth-brand-img" />
        <div className="auth-title">Create a Team Space</div>
        <form onSubmit={handleCreate}>
          <div className="field">
            <label className="label">Team Name</label>
            <input className="input" placeholder="My Team" value={name} onChange={(e) => setName(e.target.value)} required autoFocus />
          </div>
          <p className="text-muted text-sm" style={{ marginBottom: 16 }}>
            You will be the admin. An invite key will be generated — share it with your team.
          </p>
          {err && <p style={{ color: "var(--red-500)", fontSize: 12, marginBottom: 12 }}>{err}</p>}
          <button type="submit" className="btn btn--primary btn--block" disabled={busy || !name.trim()}>
            {busy ? "Creating..." : "Create Team Space"}
          </button>
          <div className="auth-foot">
            <button type="button" className="btn btn--ghost btn--sm" onClick={() => nav("/")}>&larr; Back</button>
          </div>
        </form>
      </div>
    </div>
  );
}

export function JoinTeam() {
  const { session } = useAuth();
  const nav = useNavigate();
  const { toast } = useToast();
  if (!session) return <Navigate to="/login?redirect=/team/join" replace />;

  const [key, setKey] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function handleJoin(e: React.FormEvent) {
    e.preventDefault();
    if (!key.trim()) return;
    setBusy(true);
    setErr("");
    try {
      await request<{ id: string }>("/workspaces/join", {
        method: "POST",
        body: { key: key.trim() },
      });
      setKey("");
      toast("Join request sent — wait for admin approval", "success");
    } catch (e) {
      const msg = (e as Error).message || "Failed to join workspace";
      setErr(msg);
      toast(msg, "error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-card">
        <img src="/logo.png" alt="KNOWell" className="auth-brand-img" />
        <div className="auth-title">Join a Team Space</div>
        <form onSubmit={handleJoin}>
          <div className="field">
            <label className="label">Invite Key</label>
            <input className="input" placeholder="TEAM-XXXX-XXXX-XXXX" value={key} onChange={(e) => setKey(e.target.value)} required autoFocus />
          </div>
          {err && <p style={{ color: "var(--red-500)", fontSize: 12, marginBottom: 12 }}>{err}</p>}
          <button type="submit" className="btn btn--primary btn--block" disabled={busy || !key.trim()}>
            {busy ? "Sending..." : "Send Join Request"}
          </button>
          <div className="auth-foot">
            <button type="button" className="btn btn--ghost btn--sm" onClick={() => nav("/")}>&larr; Back</button>
          </div>
        </form>
      </div>
    </div>
  );
}
