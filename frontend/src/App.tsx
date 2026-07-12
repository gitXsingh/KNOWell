import { Navigate, Route, Routes } from "react-router-dom";
import { useAuth } from "./lib/auth";
import AppShell from "./components/AppShell";
import Home, { CreateTeam, JoinTeam } from "./pages/Home";
import Login from "./pages/Login";
import Dashboard from "./pages/Dashboard";
import Workspaces from "./pages/Workspaces";
import WorkspaceDetail from "./pages/WorkspaceDetail";
import ProjectDetail from "./pages/ProjectDetail";
import Search from "./pages/Search";
import Settings from "./pages/Settings";
import Github from "./pages/Github";
import GithubCallback from "./pages/GithubCallback";
import NotFound from "./pages/NotFound";

function Protected({ children }: { children: React.ReactNode }) {
  const { session, ready } = useAuth();
  if (!ready) {
    return (
      <div className="auth-page">
        <div className="auth-card" style={{ textAlign: "center" }}>
          <div className="skeleton" style={{ width: 160, height: 18, margin: "0 auto 12px" }} />
          <div className="skeleton" style={{ width: 100, height: 12, margin: "0 auto" }} />
        </div>
      </div>
    );
  }
  if (!session) return <Navigate to="/" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Home />} />
      <Route path="/login" element={<Login />} />
      <Route path="/team/create" element={<CreateTeam />} />
      <Route path="/team/join" element={<JoinTeam />} />
      <Route path="/github/callback" element={<GithubCallback />} />
      <Route element={<Protected><AppShell /></Protected>}>
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/workspaces" element={<Workspaces />} />
        <Route path="/workspaces/:wid" element={<WorkspaceDetail />} />
        <Route path="/workspaces/:wid/projects/:pid" element={<ProjectDetail />} />
        <Route path="/workspaces/:wid/projects/:pid/:tab" element={<ProjectDetail />} />
        <Route path="/search" element={<Search />} />
        <Route path="/settings" element={<Settings />} />
        <Route path="/github" element={<Github />} />
      </Route>
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
