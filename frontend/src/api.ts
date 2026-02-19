import type {
  CollectionSummary,
  JSendResponse,
  StatsResponse,
  StoryDayBucket,
  StoryDetailResponse,
  StoryFilters,
  StoryArticlePreview,
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

function appendLang(params: URLSearchParams, lang?: string): void {
  const trimmed = (lang || "").trim();
  if (!trimmed) {
    return;
  }
  params.set("lang", trimmed);
}

function withLang(path: string, lang?: string): string {
  const trimmed = (lang || "").trim();
  if (!trimmed) {
    return path;
  }
  const separator = path.includes("?") ? "&" : "?";
  return `${path}${separator}lang=${encodeURIComponent(trimmed)}`;
}

export async function getStats(lang = ""): Promise<StatsResponse> {
  return fetchJSend<StatsResponse>(withLang("/api/v1/stats", lang));
}

export async function getCollections(lang = ""): Promise<{ items: CollectionSummary[] }> {
  return fetchJSend<{ items: CollectionSummary[] }>(withLang("/api/v1/collections", lang));
}

export async function getStoryDays(collection: string, limit = 45, lang = ""): Promise<{ items: StoryDayBucket[] }> {
  const params = new URLSearchParams();
  params.set("limit", String(limit));
  if (collection) {
    params.set("collection", collection);
  }
  appendLang(params, lang);
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
  appendLang(params, filters.lang);

  return fetchJSend<StoriesResponse>(`/api/v1/stories?${params.toString()}`);
}

export async function getStoryDetail(storyUUID: string, lang = ""): Promise<StoryDetailResponse> {
  return fetchJSend<StoryDetailResponse>(withLang(`/api/v1/stories/${storyUUID}`, lang));
}

export async function requestTranslation(
  storyUUID: string,
  targetLang: string,
  provider?: string,
): Promise<{ stats: { translated: number; cached: number; failed: number } }> {
  const body: Record<string, string> = { story_uuid: storyUUID, target_lang: targetLang };
  if (provider) body.provider = provider;
  const response = await fetch("/api/v1/translate", {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(body),
  });
  const payload = (await response.json().catch(() => ({}))) as Partial<JSendResponse<any>>;
  if (payload.status !== "success" || !payload.data) {
    throw new Error(typeof payload.message === "string" ? payload.message : "Translation failed");
  }
  return payload.data;
}

export async function getStoryArticlePreview(
  storyArticleUUID: string,
  maxChars = 1000,
  lang = "",
): Promise<StoryArticlePreview> {
  const params = new URLSearchParams();
  params.set("max_chars", String(maxChars));
  appendLang(params, lang);
  return fetchJSend<StoryArticlePreview>(`/api/v1/articles/${storyArticleUUID}/preview?${params.toString()}`);
}
