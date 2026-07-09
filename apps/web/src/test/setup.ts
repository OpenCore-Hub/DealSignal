import "@testing-library/jest-dom/vitest";

// Mock window.matchMedia for jsdom (not implemented natively)
// Guard for node environment (pure logic tests without jsdom)
if (typeof window !== "undefined") {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });

  // Mock scrollTo for jsdom
  window.scrollTo = () => {};
}
