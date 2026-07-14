import { useEffect, useState } from "react";
import { Github as GhIcon } from "lucide-react";
import { request } from "../lib/api";
import type { GitHubStatus } from "../lib/api";
import { useToast } from "../lib/toast";

export default function Github() {
  const { toast } = useToast();
  const [gh, setGh] = useState<GitHubStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [connecting, setConnecting] = useState(false);
  const [err, setErr] = useState("");

  async function load() {
    setLoading(true);
    setErr("");
    try {
      setGh(await request<GitHubStatus>("/github/account"));
    } catch (e) {
      setGh(null);
      setErr((e as Error).message || "Failed to check GitHub connection");
    }
    finally { setLoading(false); }
  }

  useEffect(() => { load(); }, []);

  async function connect() {
    setConnecting(true);
    setErr("");
    try {
      const res = await request<{ authorization_url: string }>("/github/connect");
      if (res?.authorization_url) {
        window.location.href = res.authorization_url;
      } else {
        setErr("GitHub did not return an authorization URL");
        setConnecting(false);
      }
    } catch (e) {
      const msg = (e as Error).message || "Failed to start GitHub OAuth";
      setErr(msg);
      toast(msg, "error");
      setConnecting(false);
    }
  }

  async function disconnect() {
    try {
      await request("/github/account", { method: "DELETE" });
      toast("GitHub account disconnected", "info");
      await load();
    } catch (e) { toast((e as Error).message || "Failed to disconnect", "error"); }
  }

  return (
    <div className="page-content" style={{ maxWidth: 600 }}>
      <div className="page-content__header">
        <h1>Integrations</h1>
        <p>Connect services to capture knowledge from your development workflow.</p>
      </div>

      <div className="card">
        <h3 style={{ marginBottom: "var(--space-3)" }}>GitHub</h3>
        {loading ? (
          <div className="row gap-3">
            <div className="skeleton" style={{ width: 30, height: 30, borderRadius: "50%" }} />
            <div className="flex-1">
              <div className="skeleton" style={{ height: 14, width: "30%", marginBottom: 6 }} />
              <div className="skeleton" style={{ height: 10, width: "50%" }} />
            </div>
          </div>
        ) : err ? (
          <div className="col gap-3">
            <div className="row gap-2" style={{ color: "var(--red-500)" }}>
              <span className="text-sm">{err}</span>
            </div>
            <button className="btn btn--ghost btn--sm" style={{ alignSelf: "flex-start" }} onClick={load}>
              Retry
            </button>
          </div>
        ) : gh?.connected ? (
          <div className="row justify-between">
            <div className="row gap-3">
              <div style={{ width: 30, height: 30, borderRadius: "50%", background: "var(--gray-50)", display: "grid", placeItems: "center" }}>
                <GhIcon size={16} />
              </div>
              <div>
                <div className="text-strong">Connected</div>
                <div className="text-dim text-xs">GitHub account linked</div>
              </div>
            </div>
            <button className="btn btn--outline btn--sm" style={{ color: "var(--red-500)" }} onClick={disconnect}>Disconnect</button>
          </div>
        ) : connecting ? (
          <div className="row gap-3">
            <div style={{ width: 30, height: 30, borderRadius: "50%", background: "var(--gray-50)", display: "grid", placeItems: "center" }}>
              <GhIcon size={16} />
            </div>
            <div>
              <div className="text-strong">Connecting...</div>
              <div className="text-dim text-xs">Redirecting to GitHub for authorization</div>
            </div>
          </div>
        ) : (
          <div className="row justify-between">
            <div className="row gap-3">
              <div style={{ width: 30, height: 30, borderRadius: "50%", background: "var(--gray-50)", display: "grid", placeItems: "center" }}>
                <GhIcon size={16} />
              </div>
              <div>
                <div className="text-strong">Not connected</div>
                <div className="text-dim text-xs">Authorize GitHub to connect repositories and capture knowledge from development.</div>
              </div>
            </div>
            <button className="btn btn--primary btn--sm" onClick={connect}>Connect GitHub</button>
          </div>
        )}
      </div>
    </div>
  );
}
