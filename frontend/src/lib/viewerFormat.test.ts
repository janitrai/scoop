import { describe, expect, it } from "vitest";

import type { StoryListItem } from "../types";

import { buildFeedSourceText } from "./viewerFormat";

function makeStory(overrides: Partial<StoryListItem>): StoryListItem {
  return {
    story_id: 1,
    story_uuid: "00000000-0000-0000-0000-000000000001",
    collection: "test",
    title: "Test story",
    status: "active",
    first_seen_at: "2026-02-15T00:00:00Z",
    last_seen_at: "2026-02-15T00:00:00Z",
    source_count: 1,
    item_count: 1,
    ...overrides,
  };
}

describe("buildFeedSourceText", () => {
  it("shows domain for single-source stories", () => {
    const story = makeStory({
      canonical_url: "https://www.nytimes.com/2026/02/15/world/example.html",
      source_count: 1,
    });

    expect(buildFeedSourceText(story)).toBe("nytimes.com");
  });

  it("shows domain and others for multi-source stories", () => {
    const story = makeStory({
      canonical_url: "https://news.ycombinator.com/item?id=123",
      source_count: 4,
    });

    expect(buildFeedSourceText(story)).toBe("news.ycombinator.com and 3 others");
  });

  it("falls back to representative source when URL is missing", () => {
    const story = makeStory({
      canonical_url: undefined,
      source_count: 2,
      representative: {
        document_uuid: "00000000-0000-0000-0000-000000000002",
        source: "reuters",
        source_item_id: "abc",
      },
    });

    expect(buildFeedSourceText(story)).toBe("reuters and 1 other");
  });

  it("falls back to count text when neither URL nor source exists", () => {
    const story = makeStory({
      canonical_url: undefined,
      source_count: 2,
      representative: undefined,
    });

    expect(buildFeedSourceText(story)).toBe("2 sources");
  });
});

