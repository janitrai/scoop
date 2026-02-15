import { Link } from "@tanstack/react-router";

interface AppHeaderProps {
  activeTab: "stories" | "stats";
}

export function AppHeader({ activeTab }: AppHeaderProps): JSX.Element {
  return (
    <header className="topbar">
      <div className="topbar-left">
        <span className="topbar-mark" aria-hidden="true">
          S
        </span>
        <div className="topbar-copy">
          <h1 className="topbar-title">SCOOP</h1>
        </div>
      </div>

      <nav className="top-tabs" aria-label="Sections">
        <Link to="/stories" className={`top-tab ${activeTab === "stories" ? "active" : ""}`.trim()}>
          Stories
        </Link>
        <Link to="/stats" className={`top-tab ${activeTab === "stats" ? "active" : ""}`.trim()}>
          Stats
        </Link>
      </nav>
    </header>
  );
}
