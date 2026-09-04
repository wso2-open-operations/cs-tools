// src/server/db/normalize.test.ts
import { describe, expect, it } from "vitest";
import { buildStatusNormalizer } from "./normalize";

const ALIASES = [
  { alias: "Re-Opened", canonical: "Reopened" },
  { alias: "On-Hold", canonical: "On Hold" },
];

describe("buildStatusNormalizer", () => {
  const normalize = buildStatusNormalizer(ALIASES);

  it("maps an alias to its canonical name", () => {
    expect(normalize("Re-Opened")).toBe("Reopened");
    expect(normalize("On-Hold")).toBe("On Hold");
  });

  it("trims whitespace around a non-alias status", () => {
    expect(normalize("  Open  ")).toBe("Open");
  });

  it("passes null through unchanged", () => {
    expect(normalize(null)).toBeNull();
  });

  it("passes the empty string through unchanged (does not fold into absence)", () => {
    expect(normalize("")).toBe("");
  });

  it("passes an unrecognized status through unchanged", () => {
    expect(normalize("Open")).toBe("Open");
  });
});
