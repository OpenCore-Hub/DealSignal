// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { SupportedLanguage } from "./config";

let currentLanguage: SupportedLanguage = "en";

const changeLanguageMock = vi.fn((lng: SupportedLanguage) => {
  currentLanguage = lng;
});

vi.mock("./config", () => ({
  default: {
    get language() {
      return currentLanguage;
    },
    changeLanguage: (lng: SupportedLanguage) => changeLanguageMock(lng),
  },
  isSupportedLanguage: (lng: string) => lng === "en" || lng === "zh-CN",
}));

import {
  getCurrentLanguage,
  setLanguage,
  toggleLanguage,
  updateDocumentLanguage,
  updatePageTitle,
} from "./utils";

describe("i18n utils", () => {
  beforeEach(() => {
    currentLanguage = "en";
    document.documentElement.lang = "";
    document.title = "";
    vi.clearAllMocks();
  });

  it("getCurrentLanguage returns current i18n language when supported", () => {
    currentLanguage = "zh-CN";
    expect(getCurrentLanguage()).toBe("zh-CN");
  });

  it("getCurrentLanguage falls back to en for unsupported language", () => {
    currentLanguage = "fr" as SupportedLanguage;
    expect(getCurrentLanguage()).toBe("en");
  });

  it("setLanguage calls i18n.changeLanguage", () => {
    setLanguage("zh-CN");
    expect(changeLanguageMock).toHaveBeenCalledWith("zh-CN");
  });

  it("toggleLanguage switches from en to zh-CN", () => {
    currentLanguage = "en";
    toggleLanguage();
    expect(changeLanguageMock).toHaveBeenCalledWith("zh-CN");
  });

  it("toggleLanguage switches from zh-CN to en", () => {
    currentLanguage = "zh-CN";
    toggleLanguage();
    expect(changeLanguageMock).toHaveBeenCalledWith("en");
  });

  it("updateDocumentLanguage sets html lang for supported language", () => {
    updateDocumentLanguage("zh-CN");
    expect(document.documentElement.lang).toBe("zh-CN");
  });

  it("updateDocumentLanguage falls back to zh-CN for unsupported language", () => {
    updateDocumentLanguage("fr");
    expect(document.documentElement.lang).toBe("zh-CN");
  });

  it("updatePageTitle sets document title", () => {
    updatePageTitle("Test Title");
    expect(document.title).toBe("Test Title");
  });

  it("updatePageTitle defaults to DealSignal", () => {
    updatePageTitle();
    expect(document.title).toBe("DealSignal");
  });
});
