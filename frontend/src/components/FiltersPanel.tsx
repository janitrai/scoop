import { formatCount } from "../lib/viewerFormat";
import type { CollectionSummary } from "../types";

interface FiltersPanelProps {
  activeCollection: string;
  allStoriesCount: number;
  totalItems: number;
  collections: CollectionSummary[];
  onCollectionChange: (collection: string) => void;
}

export function FiltersPanel({
  activeCollection,
  allStoriesCount,
  totalItems,
  collections,
  onCollectionChange,
}: FiltersPanelProps): JSX.Element {
  return (
    <section className="controls card">
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
