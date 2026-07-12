import { NavLink } from "react-router-dom";
import {
  LayoutDashboard,
  FolderKanban,
  Search,
  Settings,
  Github,
} from "lucide-react";
import { useAuth } from "../lib/auth";

const nav = [
  { to: "/dashboard", label: "Dashboard", Icon: LayoutDashboard },
  { to: "/workspaces", label: "Workspaces", Icon: FolderKanban },
  { to: "/search", label: "Search", Icon: Search },
  { to: "/github", label: "Integrations", Icon: Github },
  { to: "/settings", label: "Settings", Icon: Settings },
];

export default function Sidebar() {
  const { session } = useAuth();
  const user = session?.user;

  return (
    <aside className="sidebar">
      <div className="sidebar__brand">
        <span className="sidebar__brand-text sidebar__brand-logo">KNOWell</span>
      </div>

      <div className="sidebar__scroll">
        <div className="nav-group">
          {nav.map(({ to, label, Icon }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) => `nav-item ${isActive ? "active" : ""}`}
            >
              <Icon size={16} />
              <span className="nav-item__label">{label}</span>
            </NavLink>
          ))}
        </div>
      </div>

      {user && (
        <div className="sidebar__footer">
          <div className="sidebar-user">
            <div className="avatar avatar--sm">
              {user.full_name.split(" ").map(s => s[0]).join("").slice(0, 2)}
            </div>
            <div className="col" style={{ minWidth: 0, lineHeight: 1.3 }}>
              <div className="sidebar-user__name">{user.full_name}</div>
              <div className="sidebar-user__email">{user.email}</div>
            </div>
          </div>
        </div>
      )}
    </aside>
  );
}
