// src/server/lib/rate-limit.ts
// Minimal in-process fixed-window rate limiter, keyed by caller identity.
// Process-local by design, same caveat as TtlCache: under Choreo autoscaling
// each replica enforces its own independent budget, so the effective limit
// scales with replica count rather than being a hard global cap. Still
// bounds the realistic threat here — one client/session hammering a route —
// without needing a shared store for a handful of low-traffic internal routes.

interface Window {
  count: number;
  resetAt: number;
}

export class RateLimiter {
  private hits = new Map<string, Window>();

  constructor(
    private limit: number,
    private windowMs: number,
    private maxKeys = 5000,
  ) {}

  /** Records a call under `key`; returns false once `key` is over budget for the current window. */
  allow(key: string): boolean {
    const now = Date.now();
    const w = this.hits.get(key);
    if (!w || now > w.resetAt) {
      if (this.hits.size >= this.maxKeys) {
        const oldest = this.hits.keys().next().value;
        if (oldest !== undefined) this.hits.delete(oldest);
      }
      this.hits.set(key, { count: 1, resetAt: now + this.windowMs });
      return true;
    }
    if (w.count >= this.limit) return false;
    w.count++;
    return true;
  }
}
