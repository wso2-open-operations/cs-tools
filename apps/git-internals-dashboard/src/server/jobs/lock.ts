// src/server/jobs/lock.ts
// Cross-replica mutex for the recompute tick and manual sync, so they never
// interleave — including across multiple Choreo replicas under autoscaling
// (Choreo does NOT guarantee single-instance on paid/private-data-plane
// tiers with HPA enabled, unlike v2's original single-instance assumption).
//
// Backed by a Postgres session-level advisory lock (`pg_try_advisory_lock`),
// held on one dedicated `pg` connection for the process lifetime — NOT
// through Prisma's pooled connections, since advisory locks are tied to the
// exact session that acquired them and Prisma may hand out a different
// pooled connection to the "unlock" call otherwise.
//
// `running` is a same-process fast-path flag: it mirrors v2's in-process
// boolean exactly (so /api/sync/status's "running" reflects *this* replica's
// activity, same as before), while the Postgres lock is the real
// cross-replica guarantee.
import { Client } from "pg";

const LOCK_KEY = 847_362_915; // arbitrary fixed key, unique to this app's job lock

let client: Client | null = null;
let connecting: Promise<Client> | null = null;

async function getClient(): Promise<Client> {
  if (client) return client;
  if (!connecting) {
    connecting = (async () => {
      const c = new Client({ connectionString: process.env.DATABASE_URL });
      c.on("error", (err) => {
        console.error("[jobLock] connection error", err);
        client = null;
        connecting = null;
      });
      await c.connect();
      client = c;
      return c;
    })();
  }
  return connecting;
}

export const jobLock = {
  running: false,

  async tryRun<T>(fn: () => Promise<T>): Promise<T | null> {
    if (this.running) return null; // in-process fast path

    // Claim the in-process guard synchronously, before any `await`, so two
    // concurrent calls can never both observe `running === false` and both
    // go on to hit the (reentrant, session-level) advisory lock query. If
    // the advisory lock turns out to be busy — or acquiring it fails, or
    // `fn` throws — the `finally` below rolls this back so a failed attempt
    // never wedges the lock for good.
    this.running = true;

    let c: Client | null = null;
    let locked = false;
    try {
      c = await getClient();
      const { rows } = await c.query<{ locked: boolean }>("SELECT pg_try_advisory_lock($1) AS locked", [LOCK_KEY]);
      locked = rows[0].locked;
      if (!locked) return null; // another replica holds it

      return await fn();
    } finally {
      this.running = false;
      if (locked && c) {
        await c.query("SELECT pg_advisory_unlock($1)", [LOCK_KEY]).catch((err) => {
          console.error("[jobLock] failed to release advisory lock", err);
        });
      }
    }
  },
};
