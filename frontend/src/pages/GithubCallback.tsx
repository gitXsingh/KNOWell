import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { CheckCircle, XCircle, Loader } from "lucide-react";

export default function GithubCallback() {
  const [params] = useSearchParams();
  const nav = useNavigate();
  const status = params.get("status");
  const message = params.get("message");

  useEffect(() => {
    if (!status) return;
    const delay = status === "success" ? 1500 : 4000;
    const timer = setTimeout(() => nav("/github"), delay);
    return () => clearTimeout(timer);
  }, [status, nav]);

  if (!status) {
    return (
      <div className="auth-page">
        <div className="auth-card" style={{ textAlign: "center" }}>
          <Loader size={24} style={{ marginBottom: 12, color: "var(--gray-400)" }} />
          <p className="text-muted">Completing GitHub connection...</p>
        </div>
      </div>
    );
  }

  if (status === "success") {
    return (
      <div className="auth-page">
        <div className="auth-card" style={{ textAlign: "center" }}>
          <CheckCircle size={28} style={{ marginBottom: 12, color: "var(--green-500)" }} />
          <h2 style={{ marginBottom: 4 }}>Connected</h2>
          <p className="text-muted text-sm">GitHub account linked. Redirecting...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="auth-page">
      <div className="auth-card" style={{ textAlign: "center" }}>
        <XCircle size={28} style={{ marginBottom: 12, color: "var(--red-500)" }} />
        <h2 style={{ marginBottom: 4 }}>Connection Failed</h2>
        <p className="text-muted text-sm">{message || "GitHub authorization was not completed."}</p>
        <p className="text-muted text-xs" style={{ marginTop: 12 }}>Redirecting back...</p>
      </div>
    </div>
  );
}
