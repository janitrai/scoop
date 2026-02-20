export function normalizeLanguageTag(value: string): string {
  const trimmed = value.trim().toLowerCase().replaceAll("_", "-");
  if (!trimmed) {
    return "";
  }

  const parts = trimmed.split("-").filter(Boolean);
  if (parts.length === 0) {
    return "";
  }
  if (parts.some((part) => !/^[a-z]+$/.test(part))) {
    return "";
  }
  return parts.join("-");
}

export function normalizeLanguageCode(value: string): string {
  const normalized = normalizeLanguageTag(value);
  if (!normalized) {
    return "";
  }
  return normalized.split("-")[0] ?? "";
}
