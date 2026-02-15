import type { ReactNode } from "react";

import { AppHeader } from "./AppHeader";

interface PageShellProps {
  variant?: "viewer" | "stats";
  headerRight?: ReactNode;
  children: ReactNode;
}

export function PageShell({ variant = "viewer", headerRight, children }: PageShellProps): JSX.Element {
  const classNames = ["app-root"];
  if (variant === "viewer") {
    classNames.push("app-root-viewer");
  }
  if (variant === "stats") {
    classNames.push("app-root-viewer", "app-root-stats");
  }

  return (
    <div className={classNames.join(" ")}>
      <AppHeader rightSlot={headerRight} />
      {children}
    </div>
  );
}
