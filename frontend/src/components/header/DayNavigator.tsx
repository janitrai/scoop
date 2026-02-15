import { ChevronLeft, ChevronRight } from "lucide-react";
import { useState } from "react";

import { Button } from "../ui/button";
import { DayPickerPopover } from "../ui/day-picker-popover";

import type { DayNavigationState } from "../../types";

interface DayNavigatorProps {
  dayNav: DayNavigationState;
  pickerDay: string;
  onMoveOlder: () => void;
  onMoveNewer: () => void;
  onSelectDay: (day: string) => void;
}

export function DayNavigator({
  dayNav,
  pickerDay,
  onMoveOlder,
  onMoveNewer,
  onSelectDay,
}: DayNavigatorProps): JSX.Element {
  const [isDayPickerOpen, setIsDayPickerOpen] = useState(false);

  return (
    <div className="topbar-day">
      <div className="day-nav">
        <Button
          type="button"
          variant="outline"
          size="icon"
          className="day-nav-btn"
          aria-label="Older day"
          onClick={onMoveOlder}
          disabled={!dayNav.canGoOlder}
        >
          <ChevronLeft className="day-nav-icon" aria-hidden="true" />
        </Button>

        <DayPickerPopover
          value={pickerDay}
          onChange={onSelectDay}
          open={isDayPickerOpen}
          onOpenChange={setIsDayPickerOpen}
          align="end"
          sideOffset={8}
          trigger={
            <Button type="button" variant="outline" className="day-current-btn">
              <span className="day-current-line">
                {dayNav.currentLabel} • {dayNav.relativeLabel}
              </span>
            </Button>
          }
        />

        <Button
          type="button"
          variant="outline"
          size="icon"
          className="day-nav-btn"
          aria-label="Newer day"
          onClick={onMoveNewer}
          disabled={!dayNav.canGoNewer}
        >
          <ChevronRight className="day-nav-icon" aria-hidden="true" />
        </Button>
      </div>
    </div>
  );
}
