import { useLayoutEffect, useRef } from "react";

export const useAppBarHeight = () => {
  const ref = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    if (!ref.current) return;

    const observer = new ResizeObserver(([entry]) => {
      document.documentElement.style.setProperty("--app-bar-height", `${entry.contentRect.height}px`);
    });

    observer.observe(ref.current);
    return () => observer.disconnect();
  }, []);

  return { ref };
};
