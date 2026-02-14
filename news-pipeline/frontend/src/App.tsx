import { useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { useEffect, useMemo, useRef, useState } from "react";

import { AppHeader } from "./components/AppHeader";
import { FiltersPanel } from "./components/FiltersPanel";
import { StoriesListPanel } from "./components/StoriesListPanel";
import { StoryDetailPanel } from "./components/StoryDetailPanel";
import { useViewerQueries } from "./hooks/useViewerQueries";
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
  const [refreshTick, setRefreshTick] = useState(0);
  const dayPickerRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    setSearchInput(filters.query);
  }, [filters.query]);

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
    isDetailPending,
  } = useViewerQueries({
    filters,
    selectedStoryUUID,
    refreshTick,
  });

  const selectedDay = filters.from && filters.to && filters.from === filters.to ? filters.from : "";

  const dayNav: DayNavigationState = useMemo(() => {
    const customRangeActive = Boolean((filters.from || filters.to) && !selectedDay);
    const navigatorDay = selectedDay || dayBuckets[0]?.day || "";
    const currentIndex = navigatorDay ? dayBuckets.findIndex((bucket) => bucket.day === navigatorDay) : -1;
    const bucket = navigatorDay ? dayBuckets.find((row) => row.day === navigatorDay) : undefined;

    const canGoOlder = !customRangeActive && currentIndex >= 0 && currentIndex < dayBuckets.length - 1;
    const canGoNewer = !customRangeActive && currentIndex > 0;
    const storyCount = Number(bucket?.story_count ?? 0);

    let currentLabel = "Pick a day";
    let relativeLabel = "No story days yet. Pick a date from the calendar.";

    if (customRangeActive) {
      currentLabel = "Custom range";
      relativeLabel = `From ${filters.from || "start"} to ${filters.to || "now"}`;
    } else if (navigatorDay) {
      currentLabel = formatCalendarDay(navigatorDay);
      if (selectedDay) {
        relativeLabel = `${formatRelativeDay(selectedDay)} • ${formatCount(storyCount)} stories`;
      } else {
        relativeLabel = `Showing all days • latest is ${formatRelativeDay(navigatorDay)} • ${formatCount(storyCount)} stories`;
      }
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

  const totalPages = Math.max(1, pagination.total_pages);
  const allStoriesCount = useMemo(
    () => collections.reduce((acc, row) => acc + Number(row.stories || 0), 0),
    [collections],
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
        page: 1,
      });
    }, 220);

    return () => {
      window.clearTimeout(handle);
    };
  }, [searchInput, viewerSearch]);

  function setSingleDayFilter(day: string): void {
    if (!day) {
      applySearch({
        ...viewerSearch,
        from: undefined,
        to: undefined,
        page: 1,
      });
      return;
    }

    applySearch({
      ...viewerSearch,
      from: day,
      to: day,
      page: 1,
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

  function onRefresh(): void {
    setRefreshTick((tick) => tick + 1);
  }

  function onCollectionChange(collection: string): void {
    applySearch({
      ...viewerSearch,
      collection: collection || undefined,
      page: 1,
    });
  }

  function onFromChange(value: string): void {
    applySearch({
      ...viewerSearch,
      from: value || undefined,
      page: 1,
    });
  }

  function onToChange(value: string): void {
    applySearch({
      ...viewerSearch,
      to: value || undefined,
      page: 1,
    });
  }

  function onPrevPage(): void {
    if (filters.page <= 1) {
      return;
    }

    applySearch({
      ...viewerSearch,
      page: filters.page - 1,
    });
  }

  function onNextPage(): void {
    if (filters.page >= totalPages) {
      return;
    }

    applySearch({
      ...viewerSearch,
      page: filters.page + 1,
    });
  }

  const selectedStoryVisible = selectedStoryUUID
    ? stories.some((story) => story.story_uuid === selectedStoryUUID)
    : true;

  return (
    <div className="app-root">
      <AppHeader title="Story Viewer" activeTab="stories" />

      <FiltersPanel
        searchInput={searchInput}
        from={filters.from}
        to={filters.to}
        activeCollection={filters.collection}
        allStoriesCount={allStoriesCount}
        totalItems={pagination.total_items}
        hasDateFilter={Boolean(filters.from || filters.to)}
        collections={collections}
        dayNav={dayNav}
        dayPickerRef={dayPickerRef}
        onSearchInputChange={setSearchInput}
        onFromChange={onFromChange}
        onToChange={onToChange}
        onRefresh={onRefresh}
        onCollectionChange={onCollectionChange}
        onMoveOlderDay={() => moveDay(1)}
        onMoveNewerDay={() => moveDay(-1)}
        onOpenDayPicker={openDayPicker}
        onClearDays={() => setSingleDayFilter("")}
        onDayPick={setSingleDayFilter}
      />

      {globalError ? <p className="banner-error">{globalError}</p> : null}

      <main className="layout">
        <StoriesListPanel
          page={filters.page}
          totalPages={totalPages}
          totalItems={pagination.total_items}
          selectedStoryUUID={selectedStoryUUID}
          stories={stories}
          isLoading={isStoriesPending}
          error={storiesError}
          onPrevPage={onPrevPage}
          onNextPage={onNextPage}
          onSelectStory={goToStory}
        />

        <StoryDetailPanel
          selectedStoryUUID={selectedStoryUUID}
          selectedStoryVisible={selectedStoryVisible}
          detail={detail}
          isLoading={isDetailPending}
          error={detailError}
        />
      </main>
    </div>
  );
}
