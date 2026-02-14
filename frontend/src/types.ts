export interface JSendSuccess<T> {
  status: "success";
  data: T;
  message?: string;
  code?: number;
}

export interface JSendFailure {
  status: "fail" | "error";
  message?: string;
  data?: unknown;
  code?: number;
}

export type JSendResponse<T> = JSendSuccess<T> | JSendFailure;

export interface StoryRepresentative {
  document_uuid: string;
  source: string;
  source_item_id: string;
  published_at?: string;
}

export interface StoryListItem {
  story_id: number;
  story_uuid: string;
  collection: string;
  title: string;
  canonical_url?: string;
  status: string;
  first_seen_at: string;
  last_seen_at: string;
  source_count: number;
  item_count: number;
  representative?: StoryRepresentative;
}

export interface StoryMemberItem {
  story_member_uuid: string;
  document_uuid: string;
  source: string;
  source_item_id: string;
  collection: string;
  canonical_url?: string;
  published_at?: string;
  normalized_title: string;
  source_domain?: string;
  matched_at: string;
  match_type: string;
  match_score?: number;
  match_details?: Record<string, unknown>;
  dedup_decision?: string;
  dedup_exact_signal?: string;
  dedup_best_cosine?: number;
  dedup_title_overlap?: number;
  dedup_date_consistency?: number;
  dedup_composite_score?: number;
}

export interface StoryDetailResponse {
  story: StoryListItem;
  members: StoryMemberItem[];
}

export interface CollectionSummary {
  collection: string;
  documents: number;
  stories: number;
  story_items: number;
  last_story_seen_at?: string;
}

export interface StoryDayBucket {
  day: string;
  story_count: number;
}

export interface StoryPagination {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export interface StoriesResponse {
  items: StoryListItem[];
  pagination: StoryPagination;
}

export interface StatsResponse {
  raw_arrivals: number;
  documents: number;
  stories: number;
  story_members: number;
  dedup_events: number;
  running_ingest_runs: number;
  last_fetched_at?: string;
  last_story_updated?: string;
  dedup_decisions: Record<string, number>;
}

export interface StoryFilters {
  page: number;
  pageSize: number;
  collection: string;
  query: string;
  from: string;
  to: string;
}

export interface ViewerSearch {
  collection?: string;
  q?: string;
  page?: number;
  from?: string;
  to?: string;
}

export interface DayNavigationState {
  currentIndex: number;
  canGoOlder: boolean;
  canGoNewer: boolean;
  currentLabel: string;
  navigatorDay: string;
  relativeLabel: string;
}
