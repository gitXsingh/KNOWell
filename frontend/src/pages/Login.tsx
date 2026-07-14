import { useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useAuth } from "../lib/auth";
import { request } from "../lib/api";
import { useToast } from "../lib/toast";

export default function Login() {
  const { refresh } = useAuth();
  const { toast } = useToast();
  const nav = useNavigate();
  const [params] = useSearchParams();
  const redirect = params.get("redirect") || "/dashboard";

  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [fullName, setFullName] = useState("");
  const [workspaceName, setWorkspaceName] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      if (mode === "login") {
        await request("/auth/login", { method: "POST", body: { email, password } });
        toast("Welcome back!", "success");
      } else {
        await request("/auth/register", { method: "POST", body: { email, password, full_name: fullName, workspace_name: workspaceName || undefined } });
        toast("Account created", "success");
      }
      await refresh();
      nav(redirect, { replace: true });
    } catch (e) {
      const msg = (e as Error).message || "Something went wrong";
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
        <p className="text-muted text-sm" style={{ textAlign: "center", marginBottom: 16 }}>
          Capture project knowledge from development into structured documentation.
        </p>
        <div className="flex" style={{ marginBottom: 20, background: "var(--gray-50)", borderRadius: "var(--radius)", padding: 2 }}>
          <button
            className={`btn btn--sm ${mode === "login" ? "btn--primary" : "btn--ghost"}`}
            style={{ flex: 1 }}
            onClick={() => setMode("login")}
          >
            Sign In
          </button>
          <button
            className={`btn btn--sm ${mode === "register" ? "btn--primary" : "btn--ghost"}`}
            style={{ flex: 1 }}
            onClick={() => setMode("register")}
          >
            Create Account
          </button>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="field">
            <label className="label">Email</label>
            <input className="input" type="email" placeholder="you@example.com" value={email} onChange={(e) => setEmail(e.target.value)} required autoFocus />
          </div>
          <div className="field">
            <label className="label">Password</label>
            <input className="input" type="password" placeholder={mode === "register" ? "Min 6 characters" : "Your password"} value={password} onChange={(e) => setPassword(e.target.value)} required minLength={6} />
          </div>

          {mode === "register" && (
            <>
              <div className="field">
                <label className="label">Full Name</label>
                <input className="input" placeholder="Jane Doe" value={fullName} onChange={(e) => setFullName(e.target.value)} required />
              </div>
              <div className="field">
                <label className="label">Workspace (optional)</label>
                <input className="input" placeholder="My Personal Space" value={workspaceName} onChange={(e) => setWorkspaceName(e.target.value)} />
              </div>
            </>
          )}

          {err && <p style={{ color: "var(--red-500)", fontSize: 12, marginBottom: 12 }}>{err}</p>}

          <button type="submit" className="btn btn--primary btn--block btn--lg" disabled={busy}>
            {busy ? "Please wait..." : mode === "login" ? "Sign In" : "Create Account"}
          </button>

          <div className="auth-foot">
            {mode === "login" ? (
              <span>Don't have an account? <button type="button" style={{ color: "var(--blue-500)", fontWeight: 600 }} onClick={() => setMode("register")}>Sign up</button></span>
            ) : (
              <span>Already have an account? <button type="button" style={{ color: "var(--blue-500)", fontWeight: 600 }} onClick={() => setMode("login")}>Sign in</button></span>
            )}
          </div>
          <div className="auth-foot">
            <button type="button" className="btn btn--ghost btn--sm" onClick={() => nav("/")}>&larr; Back</button>
          </div>
        </form>
      </div>
    </div>
  );
}
