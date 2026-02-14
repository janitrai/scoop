import type {
  CollectionSummary,
  JSendResponse,
  StatsResponse,
  StoryDayBucket,
  StoryDetailResponse,
  StoryFilters,
  StoriesResponse,
} from "./types";

async function fetchJSend<T>(path: string): Promise<T> {
  const response = await fetch(path, {
    headers: {
      Accept: "application/json",
    },
  });

  const payload = (await response.json().catch(() => ({}))) as Partial<JSendResponse<T>>;
  const isSuccess = payload.status === "success";

  if (!response.ok || !isSuccess || payload.data === undefined) {
    const message = typeof payload.message === "string" ? payload.message : `Request failed (${response.status})`;
    throw new Error(message);
  }

  return payload.data as T;
}

export async function getStats(): Promise<StatsResponse> {
  return fetchJSend<StatsResponse>("/api/v1/stats");
}

export async function getCollections(): Promise<{ items: CollectionSummary[] }> {
  return fetchJSend<{ items: CollectionSummary[] }>("/api/v1/collections");
}

export async function getStoryDays(collection: string, limit = 45): Promise<{ items: StoryDayBucket[] }> {
  const params = new URLSearchParams();
  params.set("limit", String(limit));
  if (collection) {
    params.set("collection", collection);
  }
  return fetchJSend<{ items: StoryDayBucket[] }>(`/api/v1/story-days?${params.toString()}`);
}

export async function getStories(filters: StoryFilters): Promise<StoriesResponse> {
  const params = new URLSearchParams();
  params.set("page", String(filters.page));
  params.set("page_size", String(filters.pageSize));
  if (filters.collection) {
    params.set("collection", filters.collection);
  }
  if (filters.query) {
    params.set("q", filters.query);
  }
  if (filters.from) {
    params.set("from", filters.from);
  }
  if (filters.to) {
    params.set("to", filters.to);
  }

  return fetchJSend<StoriesResponse>(`/api/v1/stories?${params.toString()}`);
}

export async function getStoryDetail(storyUUID: string): Promise<StoryDetailResponse> {
  return fetchJSend<StoryDetailResponse>(`/api/v1/stories/${storyUUID}`);
}
