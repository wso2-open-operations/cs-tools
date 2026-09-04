// src/server/lib/ttl-cache.ts
// Minimal in-process TTL cache with a max-entry bound (evicts oldest first).
// Process-local by design: correct for the values cached here (issue titles,
// overview/timeseries responses) where a few seconds/minutes of staleness or
// a cache miss on the "wrong" replica is harmless — never used for anything
// requiring cross-replica consistency (see jobs/lock.ts for that guarantee).

interface Entry<V> {
  value: V;
  expiresAt: number;
}

export class TtlCache<K, V> {
  private map = new Map<K, Entry<V>>();

  constructor(
    private ttlMs: number,
    private maxEntries = 1000,
  ) {}

  get(key: K): V | undefined {
    const e = this.map.get(key);
    if (!e) return undefined;
    if (Date.now() > e.expiresAt) {
      this.map.delete(key);
      return undefined;
    }
    return e.value;
  }

  set(key: K, value: V): void {
    if (this.map.size >= this.maxEntries) {
      const oldest = this.map.keys().next().value;
      if (oldest !== undefined) this.map.delete(oldest);
    }
    this.map.set(key, { value, expiresAt: Date.now() + this.ttlMs });
  }

  /** get-or-compute helper; concurrent misses may compute twice (acceptable here). */
  async getOrSet(key: K, compute: () => Promise<V>): Promise<V> {
    const hit = this.get(key);
    if (hit !== undefined) return hit;
    const value = await compute();
    this.set(key, value);
    return value;
  }
}
