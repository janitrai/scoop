import { Link } from "@tanstack/react-router";

interface AppHeaderProps {
  title: string;
  activeTab: "stories" | "stats";
  rightText?: string;
}

export function AppHeader({ title, activeTab, rightText }: AppHeaderProps): JSX.Element {
  return (
    <header className="topbar">
      <div className="topbar-left">
        <span className="topbar-mark" aria-hidden="true">
          S
        </span>
        <div className="topbar-copy">
          <p className="topbar-kicker">Scoop</p>
          <h1 className="topbar-title">{title}</h1>
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

      {rightText ? <p className="topbar-meta">{rightText}</p> : null}
    </header>
  );
}
