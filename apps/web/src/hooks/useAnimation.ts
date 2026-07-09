import { useEffect, useRef, type RefObject } from "react";

/**
 * Tracks mouse position relative to an element for spotlight effects.
 * Returns a ref to attach to the target element. Sets CSS custom
 * properties `--spotlight-x` and `--spotlight-y` on the element.
 */
export function useSpotlight<T extends HTMLElement>(): RefObject<T | null> {
  const ref = useRef<T | null>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const handleMove = (e: MouseEvent) => {
      const rect = el.getBoundingClientRect();
      const x = ((e.clientX - rect.left) / rect.width) * 100;
      const y = ((e.clientY - rect.top) / rect.height) * 100;
      el.style.setProperty("--spotlight-x", `${x}%`);
      el.style.setProperty("--spotlight-y", `${y}%`);
    };

    const handleLeave = () => {
      el.style.setProperty("--spotlight-x", "50%");
      el.style.setProperty("--spotlight-y", "50%");
    };

    el.addEventListener("mousemove", handleMove);
    el.addEventListener("mouseleave", handleLeave);
    return () => {
      el.removeEventListener("mousemove", handleMove);
      el.removeEventListener("mouseleave", handleLeave);
    };
  }, []);

  return ref;
}
