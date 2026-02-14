import { Link } from "@tanstack/react-router";

interface AppHeaderProps {
  title: string;
  activeTab: "stories" | "stats";
  rightText?: string;
}

export function AppHeader({ title, activeTab, rightText }: AppHeaderProps): JSX.Element {
  return (
    <header className="topbar">
      <div className="brand">
        <p className="eyebrow">News Pipeline</p>
        <h1>{title}</h1>
        <nav className="top-tabs" aria-label="Sections">
          <Link to="/stories" className={`top-tab ${activeTab === "stories" ? "active" : ""}`.trim()}>
            Stories
          </Link>
          <Link to="/stats" className={`top-tab ${activeTab === "stats" ? "active" : ""}`.trim()}>
            Stats
          </Link>
        </nav>
      </div>

      {rightText ? <div className="stat-inline">{rightText}</div> : null}
    </header>
  );
}
