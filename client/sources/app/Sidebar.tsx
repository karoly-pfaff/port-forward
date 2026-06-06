import type { ReactElement } from "react";
import { PORTIER_DEFAULT_HOST, PORTIER_DEFAULT_PORT } from "@portier/shared";
import { type AppView, NAV_ITEMS } from "./NavItem.js";

interface SidebarProps {
  open: boolean;
  currentView: AppView;
  onNavClick: (view: AppView) => void;
}

export function Sidebar({ open, currentView, onNavClick }: SidebarProps): ReactElement {
  return (
    <aside className={`sidebar${open ? " sidebar--mobile-open" : ""}`}>
      <nav className="sidebar-nav" aria-label="Main navigation">
        {NAV_ITEMS.map(({ id, label, Icon }) => (
          <button
            key={id}
            type="button"
            className={`nav-item${currentView === id ? " nav-item--active" : ""}`}
            aria-current={currentView === id ? "page" : undefined}
            onClick={() => onNavClick(id)}
          >
            <Icon size={16} aria-hidden="true" />
            {label}
          </button>
        ))}
      </nav>
      <div className="sidebar-footer">
        <span className="sidebar-footer-dot" aria-hidden="true" />
        <span>
          Portier is running
          <span className="sidebar-footer-host">{PORTIER_DEFAULT_HOST}:{PORTIER_DEFAULT_PORT}</span>
        </span>
      </div>
    </aside>
  );
}
