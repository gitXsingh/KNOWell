import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

export default function GithubCallback() {
  const [params] = useSearchParams();
  const nav = useNavigate();
  const [msg, setMsg] = useState("Completing GitHub connection...");

  useEffect(() => {
    const status = params.get("status");
    const message = params.get("message");

    if (status === "success") {
      setMsg("Connected! Redirecting...");
      setTimeout(() => nav("/settings"), 800);
    } else {
      setMsg(message || "GitHub connection failed.");
      setTimeout(() => nav("/settings"), 2000);
    }
  }, [params, nav]);

  return (
    <div className="auth">
      <div className="auth__card" style={{ textAlign: "center" }}>
        <h2>GitHub</h2>
        <p className="muted" style={{ marginTop: 12 }}>{msg}</p>
      </div>
    </div>
  );
}
