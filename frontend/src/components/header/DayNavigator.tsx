import { ChevronLeft, ChevronRight } from "lucide-react";
import { useMemo, useState } from "react";

import { Button } from "../ui/button";
import { Calendar } from "../ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "../ui/popover";

import type { DayNavigationState } from "../../types";

interface DayNavigatorProps {
  dayNav: DayNavigationState;
  pickerDay: string;
  onMoveOlder: () => void;
  onMoveNewer: () => void;
  onSelectDay: (day: string) => void;
}

function parseDayString(value: string): Date | undefined {
  if (!value) {
    return undefined;
  }

  const [yearText, monthText, dayText] = value.split("-");
  const year = Number(yearText);
  const month = Number(monthText);
  const day = Number(dayText);
  if (!Number.isFinite(year) || !Number.isFinite(month) || !Number.isFinite(day)) {
    return undefined;
  }

  return new Date(year, month - 1, day);
}

function toDayString(value: Date): string {
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, "0");
  const day = String(value.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function DayNavigator({
  dayNav,
  pickerDay,
  onMoveOlder,
  onMoveNewer,
  onSelectDay,
}: DayNavigatorProps): JSX.Element {
  const [isDayPickerOpen, setIsDayPickerOpen] = useState(false);
  const pickerDate = useMemo(() => parseDayString(pickerDay), [pickerDay]);

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

        <Popover open={isDayPickerOpen} onOpenChange={setIsDayPickerOpen}>
          <PopoverTrigger asChild>
            <Button type="button" variant="outline" className="day-current-btn">
              <span className="day-current-line">
                {dayNav.currentLabel} • {dayNav.relativeLabel}
              </span>
            </Button>
          </PopoverTrigger>
          <PopoverContent className="day-popover" align="end" sideOffset={8}>
            <Calendar
              key={pickerDay || "no-day"}
              mode="single"
              selected={pickerDate}
              defaultMonth={pickerDate}
              onSelect={(value) => {
                if (!value) {
                  return;
                }
                onSelectDay(toDayString(value));
                setIsDayPickerOpen(false);
              }}
              initialFocus
            />
          </PopoverContent>
        </Popover>

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

