import { useEffect, useRef, useState } from "react";
import { Settings2 } from "lucide-react";

import { buildStoryMetaText, formatCount } from "../lib/viewerFormat";
import type { StoryListItem } from "../types";

interface StoriesListPanelProps {
  searchInput: string;
  from: string;
  to: string;
  totalItems: number;
  loadedItems: number;
  selectedStoryUUID: string;
  stories: StoryListItem[];
  isLoading: boolean;
  isFetchingNextPage: boolean;
  hasNextPage: boolean;
  error: string;
  onSearchInputChange: (value: string) => void;
  onFromChange: (value: string) => void;
  onToChange: (value: string) => void;
  onLoadNextPage: () => void;
  onSelectStory: (storyUUID: string) => void;
}

export function StoriesListPanel({
  searchInput,
  from,
  to,
  totalItems,
  loadedItems,
  selectedStoryUUID,
  stories,
  isLoading,
  isFetchingNextPage,
  hasNextPage,
  error,
  onSearchInputChange,
  onFromChange,
  onToChange,
  onLoadNextPage,
  onSelectStory,
}: StoriesListPanelProps): JSX.Element {
  const listRef = useRef<HTMLDivElement | null>(null);
  const loadTriggerRef = useRef<HTMLDivElement | null>(null);
  const [showAdvancedSearch, setShowAdvancedSearch] = useState(false);

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
    <section className="panel card story-feed-panel">
      <div className="feed-search">
        <div className="finder-row">
          <div className="finder-input-wrap">
            <span className="finder-icon" aria-hidden="true">
              /
            </span>
            <input
              value={searchInput}
              onChange={(event) => onSearchInputChange(event.target.value)}
              type="text"
              placeholder="Search stories (title or URL)"
              aria-label="Search stories"
            />
            {searchInput ? (
              <button
                type="button"
                className="finder-clear"
                onClick={() => onSearchInputChange("")}
                aria-label="Clear search"
              >
                x
              </button>
            ) : null}
          </div>

          <button
            className={`finder-advanced-icon ${showAdvancedSearch ? "active" : ""}`.trim()}
            type="button"
            aria-label="Toggle advanced search"
            aria-pressed={showAdvancedSearch}
            onClick={() => setShowAdvancedSearch((value) => !value)}
          >
            <Settings2 size={18} strokeWidth={1.9} aria-hidden="true" />
          </button>
        </div>

        {showAdvancedSearch ? (
          <div className="advanced-row">
            <label className="field field-small">
              <span>From</span>
              <input value={from} onChange={(event) => onFromChange(event.target.value)} type="date" />
            </label>

            <label className="field field-small">
              <span>To</span>
              <input value={to} onChange={(event) => onToChange(event.target.value)} type="date" />
            </label>
          </div>
        ) : null}
      </div>

      <div className="stories-list-wrap">
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

        <div className="feed-count-overlay" aria-live="polite">
          {formatCount(Math.min(loadedItems, totalItems))}/{formatCount(totalItems)}
        </div>
      </div>
    </section>
  );
}
