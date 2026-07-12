import { Link } from "react-router-dom";

export default function NotFound() {
  return (
    <div className="auth-page">
      <div className="auth-card" style={{ textAlign: "center" }}>
        <div style={{ fontSize: 48, fontWeight: 700, color: "var(--gray-200)", marginBottom: 8 }}>404</div>
        <div className="text-strong" style={{ marginBottom: 4 }}>Page not found</div>
        <p className="text-sm text-muted" style={{ marginBottom: 20 }}>The page you're looking for doesn't exist.</p>
        <Link to="/dashboard" className="btn btn--primary btn--sm">Go to Dashboard</Link>
      </div>
    </div>
  );
}
