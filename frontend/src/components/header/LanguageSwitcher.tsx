import { Globe } from "lucide-react";

import type { LanguageOption } from "../../types";
import { Select, SelectContent, SelectItem, SelectTrigger } from "../ui/select";

interface LanguageSwitcherProps {
  value: string;
  options: LanguageOption[];
  onChange: (value: string) => void;
}

function resolveLabel(value: string, options: LanguageOption[]): string {
  const normalized = value.trim().toLowerCase();
  const match = options.find((option) => option.code.trim().toLowerCase() === normalized);
  if (match) {
    return match.label;
  }
  return normalized ? normalized.toUpperCase() : "Original";
}

export function LanguageSwitcher({ value, options, onChange }: LanguageSwitcherProps): JSX.Element {
  return (
    <div className="lang-switcher">
      <Select
        value={value}
        onValueChange={(nextValue) => {
          onChange(nextValue);
        }}
      >
        <SelectTrigger variant="default" className="lang-select-trigger" aria-label="Content language">
          <span className="lang-select-label">
            <Globe className="lang-select-icon" aria-hidden="true" style={{ flexShrink: 0 }} />
            {resolveLabel(value, options)}
          </span>
        </SelectTrigger>
        <SelectContent className="lang-select-content">
          {options.map((option) => (
            <SelectItem key={option.code} value={option.code}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
