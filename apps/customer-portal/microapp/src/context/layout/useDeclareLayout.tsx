import { type DependencyList, useContext, useLayoutEffect } from "react";

import { LayoutContext, type LayoutDeclaration } from "./LayoutContext";

/**
 * Declares a layout configuration for the current component.
 *
 * **Dependency tracking is the caller's responsibility.**
 * This hook intentionally does not infer deps — if any field in `config` is
 * derived from props, state, or context, you *must* pass them explicitly in
 * `deps`, otherwise the layout will only be declared on mount and will silently
 * stale when those values change.
 *
 * @example
 * // ✅ Static config — omitting deps is fine
 * useDeclareLayout({ title: "Settings" });
 *
 * @example
 * // ✅ Dynamic config — deps required
 * useDeclareLayout({ title: caseTitle }, { enabled: !!caseTitle }, [caseTitle]);
 *
 * @example
 * // ❌ Dynamic config with no deps — title will never update after mount
 * useDeclareLayout({ title: caseTitle });
 */
export const useDeclareLayout = (
  config: Partial<LayoutDeclaration>,
  options: { enabled?: boolean } = { enabled: true },
  deps?: DependencyList,
) => {
  const { declareLayout } = useContext(LayoutContext);

  useLayoutEffect(() => {
    if (!options.enabled) return;
    declareLayout(config);
  }, deps ?? []);
};
