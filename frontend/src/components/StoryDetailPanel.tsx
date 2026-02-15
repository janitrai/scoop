import { useEffect, useMemo, useState } from "react";
import { ArrowLeft } from "lucide-react";

import { buildMemberSubtitle, formatDateTime } from "../lib/viewerFormat";
import type { StoryDetailResponse } from "../types";
import { Button } from "./ui/button";

interface StoryDetailPanelProps {
  selectedStoryUUID: string;
  selectedStoryVisible: boolean;
  detail: StoryDetailResponse | null;
  isLoading: boolean;
  error: string;
}

export function StoryDetailPanel({
  selectedStoryUUID,
  selectedStoryVisible,
  detail,
  isLoading,
  error,
}: StoryDetailPanelProps): JSX.Element {
  const [focusedMemberUUID, setFocusedMemberUUID] = useState("");

  useEffect(() => {
    setFocusedMemberUUID("");
  }, [selectedStoryUUID]);

  useEffect(() => {
    if (!focusedMemberUUID || !detail) {
      return;
    }

    const stillExists = detail.members.some((member) => member.story_member_uuid === focusedMemberUUID);
    if (!stillExists) {
      setFocusedMemberUUID("");
    }
  }, [detail, focusedMemberUUID]);

  const focusedMember = useMemo(() => {
    if (!detail || !focusedMemberUUID) {
      return null;
    }
    return detail.members.find((member) => member.story_member_uuid === focusedMemberUUID) ?? null;
  }, [detail, focusedMemberUUID]);

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

  function renderStoryView(): JSX.Element {
    if (!detail) {
      return <></>;
    }

    return (
      <>
        <h2 className="detail-title">{detail.story.title || "(untitled)"}</h2>
        <p className="detail-meta">
          Collection: {detail.story.collection} • {detail.story.item_count} items • {detail.story.source_count} sources
        </p>
        {detail.story.canonical_url ? (
          <a className="detail-url" href={detail.story.canonical_url} target="_blank" rel="noreferrer">
            {detail.story.canonical_url}
          </a>
        ) : null}
        <p className="detail-meta">
          first seen {formatDateTime(detail.story.first_seen_at)} • last seen {formatDateTime(detail.story.last_seen_at)}
        </p>
        <section className="member-grid">
          {detail.members.length === 0 ? <p className="muted">No items found for this story.</p> : null}
          {detail.members.map((member) => (
            <article
              key={member.story_member_uuid}
              className="member-card member-card-clickable"
              role="button"
              tabIndex={0}
              onClick={() => setFocusedMemberUUID(member.story_member_uuid)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  setFocusedMemberUUID(member.story_member_uuid);
                }
              }}
            >
              <p className="member-head">{member.normalized_title || "(no title)"}</p>
              <p className="member-preview">{buildMemberPreview(member.normalized_text)}</p>
              <p className="member-sub">{buildMemberSubtitle(member)}</p>
              <p className="member-sub">
                matched {formatDateTime(member.matched_at)} • published {formatDateTime(member.published_at)}
              </p>
              {member.dedup_decision ? (
                <span className={`decision-pill decision-${member.dedup_decision}`}>{member.dedup_decision}</span>
              ) : null}
            </article>
          ))}
        </section>
      </>
    );
  }

  function renderFocusedItemView(): JSX.Element {
    if (!focusedMember) {
      return <></>;
    }

    const content = focusedMember.normalized_text?.trim() ?? "";

    return (
      <>
        <div className="detail-item-nav">
          <Button
            type="button"
            variant="ghost"
            className="detail-back-btn"
            onClick={() => setFocusedMemberUUID("")}
          >
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
            Back to story
          </Button>
        </div>
        <h2 className="detail-title">{focusedMember.normalized_title || "(no title)"}</h2>
        <p className="detail-meta">Collection: {focusedMember.collection}</p>
        <p className="detail-meta">{buildMemberSubtitle(focusedMember)}</p>
        <p className="detail-meta">
          matched {formatDateTime(focusedMember.matched_at)} • published {formatDateTime(focusedMember.published_at)}
        </p>
        {focusedMember.canonical_url ? (
          <a className="detail-url" href={focusedMember.canonical_url} target="_blank" rel="noreferrer">
            {focusedMember.canonical_url}
          </a>
        ) : null}
        <article className="detail-item-content">
          {content ? <p className="detail-item-content-text">{content}</p> : <p className="muted">No content captured for this item.</p>}
        </article>
      </>
    );
  }

  return (
    <aside className="panel card detail-panel">
      <div className="detail-content">
        {!selectedStoryUUID ? <p className="muted">Pick a story to inspect merged documents.</p> : null}
        {selectedStoryUUID && !selectedStoryVisible ? <p className="muted">Selected story is not on the current page.</p> : null}
        {selectedStoryUUID && isLoading ? <p className="muted">Fetching story detail...</p> : null}
        {selectedStoryUUID && !isLoading && error ? <p className="muted">{error}</p> : null}

        {selectedStoryUUID && !isLoading && !error && detail ? (
          focusedMember ? renderFocusedItemView() : renderStoryView()
        ) : null}
      </div>
    </aside>
  );
}
