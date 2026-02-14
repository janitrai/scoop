import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { getStats } from "../api";
import { AppHeader } from "../components/AppHeader";
import { StatsGrid } from "../components/StatsGrid";
import { extractErrorMessage, formatDateTime, formatCount } from "../lib/viewerFormat";

export function StatsPage(): JSX.Element {
  const [refreshTick, setRefreshTick] = useState(0);

  const statsQuery = useQuery({
    queryKey: ["stats", refreshTick],
    queryFn: getStats,
  });

  const stats = statsQuery.data ?? null;
  const errorText = statsQuery.error ? extractErrorMessage(statsQuery.error) : "";
  const updatedText = stats?.last_story_updated
    ? `Last story update: ${formatDateTime(stats.last_story_updated)}`
    : "No story updates yet";

  return (
    <div className="app-root">
      <AppHeader title="Pipeline Stats" activeTab="stats" rightText={updatedText} />

      <section className="card stats-toolbar">
        <p className="muted">System-wide ingestion and deduplication counters.</p>
        <button type="button" className="btn" onClick={() => setRefreshTick((tick) => tick + 1)}>
          Refresh
        </button>
      </section>

      {errorText ? <p className="banner-error">{errorText}</p> : null}

      <StatsGrid stats={stats} />

      <section className="card stats-secondary-grid">
        <article className="stat card">
          <p className="stat-label">Story Members</p>
          <p className="stat-value">{formatCount(stats?.story_members)}</p>
        </article>
        <article className="stat card">
          <p className="stat-label">Ingest Runs (Running)</p>
          <p className="stat-value">{formatCount(stats?.running_ingest_runs)}</p>
        </article>
      </section>
    </div>
  );
}
