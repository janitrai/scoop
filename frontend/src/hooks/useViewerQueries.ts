import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";

import { getCollections, getStoryDays, getStoryDetail, getStories } from "../api";
import { extractErrorMessage } from "../lib/viewerFormat";
import type {
  CollectionSummary,
  StoryDayBucket,
  StoryDetailResponse,
  StoryFilters,
  StoryListItem,
  StoryPagination,
} from "../types";

interface UseViewerQueriesArgs {
  filters: StoryFilters;
  selectedStoryUUID: string;
  refreshTick: number;
}

interface UseViewerQueriesResult {
  collections: CollectionSummary[];
  dayBuckets: StoryDayBucket[];
  stories: StoryListItem[];
  detail: StoryDetailResponse | null;
  pagination: StoryPagination;
  globalError: string;
  storiesError: string;
  detailError: string;
  isStoriesPending: boolean;
  isFetchingNextStoriesPage: boolean;
  hasNextStoriesPage: boolean;
  fetchNextStoriesPage: () => void;
  isDetailPending: boolean;
}

export function useViewerQueries({
  filters,
  selectedStoryUUID,
  refreshTick,
}: UseViewerQueriesArgs): UseViewerQueriesResult {
  const collectionsQuery = useQuery<{ items: CollectionSummary[] }>({
    queryKey: ["collections", refreshTick],
    queryFn: getCollections,
  });

  const dayBucketsQuery = useQuery<{ items: StoryDayBucket[] }>({
    queryKey: ["story-days", filters.collection, refreshTick],
    queryFn: () => getStoryDays(filters.collection, 45),
  });

  const storiesQuery = useInfiniteQuery<{ items: StoryListItem[]; pagination: StoryPagination }>({
    queryKey: [
      "stories",
      filters.collection,
      filters.query,
      filters.from,
      filters.to,
      filters.pageSize,
      refreshTick,
    ],
    initialPageParam: 1,
    queryFn: ({ pageParam }) =>
      getStories({
        ...filters,
        page: Number(pageParam),
      }),
    getNextPageParam: (lastPage) => {
      const nextPage = lastPage.pagination.page + 1;
      return nextPage <= lastPage.pagination.total_pages ? nextPage : undefined;
    },
  });

  const detailQuery = useQuery<StoryDetailResponse>({
    queryKey: ["story-detail", selectedStoryUUID, refreshTick],
    queryFn: () => getStoryDetail(selectedStoryUUID),
    enabled: selectedStoryUUID !== "",
  });

  const collections = collectionsQuery.data?.items ?? [];
  const dayBuckets = dayBucketsQuery.data?.items ?? [];
  const storyPages = storiesQuery.data?.pages ?? [];
  const stories = useMemo(() => storyPages.flatMap((page) => page.items), [storyPages]);
  const detail = detailQuery.data ?? null;

  const pagination = useMemo((): StoryPagination => {
    const firstPage = storyPages[0]?.pagination;
    const lastPage = storyPages[storyPages.length - 1]?.pagination;

    return {
      page: lastPage?.page ?? 1,
      page_size: lastPage?.page_size ?? filters.pageSize,
      total_items: Number(firstPage?.total_items ?? 0),
      total_pages: Math.max(1, Number(firstPage?.total_pages ?? 1)),
    };
  }, [filters.pageSize, storyPages]);

  const fetchNextStoriesPage = useCallback(() => {
    if (!storiesQuery.hasNextPage || storiesQuery.isFetchingNextPage) {
      return;
    }
    void storiesQuery.fetchNextPage();
  }, [storiesQuery]);

  const globalError = useMemo(() => {
    if (collectionsQuery.error) return extractErrorMessage(collectionsQuery.error);
    if (dayBucketsQuery.error) return extractErrorMessage(dayBucketsQuery.error);
    return "";
  }, [collectionsQuery.error, dayBucketsQuery.error]);

  const storiesError = storiesQuery.error ? extractErrorMessage(storiesQuery.error) : "";
  const detailError = detailQuery.error ? extractErrorMessage(detailQuery.error) : "";

  return {
    collections,
    dayBuckets,
    stories,
    detail,
    pagination,
    globalError,
    storiesError,
    detailError,
    isStoriesPending: storiesQuery.isPending && stories.length === 0,
    isFetchingNextStoriesPage: storiesQuery.isFetchingNextPage,
    hasNextStoriesPage: Boolean(storiesQuery.hasNextPage),
    fetchNextStoriesPage,
    isDetailPending: detailQuery.isPending,
  };
}
