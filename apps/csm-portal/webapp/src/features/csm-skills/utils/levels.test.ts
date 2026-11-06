import { describe, expect, it } from "vitest";
import { distinctLevelValues, levelDotColor, sortedLevels } from "@features/csm-skills/utils/levels";
import { levelTypes } from "@features/csm-skills/test/fixtures";

describe("levels helpers", () => {
  it("sorts a scale by value, valueless levels last", () => {
    const sorted = sortedLevels({
      id: "t",
      name: "T",
      levels: [
        { id: "3", name: "No value" },
        { id: "2", name: "High", value: 9 },
        { id: "1", name: "Low", value: 1 },
      ],
    });
    expect(sorted.map((l) => l.name)).toEqual(["Low", "High", "No value"]);
  });

  it("accepts only plain hex colours as a dot colour", () => {
    expect(levelDotColor({ color: "#2e7d32" })).toBe("#2e7d32");
    expect(levelDotColor({ color: "red" })).toBeUndefined();
    expect(levelDotColor({ color: "url(javascript:1)" })).toBeUndefined();
    expect(levelDotColor(null)).toBeUndefined();
  });

  it("lists each distinct value across scales once, ascending", () => {
    expect(distinctLevelValues(levelTypes).map((v) => v.value)).toEqual([1, 2, 20, 50, 100]);
  });
});
