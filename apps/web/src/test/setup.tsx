/// <reference types="vitest/globals" />
import "@testing-library/jest-dom/vitest";

/** Default workspace role for unit tests: member (can write, cannot manage). */
vi.mock("@/hooks/useWorkspaceAccess", () => ({
  useWorkspaceAccess: vi.fn(() => ({
    role: "member",
    loading: false,
    canRead: true,
    canWrite: true,
    canManage: false,
    isGuest: false,
  })),
}));

// Mock @base-ui/react/menu for jsdom — Base UI Menu relies on pointer/positioning
// APIs that are flaky or absent under CI jsdom (portal content never opens).
vi.mock("@base-ui/react/menu", async () => {
  const React = await import("react");
  const MenuCtx = React.createContext<{
    open: boolean;
    setOpen: (v: boolean) => void;
  } | null>(null);

  const Root = ({
    children,
    open: openProp,
    defaultOpen,
    onOpenChange,
    ...props
  }: {
    children?: React.ReactNode;
    open?: boolean;
    defaultOpen?: boolean;
    onOpenChange?: (open: boolean) => void;
  } & Record<string, unknown>) => {
    const [uncontrolledOpen, setUncontrolledOpen] = React.useState(
      Boolean(defaultOpen),
    );
    const controlled = openProp !== undefined;
    const open = controlled ? Boolean(openProp) : uncontrolledOpen;
    const setOpen = (next: boolean) => {
      if (!controlled) setUncontrolledOpen(next);
      onOpenChange?.(next);
    };
    return (
      <MenuCtx.Provider value={{ open, setOpen }}>
        <div data-slot="dropdown-menu" {...props}>
          {children}
        </div>
      </MenuCtx.Provider>
    );
  };

  const Trigger = ({
    children,
    render,
    onClick: userOnClick,
    ...props
  }: {
    children?: React.ReactNode;
    // Base UI accepts either an element or a props→element render function.
    render?:
      | React.ReactElement
      | ((props: Record<string, unknown>) => React.ReactNode);
    onClick?: (e: React.MouseEvent) => void;
  } & Record<string, unknown>) => {
    const ctx = React.useContext(MenuCtx);
    const onClick = (e: React.MouseEvent) => {
      ctx?.setOpen(!(ctx?.open ?? false));
      userOnClick?.(e);
    };
    const merged = {
      ...props,
      "data-slot": "dropdown-menu-trigger",
      onClick,
    };
    if (typeof render === "function") {
      return <>{render(merged)}</>;
    }
    if (React.isValidElement(render)) {
      return React.cloneElement(render as React.ReactElement<Record<string, unknown>>, merged);
    }
    return (
      <button type="button" {...merged}>
        {children}
      </button>
    );
  };

  const Portal = ({ children }: { children?: React.ReactNode }) => <>{children}</>;
  const Positioner = ({
    children,
    ...props
  }: { children?: React.ReactNode } & Record<string, unknown>) => (
    <div data-slot="dropdown-menu-positioner" {...props}>
      {children}
    </div>
  );
  const Popup = ({
    children,
    ...props
  }: { children?: React.ReactNode } & Record<string, unknown>) => {
    const ctx = React.useContext(MenuCtx);
    if (!ctx?.open) return null;
    return (
      <div data-slot="dropdown-menu-content" role="menu" {...props}>
        {children}
      </div>
    );
  };
  const Item = ({
    children,
    disabled,
    onClick,
    ...props
  }: {
    children?: React.ReactNode;
    disabled?: boolean;
    onClick?: (e: React.MouseEvent) => void;
  } & Record<string, unknown>) => {
    const ctx = React.useContext(MenuCtx);
    return (
      <button
        type="button"
        role="menuitem"
        data-slot="dropdown-menu-item"
        {...props}
        disabled={Boolean(disabled)}
        {...(disabled ? { "data-disabled": "" } : {})}
        onClick={(e) => {
          if (disabled) return;
          onClick?.(e);
          ctx?.setOpen(false);
        }}
      >
        {children}
      </button>
    );
  };
  const Group = ({
    children,
    ...props
  }: { children?: React.ReactNode } & Record<string, unknown>) => (
    <div data-slot="dropdown-menu-group" {...props}>
      {children}
    </div>
  );
  const GroupLabel = ({
    children,
    ...props
  }: { children?: React.ReactNode } & Record<string, unknown>) => (
    <div data-slot="dropdown-menu-label" {...props}>
      {children}
    </div>
  );
  const Separator = (props: Record<string, unknown>) => (
    <hr data-slot="dropdown-menu-separator" {...props} />
  );

  return {
    Menu: {
      Root,
      Trigger,
      Portal,
      Positioner,
      Popup,
      Item,
      Group,
      GroupLabel,
      Separator,
    },
  };
});

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

  // Mock sessionStorage (real Map-backed store) for login-email cache etc.
  const sessionStore: Record<string, string> = {};
  Object.defineProperty(window, "sessionStorage", {
    writable: true,
    value: {
      getItem: (key: string) => sessionStore[key] ?? null,
      setItem: (key: string, value: string) => {
        sessionStore[key] = value;
      },
      removeItem: (key: string) => {
        delete sessionStore[key];
      },
      clear: () => {
        for (const key of Object.keys(sessionStore)) {
          delete sessionStore[key];
        }
      },
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
