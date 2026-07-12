import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import type { SessionResponse, WorkspaceSummary } from "./api";
import { request } from "./api";

interface AuthContextValue {
  session: SessionResponse | null;
  ready: boolean;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
  workspaceId: string;
  setWorkspaceId: (id: string) => void;
  workspaces: WorkspaceSummary[];
}

const AuthCtx = createContext<AuthContextValue | null>(null);

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthCtx);
  if (!ctx) throw new Error("useAuth must be inside AuthProvider");
  return ctx;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionResponse | null>(null);
  const [ready, setReady] = useState(false);
  const [workspaceId, setWorkspaceIdState] = useState<string>(
    () => localStorage.getItem("knowell.workspace") || "",
  );

  const setWorkspaceId = (id: string) => {
    setWorkspaceIdState(id);
    if (id) localStorage.setItem("knowell.workspace", id);
    else localStorage.removeItem("knowell.workspace");
  };

  const refresh = async () => {
    try {
      const payload = await request<SessionResponse>("/auth/me");
      setSession(payload);
      if (!workspaceId && payload?.workspaces?.length) {
        setWorkspaceId(payload.workspaces[0].id);
      }
    } catch {
      setSession(null);
    } finally {
      setReady(true);
    }
  };

  useEffect(() => {
    refresh();
  }, []);

  const logout = async () => {
    try {
      await request("/auth/logout", { method: "POST" });
    } catch {}
    setSession(null);
    setWorkspaceId("");
  };

  return (
    <AuthCtx.Provider
      value={{
        session,
        ready,
        refresh,
        logout,
        workspaceId,
        setWorkspaceId,
        workspaces: session?.workspaces ?? [],
      }}
    >
      {children}
    </AuthCtx.Provider>
  );
}
