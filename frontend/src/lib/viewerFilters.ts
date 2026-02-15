import type { StoryFilters } from "../types";

interface BuildStoryFiltersArgs {
  baseFilters: StoryFilters;
  routeCollection: string;
  showAdvancedSearch: boolean;
  day: string;
}

export function buildStoryFilters({
  baseFilters,
  routeCollection,
  showAdvancedSearch,
  day,
}: BuildStoryFiltersArgs): StoryFilters {
  const activeCollection = routeCollection || baseFilters.collection;
  if (showAdvancedSearch) {
    return {
      ...baseFilters,
      collection: activeCollection,
    };
  }

  const hasQuery = baseFilters.query.trim() !== "";
  return {
    ...baseFilters,
    collection: activeCollection,
    from: hasQuery ? "" : day,
    to: hasQuery ? "" : day,
  };
}
