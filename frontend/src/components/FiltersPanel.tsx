import { useState } from "react";
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
  onDayPick: (day: string) => void;
}

export function FiltersPanel({
  searchInput,
  from,
  to,
  activeCollection,
  allStoriesCount,
  totalItems,
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
  onDayPick,
}: FiltersPanelProps): JSX.Element {
  const [showAdvancedSearch, setShowAdvancedSearch] = useState(false);

  return (
    <section className="controls card">
      <div className="controls-top-row">
        <div className="day-strip">
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
              <span className="day-current-line">
                {dayNav.currentLabel} • {dayNav.relativeLabel}
              </span>
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
        </div>

        <div className="search-strip">
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
              className={`btn btn-subtle finder-advanced-btn ${showAdvancedSearch ? "active" : ""}`.trim()}
              type="button"
              onClick={() => setShowAdvancedSearch((value) => !value)}
            >
              Advanced
            </button>

            <button className="btn" type="button" onClick={onRefresh}>
              Refresh
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
    </section>
  );
}
