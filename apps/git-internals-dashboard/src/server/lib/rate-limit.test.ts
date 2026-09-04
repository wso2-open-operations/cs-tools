// src/server/lib/rate-limit.test.ts
import { describe, expect, it } from "vitest";
import { RateLimiter } from "./rate-limit";

describe("RateLimiter", () => {
  it("allows up to `limit` calls per key within the window, then denies", () => {
    const rl = new RateLimiter(2, 60_000);
    expect(rl.allow("a")).toBe(true);
    expect(rl.allow("a")).toBe(true);
    expect(rl.allow("a")).toBe(false);
  });

  it("tracks keys independently", () => {
    const rl = new RateLimiter(1, 60_000);
    expect(rl.allow("a")).toBe(true);
    expect(rl.allow("b")).toBe(true);
    expect(rl.allow("a")).toBe(false);
    expect(rl.allow("b")).toBe(false);
  });

  it("resets a key's budget once its window elapses", () => {
    const rl = new RateLimiter(1, 10);
    expect(rl.allow("a")).toBe(true);
    expect(rl.allow("a")).toBe(false);
    return new Promise<void>((resolve) => {
      setTimeout(() => {
        expect(rl.allow("a")).toBe(true);
        resolve();
      }, 20);
    });
  });

  it("evicts the oldest key once maxKeys is exceeded", () => {
    const rl = new RateLimiter(1, 60_000, 2);
    expect(rl.allow("a")).toBe(true);
    expect(rl.allow("b")).toBe(true);
    expect(rl.allow("c")).toBe(true); // evicts "a"'s window
    expect(rl.allow("a")).toBe(true); // "a" is fresh again
  });
});
