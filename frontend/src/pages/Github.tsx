import { useEffect, useState } from "react";
import { Github as GhIcon } from "lucide-react";
import { request } from "../lib/api";
import type { GitHubStatus } from "../lib/api";
import { useToast } from "../lib/toast";

export default function Github() {
  const { toast } = useToast();
  const [gh, setGh] = useState<GitHubStatus | null>(null);
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    try {
      setGh(await request<GitHubStatus>("/github/account"));
    } catch { setGh(null); }
    finally { setLoading(false); }
  }

  useEffect(() => { load(); }, []);

  async function connect() {
    try {
      const res = await request<{ authorization_url: string }>("/github/connect");
      if (res?.authorization_url) window.location.href = res.authorization_url;
    } catch (e) { toast((e as Error).message || "Failed to start GitHub OAuth", "error"); }
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
        <h1>GitHub Integration</h1>
        <p>Connect repositories and receive webhook events.</p>
      </div>

      <div className="card">
        <h3 style={{ marginBottom: "var(--space-3)" }}>GitHub Account</h3>
        {loading ? (
          <div className="row gap-3">
            <div className="skeleton" style={{ width: 30, height: 30, borderRadius: "50%" }} />
            <div className="flex-1">
              <div className="skeleton" style={{ height: 14, width: "30%", marginBottom: 6 }} />
              <div className="skeleton" style={{ height: 10, width: "50%" }} />
            </div>
          </div>
        ) : gh?.connected ? (
          <div className="row justify-between">
            <div className="row gap-3">
              <div style={{ width: 30, height: 30, borderRadius: "50%", background: "var(--gray-50)", display: "grid", placeItems: "center" }}>
                <GhIcon size={16} />
              </div>
              <div>
                <div className="text-strong">Connected</div>
                <div className="text-dim text-xs">Scopes: {(gh.token_scopes || []).join(", ") || "public"}</div>
              </div>
            </div>
            <button className="btn btn--outline btn--sm" style={{ color: "var(--red-500)" }} onClick={disconnect}>Disconnect</button>
          </div>
        ) : (
          <div className="row justify-between">
            <div className="row gap-3">
              <div style={{ width: 30, height: 30, borderRadius: "50%", background: "var(--gray-50)", display: "grid", placeItems: "center" }}>
                <GhIcon size={16} />
              </div>
              <div>
                <div className="text-strong">Not connected</div>
                <div className="text-dim text-xs">Link GitHub to connect repositories.</div>
              </div>
            </div>
            <button className="btn btn--primary btn--sm" onClick={connect}>Connect GitHub</button>
          </div>
        )}
      </div>
    </div>
  );
}
