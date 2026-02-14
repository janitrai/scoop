import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { getCollections, getStoryDays, getStoryDetail, getStories } from "../api";
import { buildPagination, extractErrorMessage } from "../lib/viewerFormat";
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

  const storiesQuery = useQuery<{ items: StoryListItem[]; pagination: StoryPagination }>({
    queryKey: [
      "stories",
      filters.collection,
      filters.query,
      filters.from,
      filters.to,
      filters.page,
      filters.pageSize,
      refreshTick,
    ],
    queryFn: () => getStories(filters),
  });

  const detailQuery = useQuery<StoryDetailResponse>({
    queryKey: ["story-detail", selectedStoryUUID, refreshTick],
    queryFn: () => getStoryDetail(selectedStoryUUID),
    enabled: selectedStoryUUID !== "",
  });

  const collections = collectionsQuery.data?.items ?? [];
  const dayBuckets = dayBucketsQuery.data?.items ?? [];
  const stories = storiesQuery.data?.items ?? [];
  const detail = detailQuery.data ?? null;

  const pagination = useMemo(
    () => buildPagination(filters.page, filters.pageSize, storiesQuery.data?.pagination),
    [filters.page, filters.pageSize, storiesQuery.data?.pagination],
  );

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
    isStoriesPending: storiesQuery.isPending,
    isDetailPending: detailQuery.isPending,
  };
}
