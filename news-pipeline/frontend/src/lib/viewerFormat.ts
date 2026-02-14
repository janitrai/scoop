import type { StoryMemberItem, StoryPagination } from "../types";

function parseCalendarDay(value: string): Date | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    return null;
  }
  const [year, month, day] = value.split("-").map((part) => Number(part));
  const date = new Date(year, month - 1, day);
  if (Number.isNaN(date.getTime())) {
    return null;
  }
  if (date.getFullYear() !== year || date.getMonth() !== month - 1 || date.getDate() !== day) {
    return null;
  }
  return date;
}

function pluralize(value: number, unit: string): string {
  return value === 1 ? `${value} ${unit}` : `${value} ${unit}s`;
}

export function formatCount(value: number | null | undefined): string {
  return Number(value ?? 0).toLocaleString("en-US");
}

export function formatDateTime(value?: string): string {
  if (!value) {
    return "n/a";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "n/a";
  }
  return date.toLocaleString();
}

export function formatCalendarDay(value: string): string {
  const date = parseCalendarDay(value);
  if (!date) {
    return "Pick a day";
  }
  return date.toLocaleDateString("en-US", {
    weekday: "short",
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

export function formatRelativeDay(value: string): string {
  const date = parseCalendarDay(value);
  if (!date) {
    return "unknown day";
  }

  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const msPerDay = 24 * 60 * 60 * 1000;
  const diffDays = Math.round((today.getTime() - date.getTime()) / msPerDay);

  if (diffDays === 0) return "today";
  if (diffDays === 1) return "yesterday";
  if (diffDays === -1) return "tomorrow";

  if (diffDays > 1) {
    if (diffDays < 7) return `${diffDays} days ago`;
    if (diffDays < 30) return `${pluralize(Math.floor(diffDays / 7), "week")} ago`;
    if (diffDays < 365) return `${pluralize(Math.floor(diffDays / 30), "month")} ago`;
    return `${pluralize(Math.floor(diffDays / 365), "year")} ago`;
  }

  const future = Math.abs(diffDays);
  if (future < 7) return `in ${pluralize(future, "day")}`;
  if (future < 30) return `in ${pluralize(Math.floor(future / 7), "week")}`;
  if (future < 365) return `in ${pluralize(Math.floor(future / 30), "month")}`;
  return `in ${pluralize(Math.floor(future / 365), "year")}`;
}

export function extractErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message.trim() !== "") {
    return error.message;
  }
  return "Request failed.";
}

export function buildStoryMetaText(lastSeenAt: string, sourceCount: number): string {
  return `last ${formatDateTime(lastSeenAt)} • ${sourceCount} sources`;
}

export function buildMemberSubtitle(member: StoryMemberItem): string {
  const scoreSuffix = member.match_score == null ? "" : ` • score ${Number(member.match_score).toFixed(3)}`;
  return `${member.source}:${member.source_item_id} • ${member.match_type}${scoreSuffix}`;
}

export function buildPagination(page: number, pageSize: number, incoming?: Partial<StoryPagination>): StoryPagination {
  return {
    page: incoming?.page ?? page,
    page_size: incoming?.page_size ?? pageSize,
    total_items: Number(incoming?.total_items ?? 0),
    total_pages: Math.max(1, Number(incoming?.total_pages ?? 1)),
  };
}
