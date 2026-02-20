import { Outlet, createRootRoute, createRoute, createRouter } from "@tanstack/react-router";

import { StoryViewerPage } from "./App";
import { AuthGate } from "./components/AuthGate";
import { StatsPage } from "./pages/StatsPage";
import type { ViewerSearch } from "./types";
import { normalizeViewerSearch } from "./viewerSearch";

function RootLayout(): JSX.Element {
  return (
    <AuthGate>
      <Outlet />
    </AuthGate>
  );
}

function validateViewerSearch(search: Record<string, unknown>): ViewerSearch {
  return normalizeViewerSearch(search);
}

const rootRoute = createRootRoute({
  component: RootLayout,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  validateSearch: validateViewerSearch,
  component: StoryViewerPage,
});

const storiesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/stories",
  validateSearch: validateViewerSearch,
  component: StoryViewerPage,
});

const storyDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/stories/$storyUUID",
  validateSearch: validateViewerSearch,
  component: StoryViewerPage,
});

const collectionRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/c/$collection",
  validateSearch: validateViewerSearch,
  component: StoryViewerPage,
});

const collectionStoryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/c/$collection/s/$storyUUID",
  validateSearch: validateViewerSearch,
  component: StoryViewerPage,
});

const collectionStoryItemRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/c/$collection/s/$storyUUID/i/$itemUUID",
  validateSearch: validateViewerSearch,
  component: StoryViewerPage,
});

const statsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/stats",
  component: StatsPage,
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  storiesRoute,
  storyDetailRoute,
  collectionRoute,
  collectionStoryRoute,
  collectionStoryItemRoute,
  statsRoute,
]);

export const router = createRouter({
  routeTree,
  defaultPreload: "intent",
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
