import { buildMemberSubtitle, formatDateTime } from "../lib/viewerFormat";
import type { StoryDetailResponse } from "../types";

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
  return (
    <aside className="panel card detail-panel">
      <div className="detail-content">
        {!selectedStoryUUID ? <p className="muted">Pick a story to inspect merged documents.</p> : null}
        {selectedStoryUUID && !selectedStoryVisible ? <p className="muted">Selected story is not on the current page.</p> : null}
        {selectedStoryUUID && isLoading ? <p className="muted">Fetching story detail...</p> : null}
        {selectedStoryUUID && !isLoading && error ? <p className="muted">{error}</p> : null}

        {selectedStoryUUID && !isLoading && !error && detail ? (
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
              {detail.members.length === 0 ? <p className="muted">No members found for this story.</p> : null}
              {detail.members.map((member) => (
                <article key={member.story_member_uuid} className="member-card">
                  <p className="member-head">{member.normalized_title || "(no title)"}</p>
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
        ) : null}
      </div>
    </aside>
  );
}
