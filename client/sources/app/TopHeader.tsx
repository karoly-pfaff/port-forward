import type { ReactElement } from "react";
import { FileCode } from "lucide-react";
import { type AppView } from "./NavItem.js";

interface TopHeaderProps {
  onMenuOpen: () => void;
  onNavClick: (view: AppView) => void;
}

export function TopHeader({ onMenuOpen, onNavClick }: TopHeaderProps): ReactElement {
  return (
    <header className="app-header">
      <div className="app-header-brand">
        <button
          type="button"
          className="mobile-menu-btn"
          aria-label="Open navigation menu"
          onClick={onMenuOpen}
        >
          ☰
        </button>
        <img
          src="/brand/portier-logo-transparent.png"
          alt="Portier logo"
          className="app-header-logo"
        />
        <span className="app-header-title">Portier</span>
      </div>
      <h1 className="app-header-subtitle">TCP/UDP port forwarding for local development</h1>
      <div className="app-header-right">
        <button
          type="button"
          className="api-docs-btn"
          onClick={() => onNavClick("api-docs")}
          title="View API documentation"
        >
          <FileCode size={15} aria-hidden="true" /> API Docs
        </button>
      </div>
    </header>
  );
}
