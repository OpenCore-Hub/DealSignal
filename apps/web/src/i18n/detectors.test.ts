// @vitest-environment jsdom
import { describe, it, expect, beforeEach, vi } from "vitest";
import {
  customLanguageDetector,
  detectBrowserLanguage,
  isPublicShareLinkPath,
  resolveInitialLanguage,
} from "./detectors";

const setLocation = (pathname: string, search = "") => {
  Object.defineProperty(window, "location", {
    value: { pathname, search },
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

describe("isPublicShareLinkPath", () => {
  it("matches share-link viewer routes", () => {
    expect(isPublicShareLinkPath("/l/abc123")).toBe(true);
    expect(isPublicShareLinkPath("/l/")).toBe(true);
  });

  it("does not match member app routes", () => {
    expect(isPublicShareLinkPath("/ws/documents")).toBe(false);
    expect(isPublicShareLinkPath("/viewer/doc-1")).toBe(false);
  });
});

describe("detectBrowserLanguage", () => {
  beforeEach(() => {
    Object.defineProperty(navigator, "language", {
      value: "en-US",
      configurable: true,
    });
    Object.defineProperty(navigator, "languages", {
      value: ["en-US"],
      configurable: true,
    });
  });

  it("returns zh-CN when any browser language is Chinese", () => {
    Object.defineProperty(navigator, "languages", {
      value: ["en-US", "zh-CN"],
      configurable: true,
    });
    expect(detectBrowserLanguage()).toBe("zh-CN");
  });

  it("returns en for non-Chinese browser languages", () => {
    Object.defineProperty(navigator, "languages", {
      value: ["fr-FR", "de-DE"],
      configurable: true,
    });
    expect(detectBrowserLanguage()).toBe("en");
  });
});

describe("resolveInitialLanguage", () => {
  beforeEach(() => {
    setLocation("/ws/documents");
    vi.stubGlobal("localStorage", createStorage());
    document.documentElement.lang = "";
    Object.defineProperty(navigator, "language", {
      value: "en-US",
      configurable: true,
    });
    Object.defineProperty(navigator, "languages", {
      value: ["en-US"],
      configurable: true,
    });
  });

  it("falls back to en when no source is set", () => {
    expect(resolveInitialLanguage("/ws/documents", "")).toBe("en");
  });

  it("detects language from query string", () => {
    expect(resolveInitialLanguage("/ws/documents", "?lng=zh-CN")).toBe("zh-CN");
  });

  it("query string en maps to en", () => {
    expect(resolveInitialLanguage("/ws/documents", "?lng=en")).toBe("en");
  });

  it("query string unknown maps to en", () => {
    expect(resolveInitialLanguage("/ws/documents", "?lng=fr")).toBe("en");
  });

  it("falls back to localStorage when no query string on member routes", () => {
    localStorage.setItem("i18nextLng", "zh-CN");
    expect(resolveInitialLanguage("/ws/documents", "")).toBe("zh-CN");
  });

  it("query string takes precedence over localStorage", () => {
    localStorage.setItem("i18nextLng", "zh-CN");
    expect(resolveInitialLanguage("/ws/documents", "?lng=en")).toBe("en");
  });

  it("falls back to navigator.language when no query string or localStorage", () => {
    Object.defineProperty(navigator, "language", {
      value: "zh-CN",
      configurable: true,
    });
    expect(resolveInitialLanguage("/ws/documents", "")).toBe("zh-CN");
  });

  it("falls back to htmlTag lang when no other source", () => {
    Object.defineProperty(navigator, "language", {
      value: "",
      configurable: true,
    });
    document.documentElement.lang = "zh-CN";
    expect(resolveInitialLanguage("/ws/documents", "")).toBe("zh-CN");
  });

  it("share links ignore localStorage and use browser Chinese", () => {
    localStorage.setItem("i18nextLng", "en");
    Object.defineProperty(navigator, "languages", {
      value: ["zh-CN", "en-US"],
      configurable: true,
    });
    expect(resolveInitialLanguage("/l/token-abc", "")).toBe("zh-CN");
  });

  it("share links default to en for non-Chinese browsers", () => {
    localStorage.setItem("i18nextLng", "zh-CN");
    Object.defineProperty(navigator, "languages", {
      value: ["fr-FR", "en-US"],
      configurable: true,
    });
    expect(resolveInitialLanguage("/l/token-abc", "")).toBe("en");
  });

  it("share links still honor explicit lng query override", () => {
    localStorage.setItem("i18nextLng", "en");
    Object.defineProperty(navigator, "languages", {
      value: ["zh-CN"],
      configurable: true,
    });
    expect(resolveInitialLanguage("/l/token-abc", "?lng=en")).toBe("en");
  });

  it("normalizes zh variants to zh-CN", () => {
    expect(resolveInitialLanguage("/ws/documents", "?lng=zh")).toBe("zh-CN");
  });

  it("normalizes en variants to en", () => {
    expect(resolveInitialLanguage("/ws/documents", "?lng=en-GB")).toBe("en");
  });
});

describe("customLanguageDetector", () => {
  beforeEach(() => {
    setLocation("/ws/documents");
    vi.stubGlobal("localStorage", createStorage());
    document.documentElement.lang = "";
    Object.defineProperty(navigator, "language", {
      value: "en-US",
      configurable: true,
    });
    Object.defineProperty(navigator, "languages", {
      value: ["en-US"],
      configurable: true,
    });
  });

  it("delegates detect to resolveInitialLanguage", () => {
    setLocation("/l/share-token");
    Object.defineProperty(navigator, "languages", {
      value: ["zh-TW"],
      configurable: true,
    });
    expect(customLanguageDetector.detect()).toBe("zh-CN");
  });

  it("caches user language to localStorage on member routes", () => {
    setLocation("/ws/documents");
    customLanguageDetector.cacheUserLanguage("zh-CN");
    expect(localStorage.getItem("i18nextLng")).toBe("zh-CN");
  });

  it("does not cache visitor locale on share links", () => {
    setLocation("/l/share-token");
    customLanguageDetector.cacheUserLanguage("zh-CN");
    expect(localStorage.getItem("i18nextLng")).toBeNull();
  });
});
