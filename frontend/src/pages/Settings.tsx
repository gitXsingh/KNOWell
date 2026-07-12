import { useAuth } from "../lib/auth";

export default function Settings() {
  const { session, logout } = useAuth();
  const user = session?.user;

  return (
    <div className="page-content" style={{ maxWidth: 600 }}>
      <div className="page-content__header">
        <h1>Settings</h1>
        <p>Manage your account and preferences.</p>
      </div>

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

        <div className="card">
          <h3 style={{ marginBottom: "var(--space-3)" }}>Account</h3>
          <div className="row justify-between" style={{ padding: "var(--space-1) 0" }}>
            <span className="text-muted text-sm">Member since</span>
            <span className="text-sm">{user?.created_at ? new Date(user.created_at).toLocaleDateString() : "—"}</span>
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
    </div>
  );
}
