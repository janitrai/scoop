import { Select, SelectContent, SelectItem, SelectTrigger } from "../ui/select";

import { formatCount } from "../../lib/viewerFormat";
import type { CollectionSummary } from "../../types";

interface CollectionDropdownProps {
  selectedCollection: string;
  allCollectionsValue: string;
  allCollectionsLabel: string;
  currentCollectionLabel: string;
  collections: CollectionSummary[];
  onCollectionChange: (value: string) => void;
}

export function CollectionDropdown({
  selectedCollection,
  allCollectionsValue,
  allCollectionsLabel,
  currentCollectionLabel,
  collections,
  onCollectionChange,
}: CollectionDropdownProps): JSX.Element {
  return (
    <div className="brand-select">
      <Select
        value={selectedCollection || allCollectionsValue}
        onValueChange={(value) => onCollectionChange(value === allCollectionsValue ? "" : value)}
      >
        <SelectTrigger
          variant="ghost"
          className="brand-select-trigger"
          aria-label={`Collection filter: ${currentCollectionLabel}`}
        >
          <div className="brand-select-label">
            <span className="brand-select-prefix">SCOOP</span>
            <span className="brand-select-separator-dot" aria-hidden="true" />
            <span className="brand-select-current">{currentCollectionLabel}</span>
          </div>
        </SelectTrigger>
        <SelectContent className="collection-select-content">
          <SelectItem value={allCollectionsValue}>{allCollectionsLabel}</SelectItem>
          {collections.map((row) => (
            <SelectItem key={row.collection} value={row.collection}>
              {row.collection} ({formatCount(row.stories)})
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
