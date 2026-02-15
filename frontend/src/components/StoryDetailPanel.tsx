import { useMemo } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";

import { buildMemberSubtitle, formatDateTime } from "../lib/viewerFormat";
import type { StoryDetailResponse } from "../types";

interface StoryDetailPanelProps {
  selectedStoryUUID: string;
  selectedItemUUID: string;
  detail: StoryDetailResponse | null;
  isLoading: boolean;
  error: string;
  onSelectItem: (itemUUID: string) => void;
  onClearSelectedItem: () => void;
}

export function StoryDetailPanel({
  selectedStoryUUID,
  selectedItemUUID,
  detail,
  isLoading,
  error,
  onSelectItem,
  onClearSelectedItem,
}: StoryDetailPanelProps): JSX.Element {
  const mergedLinks = useMemo(() => {
    if (!detail) {
      return [];
    }

    const byURL = new Map<string, string>();

    const storyURL = detail.story.canonical_url?.trim();
    if (storyURL) {
      byURL.set(storyURL, storyURL);
    }

    for (const member of detail.members) {
      const itemURL = member.canonical_url?.trim();
      if (!itemURL || byURL.has(itemURL)) {
        continue;
      }
      byURL.set(itemURL, itemURL);
    }

    return Array.from(byURL.values());
  }, [detail]);

  function buildMemberPreview(text?: string): string {
    const collapsed = (text ?? "").replace(/\s+/g, " ").trim();
    if (!collapsed) {
      return "No content captured for this item.";
    }

    const maxChars = 260;
    if (collapsed.length <= maxChars) {
      return collapsed;
    }
    return `${collapsed.slice(0, maxChars).trimEnd()}...`;
  }

  function renderStoryHeader(): JSX.Element {
    if (!detail) {
      return <></>;
    }

    return (
      <>
        <h2 className="detail-title">{detail.story.title || "(untitled)"}</h2>
        <p className="detail-meta">
          Collection: {detail.story.collection} • {detail.story.item_count} items • {detail.story.source_count} sources
        </p>

        {mergedLinks.length > 0 ? (
          <section className="story-links-block">
            <ul className="story-links-list">
              {mergedLinks.map((url) => (
                <li key={url}>
                  <a className="detail-url" href={url} target="_blank" rel="noreferrer">
                    {url}
                  </a>
                </li>
              ))}
            </ul>
          </section>
        ) : null}

        <p className="detail-meta">
          first seen {formatDateTime(detail.story.first_seen_at)} • last seen {formatDateTime(detail.story.last_seen_at)}
        </p>
      </>
    );
  }

  function renderStoryView(): JSX.Element {
    if (!detail) {
      return <></>;
    }

    return (
      <>
        {renderStoryHeader()}
        <section className="member-grid">
          {detail.members.length === 0 ? <p className="muted">No items found for this story.</p> : null}
          {detail.members.map((member) => {
            const isExpanded = member.story_member_uuid === selectedItemUUID;
            const content = member.normalized_text ?? "";
            const hasContent = content.trim() !== "";
            const decisionText = member.dedup_decision ? member.dedup_decision.toLowerCase() : "";

            return (
              <article
                key={member.story_member_uuid}
                className={`member-card ${isExpanded ? "member-card-expanded" : ""}`.trim()}
              >
              <button
                type="button"
                className={`member-toggle ${isExpanded ? "expanded" : ""}`.trim()}
                onClick={() => {
                  if (isExpanded) {
                    onClearSelectedItem();
                    return;
                  }
                  onSelectItem(member.story_member_uuid);
                }}
                aria-expanded={isExpanded}
                aria-label={`${isExpanded ? "Collapse" : "Expand"} item ${member.normalized_title || "(no title)"}`}
              >
                <p className="member-head">{member.normalized_title || "(no title)"}</p>
                {isExpanded ? (
                  <ChevronDown className="member-toggle-icon" aria-hidden="true" />
                ) : (
                  <ChevronRight className="member-toggle-icon" aria-hidden="true" />
                )}
              </button>
              {isExpanded ? <p className="member-sub">{buildMemberSubtitle(member)}</p> : null}
              <p className="member-sub">
                matched {formatDateTime(member.matched_at)} • published {formatDateTime(member.published_at)}
                {decisionText ? (
                  <>
                    {" "}
                    • <span className="member-decision-inline">{decisionText}</span>
                  </>
                ) : null}
              </p>
              {isExpanded ? (
                <>
                  {member.canonical_url ? (
                    <a className="member-expanded-url" href={member.canonical_url} target="_blank" rel="noreferrer">
                      {member.canonical_url}
                    </a>
                  ) : null}
                  <article className="detail-item-content member-expanded-content">
                    {hasContent ? (
                      <p className="detail-item-content-text">{content}</p>
                    ) : (
                      <p className="muted">No content captured for this item.</p>
                    )}
                  </article>
                </>
              ) : null}
              {!isExpanded ? <p className="member-preview member-preview-collapsed">{buildMemberPreview(member.normalized_text)}</p> : null}
            </article>
            );
          })}
        </section>
      </>
    );
  }

  return (
    <aside className="panel card detail-panel">
      <div className="detail-content">
        {!selectedStoryUUID ? <p className="muted">Pick a story to inspect merged documents.</p> : null}
        {selectedStoryUUID && isLoading ? <p className="muted">Fetching story detail...</p> : null}
        {selectedStoryUUID && !isLoading && error ? <p className="muted">{error}</p> : null}

        {selectedStoryUUID && !isLoading && !error && detail ? renderStoryView() : null}
      </div>
    </aside>
  );
}
