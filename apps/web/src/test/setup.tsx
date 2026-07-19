/// <reference types="vitest/globals" />
import "@testing-library/jest-dom/vitest";

// Mock @base-ui/react/switch for jsdom — the real component depends on browser
// APIs (PointerEvent, ResizeObserver) that are not available in jsdom and
// silently fails to render role="switch" elements.
vi.mock("@base-ui/react/switch", () => ({
  Switch: {
    Root: ({
      checked,
      onCheckedChange,
      "aria-label": ariaLabel,
      "aria-labelledby": ariaLabelledby,
      disabled,
      className,
      children,
      ...props
    }: Record<string, unknown>) => {
      const Tag = "button";
      return (
        <Tag
          type="button"
          role="switch"
          aria-checked={checked as boolean}
          aria-label={ariaLabel as string}
          aria-labelledby={ariaLabelledby as string}
          disabled={disabled as boolean}
          className={className as string}
          onClick={() => {
            if (!disabled) {
              (onCheckedChange as (v: boolean) => void)?.(!(checked as boolean));
            }
          }}
          {...props}
        >
          <span>{children as React.ReactNode}</span>
        </Tag>
      );
    },
    Thumb: ({ className, ...props }: Record<string, unknown>) => (
      <span className={className as string} {...props} />
    ),
  },
}));

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

  // Mock localStorage for zustand persist middleware in jsdom.
  const store: Record<string, string> = {};
  Object.defineProperty(window, "localStorage", {
    writable: true,
    value: {
      getItem: (key: string) => store[key] ?? null,
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
      length: 0,
      key: () => null,
    },
  });

  // Mock sessionStorage too so auth/session-dependent code behaves consistently.
  Object.defineProperty(window, "sessionStorage", {
    writable: true,
    value: {
      getItem: () => null,
      setItem: () => {},
      removeItem: () => {},
      clear: () => {},
      length: 0,
      key: () => null,
    },
  });

  // Mock ResizeObserver for @base-ui/react and @radix-ui components
  // Must be a constructor function, not a factory.
  window.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;

  // Mock PointerEvent for @base-ui/react (not available in jsdom)
  if (typeof PointerEvent === "undefined") {
    (window as unknown as Record<string, unknown>).PointerEvent = MouseEvent;
  }
}
