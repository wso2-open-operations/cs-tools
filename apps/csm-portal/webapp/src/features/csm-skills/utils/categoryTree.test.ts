import { describe, expect, it } from "vitest";
import { buildCategoryTree, flattenCategoryTree } from "@features/csm-skills/utils/categoryTree";
import type { SkillCategory } from "@features/csm-skills/types";

const cat = (id: string, name: string, parentId: string | null = null): SkillCategory => ({
  id,
  name,
  parentId,
  isLowestCategory: false,
});

describe("buildCategoryTree / flattenCategoryTree", () => {
  it("nests children under their parent, sorted by name, in depth-first order", () => {
    const flat = flattenCategoryTree(
      buildCategoryTree([cat("c", "Zeta"), cat("b", "Beta", "a"), cat("a", "Alpha"), cat("d", "Delta", "a")]),
    );
    expect(flat.map((f) => [f.category.name, f.depth])).toEqual([
      ["Alpha", 0],
      ["Beta", 1],
      ["Delta", 1],
      ["Zeta", 0],
    ]);
  });

  it("treats a category whose parent is missing as a root", () => {
    const flat = flattenCategoryTree(buildCategoryTree([cat("x", "Orphan", "gone")]));
    expect(flat).toHaveLength(1);
    expect(flat[0].depth).toBe(0);
  });

  it("keeps every category even if parents form a cycle", () => {
    const flat = flattenCategoryTree(buildCategoryTree([cat("a", "A", "b"), cat("b", "B", "a")]));
    expect(flat.map((f) => f.category.id).sort()).toEqual(["a", "b"]);
  });
});
