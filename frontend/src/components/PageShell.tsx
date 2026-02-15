import type { ReactNode } from "react";

import { AppHeader } from "./AppHeader";

interface PageShellProps {
  activeTab: "stories" | "stats";
  variant?: "viewer" | "stats";
  children: ReactNode;
}

export function PageShell({ activeTab, variant = "viewer", children }: PageShellProps): JSX.Element {
  const classNames = ["app-root"];
  if (variant === "viewer") {
    classNames.push("app-root-viewer");
  }
  if (variant === "stats") {
    classNames.push("app-root-viewer", "app-root-stats");
  }

  return (
    <div className={classNames.join(" ")}>
      <AppHeader activeTab={activeTab} />
      {children}
    </div>
  );
}
