// src/server/db/client.ts
import { PrismaClient } from "@prisma/client";

// Choreo's managed-Postgres "Connection" feature injects five separate env
// vars (CHOREO_<NAME>_HOSTNAME/_PORT/_USERNAME/_PASSWORD/_DATABASENAME) rather
// than one DATABASE_URL — <NAME> is whatever the connection was named in the
// Choreo console. If DATABASE_URL isn't set, assemble one from that prefix
// (default "DB"; override via CHOREO_DB_CONNECTION_PREFIX to match your
// console-configured connection name) so the same Prisma schema works
// unmodified on Choreo, in Docker Compose, or any other Postgres host.
function resolveDatabaseUrl(): string | undefined {
  if (process.env.DATABASE_URL) return process.env.DATABASE_URL;

  const prefix = (process.env.CHOREO_DB_CONNECTION_PREFIX ?? "DB").trim();
  const host = process.env[`CHOREO_${prefix}_HOSTNAME`];
  const port = process.env[`CHOREO_${prefix}_PORT`];
  const user = process.env[`CHOREO_${prefix}_USERNAME`];
  const password = process.env[`CHOREO_${prefix}_PASSWORD`];
  const database = process.env[`CHOREO_${prefix}_DATABASENAME`];
  if (!host || !port || !user || !password || !database) return undefined;

  const url = `postgresql://${encodeURIComponent(user)}:${encodeURIComponent(password)}@${host}:${port}/${database}`;
  process.env.DATABASE_URL = url; // Prisma's datasource reads env("DATABASE_URL") at client construction
  return url;
}

resolveDatabaseUrl();

// Single shared client per process. Prisma maintains an internal connection pool
// sized by the `connection_limit` query param on DATABASE_URL (default 10 here).
// A module-level global survives Next.js dev-mode HMR re-evaluation of this
// module without opening a fresh pool on every edit.
const globalForPrisma = globalThis as unknown as { prisma?: PrismaClient };

export const prisma = globalForPrisma.prisma ?? new PrismaClient();

if (process.env.NODE_ENV !== "production") {
  globalForPrisma.prisma = prisma;
}
