import { buildStoryMetaText, formatCount } from "../lib/viewerFormat";
import type { StoryListItem } from "../types";

interface StoriesListPanelProps {
  page: number;
  totalPages: number;
  totalItems: number;
  selectedStoryUUID: string;
  stories: StoryListItem[];
  isLoading: boolean;
  error: string;
  onPrevPage: () => void;
  onNextPage: () => void;
  onSelectStory: (storyUUID: string) => void;
}

export function StoriesListPanel({
  page,
  totalPages,
  totalItems,
  selectedStoryUUID,
  stories,
  isLoading,
  error,
  onPrevPage,
  onNextPage,
  onSelectStory,
}: StoriesListPanelProps): JSX.Element {
  return (
    <section className="panel card">
      <div className="panel-header">
        <div>
          <p className="eyebrow">Canonical Stories</p>
          <h2>{formatCount(totalItems)} Stories</h2>
        </div>
        <div className="pager">
          <button className="btn btn-subtle" type="button" onClick={onPrevPage} disabled={page <= 1}>
            Prev
          </button>
          <span className="page-label">
            Page {page} / {totalPages}
          </span>
          <button className="btn btn-subtle" type="button" onClick={onNextPage} disabled={page >= totalPages}>
            Next
          </button>
        </div>
      </div>

      <div className="stories-list">
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
      </div>
    </section>
  );
}
