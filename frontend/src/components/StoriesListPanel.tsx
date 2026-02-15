import { useEffect, useRef } from "react";

import { buildStoryMetaText, formatCount } from "../lib/viewerFormat";
import type { StoryListItem } from "../types";

interface StoriesListPanelProps {
  totalItems: number;
  loadedItems: number;
  selectedStoryUUID: string;
  stories: StoryListItem[];
  isLoading: boolean;
  isFetchingNextPage: boolean;
  hasNextPage: boolean;
  error: string;
  onLoadNextPage: () => void;
  onSelectStory: (storyUUID: string) => void;
}

export function StoriesListPanel({
  totalItems,
  loadedItems,
  selectedStoryUUID,
  stories,
  isLoading,
  isFetchingNextPage,
  hasNextPage,
  error,
  onLoadNextPage,
  onSelectStory,
}: StoriesListPanelProps): JSX.Element {
  const listRef = useRef<HTMLDivElement | null>(null);
  const loadTriggerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!hasNextPage || isLoading || isFetchingNextPage || Boolean(error)) {
      return;
    }

    const root = listRef.current;
    const target = loadTriggerRef.current;
    if (!root || !target) {
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          onLoadNextPage();
        }
      },
      {
        root,
        rootMargin: "220px 0px",
        threshold: 0,
      },
    );

    observer.observe(target);
    return () => observer.disconnect();
  }, [error, hasNextPage, isFetchingNextPage, isLoading, onLoadNextPage, stories.length]);

  return (
    <section className="panel card">
      <div className="panel-header">
        <div>
          <p className="eyebrow">Story Feed</p>
          <h2>{formatCount(totalItems)} Stories</h2>
        </div>
        <span className="page-label">{formatCount(Math.min(loadedItems, totalItems))} loaded</span>
      </div>

      <div ref={listRef} className="stories-list">
        {isLoading ? <p className="muted">Loading stories...</p> : null}
        {!isLoading && error ? <p className="muted">{error}</p> : null}
        {!isLoading && !error && stories.length === 0 ? <p className="muted">No stories match this filter.</p> : null}

        {!isLoading && !error
          ? stories.map((story) => (
              <article
                key={story.story_uuid}
                className={`story-card ${story.story_uuid === selectedStoryUUID ? "active" : ""}`.trim()}
                onClick={() => onSelectStory(story.story_uuid)}
                role="button"
                tabIndex={0}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    onSelectStory(story.story_uuid);
                  }
                }}
              >
                <header>
                  <div className="story-topline">
                    <span className="badge collection">{story.collection}</span>
                    <span className="badge count">{story.item_count} docs</span>
                  </div>
                  <h3 className="story-title">{story.title || "(untitled)"}</h3>
                </header>
                <p className="story-meta">{buildStoryMetaText(story.last_seen_at, story.source_count)}</p>
              </article>
            ))
          : null}

        {!isLoading && !error ? <div ref={loadTriggerRef} className="stories-load-sentinel" aria-hidden="true" /> : null}
        {isFetchingNextPage ? <p className="muted stories-status">Loading more stories...</p> : null}
        {!isFetchingNextPage && !hasNextPage && stories.length > 0 ? (
          <p className="muted stories-status">Reached the end of this story feed.</p>
        ) : null}
      </div>
    </section>
  );
}
