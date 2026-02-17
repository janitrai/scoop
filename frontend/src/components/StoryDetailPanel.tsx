import { useEffect, useMemo, useRef, useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";

import { getStoryArticlePreview } from "../api";
import { buildMemberSubtitle, formatDateTime } from "../lib/viewerFormat";
import type { StoryDetailResponse, StoryArticlePreview, StoryArticle } from "../types";

interface StoryDetailPanelProps {
  selectedStoryUUID: string;
  selectedItemUUID: string;
  detail: StoryDetailResponse | null;
  isLoading: boolean;
  error: string;
  onSelectItem: (itemUUID: string) => void;
  onClearSelectedItem: () => void;
}

function pruneRecord<T>(record: Record<string, T>, validIDs: Set<string>): Record<string, T> {
  const next: Record<string, T> = {};
  let changed = false;

  for (const [key, value] of Object.entries(record)) {
    if (validIDs.has(key)) {
      next[key] = value;
      continue;
    }
    changed = true;
  }

  if (!changed && Object.keys(next).length === Object.keys(record).length) {
    return record;
  }
  return next;
}

interface MemberURLGroup {
  key: string;
  canonicalURL: string;
  members: StoryArticle[];
  representative: StoryArticle;
  sourceCount: number;
}

function memberGroupKey(member: StoryArticle): string {
  const canonicalURL = member.canonical_url?.trim().toLowerCase() ?? "";
  if (canonicalURL) {
    return `url:${canonicalURL}`;
  }
  return `member:${member.story_article_uuid}`;
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
  const [expandedGroupKeys, setExpandedGroupKeys] = useState<string[]>([]);
  const [itemPreviewByUUID, setItemPreviewByUUID] = useState<Record<string, StoryArticlePreview>>({});
  const [itemPreviewLoadingByUUID, setItemPreviewLoadingByUUID] = useState<Record<string, boolean>>({});
  const [itemPreviewRequestedByUUID, setItemPreviewRequestedByUUID] = useState<Record<string, boolean>>({});
  const [itemPreviewErrorByUUID, setItemPreviewErrorByUUID] = useState<Record<string, string>>({});
  const previousStoryUUIDRef = useRef<string>("");

  const memberGroups = useMemo<MemberURLGroup[]>(() => {
    if (!detail) {
      return [];
    }

    const grouped = new Map<string, StoryArticle[]>();
    for (const member of detail.members) {
      const key = memberGroupKey(member);
      const members = grouped.get(key);
      if (members) {
        members.push(member);
        continue;
      }
      grouped.set(key, [member]);
    }

    return Array.from(grouped.entries()).map(([key, members]) => {
      const sourceCount = new Set(members.map((member) => member.source)).size;
      const representative = members[0];

      return {
        key,
        canonicalURL: representative.canonical_url?.trim() ?? "",
        members,
        representative,
        sourceCount,
      };
    });
  }, [detail]);

  const groupKeyByItemUUID = useMemo<Record<string, string>>(() => {
    const mapping: Record<string, string> = {};
    for (const group of memberGroups) {
      for (const member of group.members) {
        mapping[member.story_article_uuid] = group.key;
      }
    }
    return mapping;
  }, [memberGroups]);

  const selectedGroupKey = selectedItemUUID ? (groupKeyByItemUUID[selectedItemUUID] ?? "") : "";

  useEffect(() => {
    if (!detail) {
      setExpandedGroupKeys([]);
      setItemPreviewByUUID({});
      setItemPreviewLoadingByUUID({});
      setItemPreviewRequestedByUUID({});
      setItemPreviewErrorByUUID({});
      previousStoryUUIDRef.current = "";
      return;
    }

    const validItemIDs = new Set(detail.members.map((member) => member.story_article_uuid));
    const validGroupKeys = new Set(memberGroups.map((group) => group.key));
    const isNewStorySelection = previousStoryUUIDRef.current !== detail.story.story_uuid;
    previousStoryUUIDRef.current = detail.story.story_uuid;

    setExpandedGroupKeys((previous) => {
      if (isNewStorySelection) {
        const next = memberGroups.map((group) => group.key);

        if (selectedGroupKey && validGroupKeys.has(selectedGroupKey) && !next.includes(selectedGroupKey)) {
          next.push(selectedGroupKey);
        }

        return next;
      }

      const next = previous.filter((groupKey) => validGroupKeys.has(groupKey));

      if (selectedGroupKey && validGroupKeys.has(selectedGroupKey) && !next.includes(selectedGroupKey)) {
        next.push(selectedGroupKey);
      }

      if (
        next.length === previous.length &&
        next.every((groupKey, index) => groupKey === previous[index])
      ) {
        return previous;
      }

      return next;
    });

    setItemPreviewByUUID((previous) => pruneRecord(previous, validItemIDs));
    setItemPreviewLoadingByUUID((previous) => pruneRecord(previous, validItemIDs));
    setItemPreviewRequestedByUUID((previous) => pruneRecord(previous, validItemIDs));
    setItemPreviewErrorByUUID((previous) => pruneRecord(previous, validItemIDs));
  }, [detail, memberGroups, selectedGroupKey]);

  useEffect(() => {
    if (!detail) {
      return;
    }

    for (const member of detail.members) {
      const itemUUID = member.story_article_uuid;
      if (itemPreviewRequestedByUUID[itemUUID]) {
        continue;
      }

      setItemPreviewRequestedByUUID((previous) => ({
        ...previous,
        [itemUUID]: true,
      }));
      setItemPreviewLoadingByUUID((previous) => ({
        ...previous,
        [itemUUID]: true,
      }));
      setItemPreviewErrorByUUID((previous) => {
        if (!previous[itemUUID]) {
          return previous;
        }
        const next = { ...previous };
        delete next[itemUUID];
        return next;
      });

      void getStoryArticlePreview(itemUUID, 1000)
        .then((preview) => {
          setItemPreviewByUUID((previous) => ({
            ...previous,
            [itemUUID]: preview,
          }));
        })
        .catch((fetchErr) => {
          const message = fetchErr instanceof Error ? fetchErr.message : "Failed to fetch reader preview.";
          setItemPreviewErrorByUUID((previous) => ({
            ...previous,
            [itemUUID]: message,
          }));
        })
        .finally(() => {
          setItemPreviewLoadingByUUID((previous) => {
            if (!previous[itemUUID]) {
              return previous;
            }
            const next = { ...previous };
            delete next[itemUUID];
            return next;
          });
        });
    }
  }, [detail, itemPreviewRequestedByUUID]);

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

  function toParagraphs(text: string): string[] {
    return text
      .split(/\n{2,}/)
      .map((paragraph) => paragraph.trim())
      .filter((paragraph) => paragraph.length > 0);
  }

  function renderStoryHeader(): JSX.Element {
    if (!detail) {
      return <></>;
    }

    return (
      <>
        <h2 className="detail-title">{detail.story.title || "(untitled)"}</h2>
        <p className="detail-meta">
          Collection: {detail.story.collection} • {detail.story.article_count} items • {detail.story.source_count} sources
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
          {memberGroups.length === 0 ? <p className="muted">No items found for this story.</p> : null}
          {memberGroups.map((group) => {
            const representative = group.representative;
            const isExpanded = expandedGroupKeys.includes(group.key);
            const hasSelectedMember = selectedGroupKey === group.key;
            const decisionText = representative.dedup_decision ? representative.dedup_decision.toLowerCase() : "";

            const previewTexts = group.members
              .map((member) => itemPreviewByUUID[member.story_article_uuid]?.preview_text?.trim() ?? "")
              .filter((text) => text.length > 0);
            const normalizedTexts = group.members
              .map((member) => member.normalized_text?.trim() ?? "")
              .filter((text) => text.length > 0);

            const resolvedExpandedText = previewTexts[0] || normalizedTexts[0] || "";
            const resolvedParagraphs = toParagraphs(resolvedExpandedText);
            const hasResolvedContent = resolvedParagraphs.length > 0;
            const isPreviewLoading = group.members.some(
              (member) => Boolean(itemPreviewLoadingByUUID[member.story_article_uuid]),
            );
            const previewError = group.members.some(
              (member) => Boolean(itemPreviewErrorByUUID[member.story_article_uuid]),
            );
            const routeItemUUID = hasSelectedMember ? selectedItemUUID : representative.story_article_uuid;

            return (
              <article
                key={group.key}
                className={`member-card ${isExpanded ? "member-card-expanded" : ""}`.trim()}
              >
                <button
                  type="button"
                  className={`member-toggle ${isExpanded ? "expanded" : ""}`.trim()}
                  onClick={() => {
                    if (isExpanded) {
                      setExpandedGroupKeys((previous) =>
                        previous.filter((existingGroupKey) => existingGroupKey !== group.key),
                      );
                      if (hasSelectedMember) {
                        onClearSelectedItem();
                      }
                      return;
                    }

                    setExpandedGroupKeys((previous) => {
                      if (previous.includes(group.key)) {
                        return previous;
                      }
                      return [...previous, group.key];
                    });
                    onSelectItem(routeItemUUID);
                  }}
                  aria-expanded={isExpanded}
                  aria-label={`${isExpanded ? "Collapse" : "Expand"} item ${representative.normalized_title || "(no title)"}`}
                >
                  <p className="member-head">{representative.normalized_title || "(no title)"}</p>
                  {isExpanded ? (
                    <ChevronDown className="member-toggle-icon" aria-hidden="true" />
                  ) : (
                    <ChevronRight className="member-toggle-icon" aria-hidden="true" />
                  )}
                </button>
                <p className="member-sub">
                  matched {formatDateTime(representative.matched_at)} • published {formatDateTime(representative.published_at)}
                  {decisionText ? (
                    <>
                      {" "}
                      • <span className="member-decision-inline">{decisionText}</span>
                    </>
                  ) : null}
                  {group.members.length > 1 ? (
                    <>
                      {" "}
                      • merged {group.members.length} items from {group.sourceCount} sources
                    </>
                  ) : null}
                </p>
                {isExpanded ? (
                  <>
                    {group.canonicalURL ? (
                      <a className="member-expanded-url" href={group.canonicalURL} target="_blank" rel="noreferrer">
                        {group.canonicalURL}
                      </a>
                    ) : null}
                    <article className="detail-item-content member-expanded-content">
                      {isPreviewLoading ? <p className="muted">Fetching reader preview...</p> : null}
                      {!isPreviewLoading && hasResolvedContent ? (
                        <div className="detail-item-content-body">
                          {resolvedParagraphs.map((paragraph, index) => (
                            <p
                              key={`${group.key}-paragraph-${index}`}
                              className="detail-item-content-text"
                            >
                              {paragraph}
                            </p>
                          ))}
                        </div>
                      ) : null}
                      {!isPreviewLoading && !hasResolvedContent ? <p className="muted">No content captured for this item.</p> : null}
                      {!isPreviewLoading && previewError && previewTexts.length === 0 ? (
                        <p className="muted">Reader preview unavailable. Showing captured content when available.</p>
                      ) : null}
                    </article>
                    {group.members.length > 1 ? (
                      <section className="member-merge-provenance">
                        <p className="member-merge-provenance-title">Deduped items</p>
                        <ul className="member-merge-provenance-list">
                          {group.members.map((groupMember) => {
                            const memberDecision = groupMember.dedup_decision
                              ? groupMember.dedup_decision.toLowerCase()
                              : "";
                            const isSelected = selectedItemUUID === groupMember.story_article_uuid;

                            return (
                              <li
                                key={groupMember.story_article_uuid}
                                className={`member-merge-provenance-row ${isSelected ? "is-selected" : ""}`.trim()}
                              >
                                <button
                                  type="button"
                                  className="member-merge-provenance-link"
                                  onClick={() => onSelectItem(groupMember.story_article_uuid)}
                                >
                                  {buildMemberSubtitle(groupMember)}
                                </button>
                                <p className="member-sub">
                                  matched {formatDateTime(groupMember.matched_at)} • published{" "}
                                  {formatDateTime(groupMember.published_at)}
                                  {memberDecision ? (
                                    <>
                                      {" "}
                                      • <span className="member-decision-inline">{memberDecision}</span>
                                    </>
                                  ) : null}
                                </p>
                              </li>
                            );
                          })}
                        </ul>
                      </section>
                    ) : null}
                  </>
                ) : null}
                {!isExpanded ? (
                  <p className="member-preview member-preview-collapsed">{buildMemberPreview(resolvedExpandedText)}</p>
                ) : null}
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
        {!selectedStoryUUID ? <p className="muted">Pick a story to inspect merged articles.</p> : null}
        {selectedStoryUUID && isLoading ? <p className="muted">Fetching story detail...</p> : null}
        {selectedStoryUUID && !isLoading && error ? <p className="muted">{error}</p> : null}

        {selectedStoryUUID && !isLoading && !error && detail ? renderStoryView() : null}
      </div>
    </aside>
  );
}
