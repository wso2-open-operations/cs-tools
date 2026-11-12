import { describe, expect, it } from "vitest";
import { categories, skillGo, skillKube, skillOld } from "@features/csm-skills/test/fixtures";
import { allNodeIds, buildSkillTree, pathToSkill, UNCATEGORISED_ID } from "@features/csm-skills/utils/skillTree";

describe("buildSkillTree", () => {
  it("files skills under their categories, counts them up the tree, and drops empty branches", () => {
    const tree = buildSkillTree(categories, [skillGo, skillKube]);
    expect(tree.map((n) => n.name)).toEqual(["Engineering", "Operations"]);
    const eng = tree[0];
    expect(eng.total).toBe(1);
    expect(eng.skills).toEqual([]);
    expect(eng.children.map((c) => [c.name, c.skills.map((s) => s.name)])).toEqual([["Languages", ["Go"]]]);
    // Only Go: Operations has nothing, so it is not shown.
    expect(buildSkillTree(categories, [skillGo]).map((n) => n.name)).toEqual(["Engineering"]);
  });

  it("puts skills filed nowhere under Uncategorised, last", () => {
    const tree = buildSkillTree(categories, [skillKube, { ...skillOld, active: true }]);
    expect(tree.at(-1)?.id).toBe(UNCATEGORISED_ID);
    expect(tree.at(-1)?.skills.map((s) => s.name)).toEqual(["Legacy Tool"]);
  });

  it("finds the way to a skill, and lists every node", () => {
    const tree = buildSkillTree(categories, [skillGo, skillKube]);
    expect(pathToSkill(tree, "skill-go")).toEqual(["cat-eng", "cat-lang"]);
    expect(pathToSkill(tree, "nope")).toBeNull();
    expect(allNodeIds(tree).sort()).toEqual(["cat-eng", "cat-lang", "cat-ops"]);
  });
});
