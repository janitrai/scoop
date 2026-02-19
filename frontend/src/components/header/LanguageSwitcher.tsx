import { Globe } from "lucide-react";

import { Select, SelectContent, SelectItem, SelectTrigger } from "../ui/select";

export type ViewerLanguage = "original" | "en" | "zh";

interface LanguageSwitcherProps {
  value: ViewerLanguage;
  onChange: (value: ViewerLanguage) => void;
}

const LANGUAGE_LABEL: Record<ViewerLanguage, string> = {
  original: "Original",
  en: "English",
  zh: "Chinese",
};

export function LanguageSwitcher({ value, onChange }: LanguageSwitcherProps): JSX.Element {
  return (
    <div className="lang-switcher">
      <Select
        value={value}
        onValueChange={(nextValue) => {
          onChange(nextValue as ViewerLanguage);
        }}
      >
        <SelectTrigger variant="default" className="lang-select-trigger" aria-label="Content language">
          <span className="lang-select-label">
            <Globe className="lang-select-icon" aria-hidden="true" />
            <span>{LANGUAGE_LABEL[value]}</span>
          </span>
        </SelectTrigger>
        <SelectContent className="lang-select-content">
          <SelectItem value="original">Original</SelectItem>
          <SelectItem value="en">English</SelectItem>
          <SelectItem value="zh">Chinese</SelectItem>
        </SelectContent>
      </Select>
    </div>
  );
}
