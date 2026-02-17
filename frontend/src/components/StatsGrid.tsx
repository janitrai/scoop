import { formatCount } from "../lib/viewerFormat";
import type { StatsResponse } from "../types";

interface StatsGridProps {
  stats: StatsResponse | null;
}

export function StatsGrid({ stats }: StatsGridProps): JSX.Element {
  return (
    <section className="stats-grid">
      <article className="stat card">
        <p className="stat-label">Raw Arrivals</p>
        <p className="stat-value">{formatCount(stats?.raw_arrivals)}</p>
      </article>
      <article className="stat card">
        <p className="stat-label">Articles</p>
        <p className="stat-value">{formatCount(stats?.articles)}</p>
      </article>
      <article className="stat card">
        <p className="stat-label">Stories</p>
        <p className="stat-value">{formatCount(stats?.stories)}</p>
      </article>
      <article className="stat card">
        <p className="stat-label">Dedup Events</p>
        <p className="stat-value">{formatCount(stats?.dedup_events)}</p>
      </article>
    </section>
  );
}
