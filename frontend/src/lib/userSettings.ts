const STORAGE_KEY = "scoop.user_settings.v1";

const FEED_WIDTH_MIN = 10;
const FEED_WIDTH_MAX = 80;
const FEED_WIDTH_DEFAULT = 35;

interface ViewerSettings {
  desktopFeedWidthPct: number;
}

interface UserSettings {
  viewer: ViewerSettings;
}

const DEFAULT_SETTINGS: UserSettings = {
  viewer: {
    desktopFeedWidthPct: FEED_WIDTH_DEFAULT,
  },
};

function clampFeedWidth(value: number): number {
  if (!Number.isFinite(value)) {
    return FEED_WIDTH_DEFAULT;
  }
  return Math.min(FEED_WIDTH_MAX, Math.max(FEED_WIDTH_MIN, Math.round(value * 100) / 100));
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function normalizeSettings(input: unknown): UserSettings {
  if (!isObject(input)) {
    return DEFAULT_SETTINGS;
  }

  const viewerRaw = isObject(input.viewer) ? input.viewer : {};
  const widthRaw = viewerRaw.desktopFeedWidthPct;
  const widthValue = typeof widthRaw === "number" ? widthRaw : FEED_WIDTH_DEFAULT;

  return {
    viewer: {
      desktopFeedWidthPct: clampFeedWidth(widthValue),
    },
  };
}

function readStorage(): UserSettings {
  if (typeof window === "undefined") {
    return DEFAULT_SETTINGS;
  }

  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return DEFAULT_SETTINGS;
    }
    return normalizeSettings(JSON.parse(raw) as unknown);
  } catch {
    return DEFAULT_SETTINGS;
  }
}

function writeStorage(settings: UserSettings): void {
  if (typeof window === "undefined") {
    return;
  }

  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
  } catch {
    // Ignore storage write failures in private/incognito modes.
  }
}

export function getDesktopFeedWidthPct(): number {
  return readStorage().viewer.desktopFeedWidthPct;
}

export function setDesktopFeedWidthPct(value: number): void {
  const current = readStorage();
  const next: UserSettings = {
    viewer: {
      ...current.viewer,
      desktopFeedWidthPct: clampFeedWidth(value),
    },
  };
  writeStorage(next);
}

export function getDesktopFeedWidthBounds(): { min: number; max: number; defaultValue: number } {
  return {
    min: FEED_WIDTH_MIN,
    max: FEED_WIDTH_MAX,
    defaultValue: FEED_WIDTH_DEFAULT,
  };
}
