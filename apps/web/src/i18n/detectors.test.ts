// @vitest-environment jsdom
import { describe, it, expect, beforeEach, vi } from "vitest";
import { customLanguageDetector } from "./detectors";

const setSearch = (search: string) => {
  Object.defineProperty(window, "location", {
    value: { search },
    configurable: true,
  });
};

function createStorage() {
  const store: Record<string, string> = {};
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = value;
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      for (const key of Object.keys(store)) {
        delete store[key];
      }
    },
  };
}

describe("customLanguageDetector", () => {
  beforeEach(() => {
    setSearch("");
    vi.stubGlobal("localStorage", createStorage());
    document.documentElement.lang = "";
    Object.defineProperty(navigator, "language", {
      value: "en-US",
      configurable: true,
    });
  });

  it("falls back to en when no source is set", () => {
    expect(customLanguageDetector.detect()).toBe("en");
  });

  it("detects language from query string", () => {
    setSearch("?lng=zh-CN");
    expect(customLanguageDetector.detect()).toBe("zh-CN");
  });

  it("query string en maps to en", () => {
    setSearch("?lng=en");
    expect(customLanguageDetector.detect()).toBe("en");
  });

  it("query string unknown maps to en", () => {
    setSearch("?lng=fr");
    expect(customLanguageDetector.detect()).toBe("en");
  });

  it("falls back to localStorage when no query string", () => {
    localStorage.setItem("i18nextLng", "zh-CN");
    expect(customLanguageDetector.detect()).toBe("zh-CN");
  });

  it("query string takes precedence over localStorage", () => {
    localStorage.setItem("i18nextLng", "zh-CN");
    setSearch("?lng=en");
    expect(customLanguageDetector.detect()).toBe("en");
  });

  it("falls back to navigator.language when no query string or localStorage", () => {
    Object.defineProperty(navigator, "language", {
      value: "zh-CN",
      configurable: true,
    });
    expect(customLanguageDetector.detect()).toBe("zh-CN");
  });

  it("falls back to htmlTag lang when no other source", () => {
    Object.defineProperty(navigator, "language", {
      value: "",
      configurable: true,
    });
    document.documentElement.lang = "zh-CN";
    expect(customLanguageDetector.detect()).toBe("zh-CN");
  });

  it("normalizes zh variants to zh-CN", () => {
    setSearch("?lng=zh");
    expect(customLanguageDetector.detect()).toBe("zh-CN");
  });

  it("normalizes en variants to en", () => {
    setSearch("?lng=en-GB");
    expect(customLanguageDetector.detect()).toBe("en");
  });

  it("caches user language to localStorage", () => {
    customLanguageDetector.cacheUserLanguage("zh-CN");
    expect(localStorage.getItem("i18nextLng")).toBe("zh-CN");
  });
});
