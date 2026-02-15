import { useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Group, Panel, Separator } from "react-resizable-panels";

import { PageShell } from "./components/PageShell";
import { StoriesListPanel } from "./components/StoriesListPanel";
import { StoryDetailPanel } from "./components/StoryDetailPanel";
import { Button } from "./components/ui/button";
import { Calendar } from "./components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "./components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger } from "./components/ui/select";
import { useViewerQueries } from "./hooks/useViewerQueries";
import { getDesktopFeedWidthBounds, getDesktopFeedWidthPct, setDesktopFeedWidthPct } from "./lib/userSettings";
import { formatCalendarDay, formatCount, formatRelativeDay } from "./lib/viewerFormat";
import type { DayNavigationState, ViewerSearch } from "./types";
import { compactViewerSearch, normalizeViewerSearch, toStoryFilters } from "./viewerSearch";

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

export function StoryViewerPage(): JSX.Element {
  const allCollectionsValue = "__all_collections__";
  const navigate = useNavigate();
  const rawSearch = useSearch({ strict: false });
  const rawParams = useParams({ strict: false }) as { storyUUID?: string };

  const viewerSearch = useMemo(
    () => normalizeViewerSearch(rawSearch as unknown as Record<string, unknown>),
    [rawSearch],
  );

  const filters = useMemo(() => toStoryFilters(viewerSearch), [viewerSearch]);
  const selectedStoryUUID = typeof rawParams.storyUUID === "string" ? rawParams.storyUUID : "";

  const [searchInput, setSearchInput] = useState(filters.query);
  const [isDayPickerOpen, setIsDayPickerOpen] = useState(false);
  const [desktopFeedWidthPct, setDesktopFeedWidthPctState] = useState(() => getDesktopFeedWidthPct());
  const [isDesktopLayout, setIsDesktopLayout] = useState(() => {
    if (typeof window === "undefined") {
      return true;
    }
    return window.matchMedia("(min-width: 1021px)").matches;
  });
  useEffect(() => {
    setSearchInput(filters.query);
  }, [filters.query]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    const mediaQuery = window.matchMedia("(min-width: 1021px)");
    const updateLayout = (): void => {
      setIsDesktopLayout(mediaQuery.matches);
    };

    updateLayout();
    mediaQuery.addEventListener("change", updateLayout);
    return () => mediaQuery.removeEventListener("change", updateLayout);
  }, []);

  const feedWidthBounds = useMemo(() => getDesktopFeedWidthBounds(), []);
  const feedPanelSize = useMemo(() => `${feedWidthBounds.defaultValue}%`, [feedWidthBounds.defaultValue]);
  const feedPanelMin = useMemo(() => `${feedWidthBounds.min}%`, [feedWidthBounds.min]);
  const feedPanelMax = useMemo(() => `${feedWidthBounds.max}%`, [feedWidthBounds.max]);
  const desktopLayout = useMemo(
    () => ({
      storyFeed: desktopFeedWidthPct,
      storyDetail: 100 - desktopFeedWidthPct,
    }),
    [desktopFeedWidthPct],
  );

  function onLayoutChanged(layout: Record<string, number>): void {
    if (!isDesktopLayout) {
      return;
    }

    const nextWidth = layout.storyFeed;
    if (typeof nextWidth !== "number" || !Number.isFinite(nextWidth)) {
      return;
    }

    setDesktopFeedWidthPct(nextWidth);
    setDesktopFeedWidthPctState(getDesktopFeedWidthPct());
  }

  const {
    collections,
    dayBuckets,
    stories,
    detail,
    pagination,
    globalError,
    storiesError,
    detailError,
    isStoriesPending,
    isFetchingNextStoriesPage,
    hasNextStoriesPage,
    fetchNextStoriesPage,
    isDetailPending,
  } = useViewerQueries({
    filters,
    selectedStoryUUID,
  });

  const selectedDay = filters.from && filters.to && filters.from === filters.to ? filters.from : "";

  const dayNav: DayNavigationState = useMemo(() => {
    const customRangeActive = Boolean((filters.from || filters.to) && !selectedDay);
    const navigatorDay = selectedDay || dayBuckets[0]?.day || "";
    const currentIndex = navigatorDay ? dayBuckets.findIndex((bucket) => bucket.day === navigatorDay) : -1;

    const canGoOlder = !customRangeActive && currentIndex >= 0 && currentIndex < dayBuckets.length - 1;
    const canGoNewer = !customRangeActive && currentIndex > 0;

    let currentLabel = "Pick a day";
    let relativeLabel = "No story days yet. Pick a date from the calendar.";

    if (customRangeActive) {
      currentLabel = "Custom range";
      relativeLabel = `From ${filters.from || "start"} to ${filters.to || "now"}`;
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
  }, [dayBuckets, filters.from, filters.to, selectedDay]);

  useEffect(() => {
    if (filters.from || filters.to) {
      return;
    }

    const latestDay = dayBuckets[0]?.day;
    if (!latestDay) {
      return;
    }

    applySearch({
      ...viewerSearch,
      from: latestDay,
      to: latestDay,
      page: undefined,
    });
  }, [dayBuckets, filters.from, filters.to, viewerSearch]);

  const allStoriesCount = useMemo(
    () => collections.reduce((acc, row) => acc + Number(row.stories || 0), 0),
    [collections],
  );
  const allCollectionsLabel = useMemo(
    () => `All collections (${formatCount(allStoriesCount || pagination.total_items)})`,
    [allStoriesCount, pagination.total_items],
  );

  function applySearch(nextSearch: ViewerSearch): void {
    void navigate({
      to: ".",
      search: compactViewerSearch(nextSearch),
      replace: true,
    });
  }

  function goToStory(storyUUID: string): void {
    void navigate({
      to: "/stories/$storyUUID",
      params: { storyUUID },
      search: compactViewerSearch(viewerSearch),
      replace: false,
    });
  }

  useEffect(() => {
    const handle = window.setTimeout(() => {
      const trimmed = searchInput.trim();
      const current = viewerSearch.q || "";
      if (trimmed === current) {
        return;
      }

      applySearch({
        ...viewerSearch,
        q: trimmed || undefined,
        page: undefined,
      });
    }, 220);

    return () => {
      window.clearTimeout(handle);
    };
  }, [searchInput, viewerSearch]);

  function setSingleDayFilter(day: string): void {
    if (!day) {
      return;
    }

    applySearch({
      ...viewerSearch,
      from: day,
      to: day,
      page: undefined,
    });
  }

  function moveDay(offset: number): void {
    if (dayNav.currentIndex < 0) {
      return;
    }

    const nextIndex = dayNav.currentIndex + offset;
    if (nextIndex < 0 || nextIndex >= dayBuckets.length) {
      return;
    }

    const nextDay = dayBuckets[nextIndex]?.day;
    if (!nextDay) {
      return;
    }

    setSingleDayFilter(nextDay);
  }

  function onCollectionChange(collection: string): void {
    applySearch({
      ...viewerSearch,
      collection: collection || undefined,
      page: undefined,
    });
  }

  function onFromChange(value: string): void {
    applySearch({
      ...viewerSearch,
      from: value || undefined,
      page: undefined,
    });
  }

  function onToChange(value: string): void {
    applySearch({
      ...viewerSearch,
      to: value || undefined,
      page: undefined,
    });
  }

  const selectedStoryVisible = selectedStoryUUID
    ? stories.some((story) => story.story_uuid === selectedStoryUUID)
    : true;
  const currentCollectionLabel = useMemo(() => {
    if (!filters.collection) {
      return "All collections";
    }
    const current = collections.find((row) => row.collection === filters.collection);
    return current?.collection || filters.collection;
  }, [collections, filters.collection]);
  const pickerDay = selectedDay || dayNav.navigatorDay;
  const pickerDate = useMemo(() => parseDayString(pickerDay), [pickerDay]);

  const headerLeft = (
    <div className="brand-select">
      <Select
        value={filters.collection || allCollectionsValue}
        onValueChange={(value) => onCollectionChange(value === allCollectionsValue ? "" : value)}
      >
        <SelectTrigger className="brand-select-trigger" aria-label={`Collection filter: ${currentCollectionLabel}`}>
          <div className="brand-select-label">
            <span className="brand-select-prefix">SCOOP</span>
            <span className="brand-select-separator-dot" aria-hidden="true" />
            <span className="brand-select-current">{currentCollectionLabel}</span>
          </div>
        </SelectTrigger>
        <SelectContent className="collection-select-content">
          <SelectItem value={allCollectionsValue}>{allCollectionsLabel}</SelectItem>
          {collections.map((row) => (
            <SelectItem key={row.collection} value={row.collection}>
              {row.collection} ({formatCount(row.stories)})
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );

  const headerRight = (
    <div className="topbar-day">
      <div className="day-nav">
        <Button
          type="button"
          variant="outline"
          size="icon"
          className="day-nav-btn"
          aria-label="Older day"
          onClick={() => moveDay(1)}
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
                setSingleDayFilter(toDayString(value));
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
          onClick={() => moveDay(-1)}
          disabled={!dayNav.canGoNewer}
        >
          <ChevronRight className="day-nav-icon" aria-hidden="true" />
        </Button>
      </div>
    </div>
  );

  return (
    <PageShell variant="viewer" headerLeft={headerLeft} headerRight={headerRight}>
      {globalError ? <p className="banner-error">{globalError}</p> : null}

      <main className="layout">
        <Group
          key={isDesktopLayout ? "desktop-layout" : "mobile-layout"}
          orientation={isDesktopLayout ? "horizontal" : "vertical"}
          className="layout-panels"
          defaultLayout={isDesktopLayout ? desktopLayout : undefined}
          onLayoutChanged={onLayoutChanged}
        >
          <Panel
            id="storyFeed"
            defaultSize={isDesktopLayout ? feedPanelSize : "45%"}
            minSize={isDesktopLayout ? feedPanelMin : "30%"}
            maxSize={isDesktopLayout ? feedPanelMax : "70%"}
          >
            <StoriesListPanel
              searchInput={searchInput}
              from={filters.from}
              to={filters.to}
              totalItems={pagination.total_items}
              loadedItems={stories.length}
              selectedStoryUUID={selectedStoryUUID}
              stories={stories}
              isLoading={isStoriesPending}
              isFetchingNextPage={isFetchingNextStoriesPage}
              hasNextPage={hasNextStoriesPage}
              error={storiesError}
              onSearchInputChange={setSearchInput}
              onFromChange={onFromChange}
              onToChange={onToChange}
              onLoadNextPage={fetchNextStoriesPage}
              onSelectStory={goToStory}
            />
          </Panel>

          <Separator
            className={`layout-resize-handle ${isDesktopLayout ? "horizontal" : "vertical"}`.trim()}
          />

          <Panel id="storyDetail" minSize={isDesktopLayout ? "20%" : "30%"}>
            <StoryDetailPanel
              selectedStoryUUID={selectedStoryUUID}
              selectedStoryVisible={selectedStoryVisible}
              detail={detail}
              isLoading={isDetailPending}
              error={detailError}
            />
          </Panel>
        </Group>
      </main>
    </PageShell>
  );
}
