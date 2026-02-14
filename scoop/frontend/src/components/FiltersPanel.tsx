import type { MutableRefObject } from "react";

import { formatCount } from "../lib/viewerFormat";
import type { CollectionSummary, DayNavigationState } from "../types";

interface FiltersPanelProps {
  searchInput: string;
  from: string;
  to: string;
  activeCollection: string;
  allStoriesCount: number;
  totalItems: number;
  hasDateFilter: boolean;
  collections: CollectionSummary[];
  dayNav: DayNavigationState;
  dayPickerRef: MutableRefObject<HTMLInputElement | null>;
  onSearchInputChange: (value: string) => void;
  onFromChange: (value: string) => void;
  onToChange: (value: string) => void;
  onRefresh: () => void;
  onCollectionChange: (collection: string) => void;
  onMoveOlderDay: () => void;
  onMoveNewerDay: () => void;
  onOpenDayPicker: () => void;
  onClearDays: () => void;
  onDayPick: (day: string) => void;
}

export function FiltersPanel({
  searchInput,
  from,
  to,
  activeCollection,
  allStoriesCount,
  totalItems,
  hasDateFilter,
  collections,
  dayNav,
  dayPickerRef,
  onSearchInputChange,
  onFromChange,
  onToChange,
  onRefresh,
  onCollectionChange,
  onMoveOlderDay,
  onMoveNewerDay,
  onOpenDayPicker,
  onClearDays,
  onDayPick,
}: FiltersPanelProps): JSX.Element {
  return (
    <section className="controls card">
      <div className="control-row">
        <label className="field">
          <span>Search</span>
          <input
            value={searchInput}
            onChange={(event) => onSearchInputChange(event.target.value)}
            type="text"
            placeholder="Title or canonical URL"
          />
        </label>

        <label className="field field-small">
          <span>From</span>
          <input value={from} onChange={(event) => onFromChange(event.target.value)} type="date" />
        </label>

        <label className="field field-small">
          <span>To</span>
          <input value={to} onChange={(event) => onToChange(event.target.value)} type="date" />
        </label>

        <button className="btn" type="button" onClick={onRefresh}>
          Refresh
        </button>
      </div>

      <div className="chips-row">
        <button
          type="button"
          className={`chip ${activeCollection === "" ? "active" : ""}`.trim()}
          onClick={() => onCollectionChange("")}
        >
          all ({formatCount(allStoriesCount || totalItems)} stories)
        </button>

        {collections.map((row) => (
          <button
            key={row.collection}
            type="button"
            className={`chip ${activeCollection === row.collection ? "active" : ""}`.trim()}
            onClick={() => onCollectionChange(row.collection)}
          >
            {row.collection} ({formatCount(row.stories)} stories)
          </button>
        ))}
      </div>

      <div className="day-strip">
        <p className="eyebrow day-eyebrow">Browse By Day</p>
        <div className="day-nav">
          <button
            type="button"
            className="btn btn-subtle day-nav-btn"
            aria-label="Older day"
            onClick={onMoveOlderDay}
            disabled={!dayNav.canGoOlder}
          >
            &larr;
          </button>

          <button type="button" className="day-current-btn" onClick={onOpenDayPicker}>
            {dayNav.currentLabel}
          </button>

          <button
            type="button"
            className="btn btn-subtle day-nav-btn"
            aria-label="Newer day"
            onClick={onMoveNewerDay}
            disabled={!dayNav.canGoNewer}
          >
            &rarr;
          </button>

          <button
            type="button"
            className={`chip day-clear-btn ${!hasDateFilter ? "active" : ""}`.trim()}
            onClick={onClearDays}
            disabled={!hasDateFilter}
          >
            All days
          </button>

          <input
            ref={(node) => {
              dayPickerRef.current = node;
            }}
            className="day-picker-input"
            type="date"
            aria-hidden="true"
            tabIndex={-1}
            onChange={(event) => {
              if (!event.target.value) {
                return;
              }
              onDayPick(event.target.value);
            }}
          />
        </div>
        <p className="day-relative">{dayNav.relativeLabel}</p>
      </div>
    </section>
  );
}
