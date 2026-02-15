import type { ReactNode } from "react";

interface AppHeaderProps {
  rightSlot?: ReactNode;
}

export function AppHeader({ rightSlot }: AppHeaderProps): JSX.Element {
  return (
    <header className="topbar">
      <div className="topbar-left">
        <div className="topbar-copy">
          <h1 className="topbar-title">SCOOP</h1>
        </div>
      </div>

      {rightSlot ? <div className="topbar-right">{rightSlot}</div> : null}
    </header>
  );
}
