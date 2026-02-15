import { useMemo } from "react";

import { formatCalendarDay, formatRelativeDay } from "../lib/viewerFormat";
import type { DayNavigationState, StoryDayBucket } from "../types";

interface UseDayNavigationStateArgs {
  dayBuckets: StoryDayBucket[];
  day: string;
  from: string;
  to: string;
}

interface UseDayNavigationStateResult {
  dayNav: DayNavigationState;
  selectedDay: string;
}

export function useDayNavigationState({
  dayBuckets,
  day,
  from,
  to,
}: UseDayNavigationStateArgs): UseDayNavigationStateResult {
  const rangeDay = from && to && from === to ? from : "";
  const selectedDay = day || rangeDay;

  const dayNav = useMemo<DayNavigationState>(() => {
    const customRangeActive = Boolean((from || to) && !rangeDay);
    const navigatorDay = selectedDay || dayBuckets[0]?.day || "";
    const currentIndex = navigatorDay ? dayBuckets.findIndex((bucket) => bucket.day === navigatorDay) : -1;

    const canGoOlder = !customRangeActive && currentIndex >= 0 && currentIndex < dayBuckets.length - 1;
    const canGoNewer = !customRangeActive && currentIndex > 0;

    let currentLabel = "Pick a day";
    let relativeLabel = "No story days yet. Pick a date from the calendar.";

    if (customRangeActive) {
      currentLabel = "Custom range";
      relativeLabel = `From ${from || "start"} to ${to || "now"}`;
    } else if (navigatorDay) {
      currentLabel = formatCalendarDay(navigatorDay);
      relativeLabel = formatRelativeDay(navigatorDay);
    }

    return {
      currentIndex,
      canGoOlder,
      canGoNewer,
      currentLabel,
      navigatorDay,
      relativeLabel,
    };
  }, [dayBuckets, from, rangeDay, selectedDay, to]);

  return { dayNav, selectedDay };
}
