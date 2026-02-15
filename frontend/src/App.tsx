import { useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Group, Panel, Separator } from "react-resizable-panels";

import { PageShell } from "./components/PageShell";
import { StoriesListPanel } from "./components/StoriesListPanel";
import { StoryDetailPanel } from "./components/StoryDetailPanel";
import { useViewerQueries } from "./hooks/useViewerQueries";
import { getDesktopFeedWidthBounds, getDesktopFeedWidthPct, setDesktopFeedWidthPct } from "./lib/userSettings";
import { formatCalendarDay, formatCount, formatRelativeDay } from "./lib/viewerFormat";
import type { DayNavigationState, ViewerSearch } from "./types";
import { compactViewerSearch, normalizeViewerSearch, toStoryFilters } from "./viewerSearch";

export function StoryViewerPage(): JSX.Element {
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
  const [desktopFeedWidthPct, setDesktopFeedWidthPctState] = useState(() => getDesktopFeedWidthPct());
  const [isDesktopLayout, setIsDesktopLayout] = useState(() => {
    if (typeof window === "undefined") {
      return true;
    }
    return window.matchMedia("(min-width: 1021px)").matches;
  });
  const dayPickerRef = useRef<HTMLInputElement | null>(null);

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

  function openDayPicker(): void {
    const picker = dayPickerRef.current;
    if (!picker) {
      return;
    }

    const anchorDay = selectedDay || dayNav.navigatorDay || new Date().toISOString().slice(0, 10);
    picker.value = anchorDay;

    const pickerWithShow = picker as HTMLInputElement & { showPicker?: () => void };
    if (typeof pickerWithShow.showPicker === "function") {
      pickerWithShow.showPicker();
      return;
    }

    picker.focus();
    picker.click();
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

  const headerRight = (
    <div className="topbar-controls">
      <label className="collection-picker" aria-label="Collection filter">
        <span className="sr-only">Collection</span>
        <select
          className="collection-picker-select"
          value={filters.collection}
          onChange={(event) => onCollectionChange(event.target.value)}
        >
          <option value="">{allCollectionsLabel}</option>
          {collections.map((row) => (
            <option key={row.collection} value={row.collection}>
              {row.collection} ({formatCount(row.stories)})
            </option>
          ))}
        </select>
        <span className="collection-picker-caret" aria-hidden="true">
          ▾
        </span>
      </label>

      <div className="topbar-day">
        <div className="day-nav">
          <button
            type="button"
            className="btn btn-subtle day-nav-btn"
            aria-label="Older day"
            onClick={() => moveDay(1)}
            disabled={!dayNav.canGoOlder}
          >
            <ChevronLeft className="day-nav-icon" aria-hidden="true" />
          </button>

          <button type="button" className="day-current-btn" onClick={openDayPicker}>
            <span className="day-current-line">
              {dayNav.currentLabel} • {dayNav.relativeLabel}
            </span>
          </button>

          <button
            type="button"
            className="btn btn-subtle day-nav-btn"
            aria-label="Newer day"
            onClick={() => moveDay(-1)}
            disabled={!dayNav.canGoNewer}
          >
            <ChevronRight className="day-nav-icon" aria-hidden="true" />
          </button>

          <input
            ref={(node) => {
              dayPickerRef.current = node;
            }}
            className="day-picker-input"
            type="date"
            aria-hidden="true"
            tabIndex={-1}
            onChange={(event) => {
              if (!event.target.value) {
                return;
              }
              setSingleDayFilter(event.target.value);
            }}
          />
        </div>
      </div>
    </div>
  );

  return (
    <PageShell variant="viewer" headerRight={headerRight}>
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
