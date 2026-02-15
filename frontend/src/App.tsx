import { useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { Group, Panel, Separator } from "react-resizable-panels";

import { CollectionDropdown } from "./components/header/CollectionDropdown";
import { DayNavigator } from "./components/header/DayNavigator";
import { PageShell } from "./components/PageShell";
import { StoriesListPanel } from "./components/StoriesListPanel";
import { StoryDetailPanel } from "./components/StoryDetailPanel";
import { useCurrentCollectionLabel } from "./hooks/useCurrentCollectionLabel";
import { useDayNavigationState } from "./hooks/useDayNavigationState";
import { useViewerQueries } from "./hooks/useViewerQueries";
import { getDesktopFeedWidthBounds, getDesktopFeedWidthPct, setDesktopFeedWidthPct } from "./lib/userSettings";
import { formatCount } from "./lib/viewerFormat";
import type { ViewerSearch } from "./types";
import { compactViewerSearch, normalizeViewerSearch, toStoryFilters } from "./viewerSearch";

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

  const { dayNav, selectedDay } = useDayNavigationState({
    dayBuckets,
    from: filters.from,
    to: filters.to,
  });

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
  const currentCollectionLabel = useCurrentCollectionLabel(collections, filters.collection);

  useEffect(() => {
    if (typeof document === "undefined") {
      return;
    }
    document.title = `Scoop • ${currentCollectionLabel}`;
  }, [currentCollectionLabel]);

  const pickerDay = selectedDay || dayNav.navigatorDay;
  const headerLeft = (
    <CollectionDropdown
      selectedCollection={filters.collection}
      allCollectionsValue={allCollectionsValue}
      allCollectionsLabel={allCollectionsLabel}
      currentCollectionLabel={currentCollectionLabel}
      collections={collections}
      onCollectionChange={onCollectionChange}
    />
  );

  const headerRight = (
    <DayNavigator
      dayNav={dayNav}
      pickerDay={pickerDay}
      onMoveOlder={() => moveDay(1)}
      onMoveNewer={() => moveDay(-1)}
      onSelectDay={setSingleDayFilter}
    />
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
