// src/server/lib/issue-titles.ts
// Runtime issue-title resolution (PRIVACY: titles are never persisted).
//
// Flow:  ids -> (owner, name, number) from OUR DB -> batched GitHub GraphQL
//        (aliased repository/issue lookups, <=100 issues per request)
//        -> in-memory TTL cache (15 min) -> { [id]: title | null }.
//
// Returns null per issue when: no GITHUB_TOKEN, unknown id, synthetic fixture,
// deleted GitHub issue, or GitHub API failure.
import { prisma } from "@/server/db";
import { TtlCache } from "./ttl-cache";

const GITHUB_GRAPHQL = "https://api.github.com/graphql";
const BATCH = 100; // issues per GraphQL request

// key: "owner/name#number" -> title (or null for confirmed-unresolvable)
const titleCache = new TtlCache<string, string | null>(15 * 60_000, 5000);

interface Ref {
  id: number;
  owner: string;
  name: string;
  number: number;
}

const refKey = (r: Ref) => `${r.owner}/${r.name}#${r.number}`;

// GraphQL alias-safe fragment: aliases must match [A-Za-z_][A-Za-z0-9_]*
export function buildQuery(refs: Ref[]): string {
  const byRepo = new Map<string, Ref[]>();
  for (const r of refs) {
    const k = `${r.owner}/${r.name}`;
    if (!byRepo.has(k)) byRepo.set(k, []);
    byRepo.get(k)!.push(r);
  }

  let ri = 0;
  const parts: string[] = [];
  for (const group of byRepo.values()) {
    const { owner, name } = group[0];
    const issues = group
      .map((r) => `n${r.number}: issue(number: ${r.number}) { title }`)
      .join("\n      ");
    // owner/name come from our DB config; JSON.stringify still escapes defensively.
    parts.push(
      `r${ri}: repository(owner: ${JSON.stringify(owner)}, name: ${JSON.stringify(name)}) {\n      ${issues}\n    }`,
    );
    ri++;
  }
  return `query IssueTitles {\n    ${parts.join("\n    ")}\n  }`;
}

async function fetchBatch(refs: Ref[]): Promise<Map<string, string | null>> {
  const out = new Map<string, string | null>();
  const token = (process.env.GITHUB_TOKEN ?? "").trim();
  if (refs.length === 0 || !token) return out;

  const res = await fetch(GITHUB_GRAPHQL, {
    method: "POST",
    signal: AbortSignal.timeout(15_000),
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      "User-Agent": "sla-tracker-api",
    },
    body: JSON.stringify({ query: buildQuery(refs) }),
  });
  if (!res.ok) throw new Error(`GitHub GraphQL HTTP ${res.status}`);

  // Partial data + errors is normal (e.g. one deleted issue); use what came back.
  const json = (await res.json()) as {
    data?: Record<string, Record<string, { title: string } | null> | null>;
    errors?: Array<{ message: string }>;
  };

  // GraphQL can return HTTP 200 with `data: null` (missing scope, secondary
  // rate limit, query too complex). That's a whole-batch failure, not a
  // per-issue null — throw so the caller doesn't cache nulls for 15 min.
  if (!json.data) {
    throw new Error(`GitHub GraphQL: empty response (no data)${json.errors ? `: ${json.errors.map((e) => e.message).join("; ")}` : ""}`);
  }

  // Re-derive the alias layout to read the response back.
  const byRepo = new Map<string, Ref[]>();
  for (const r of refs) {
    const k = `${r.owner}/${r.name}`;
    if (!byRepo.has(k)) byRepo.set(k, []);
    byRepo.get(k)!.push(r);
  }
  let ri = 0;
  for (const group of byRepo.values()) {
    const repoNode = json.data?.[`r${ri}`] ?? null;
    for (const r of group) {
      out.set(refKey(r), repoNode?.[`n${r.number}`]?.title ?? null);
    }
    ri++;
  }
  return out;
}

/**
 * Resolve display titles for internal issue ids. Ids that don't exist in our
 * DB, or can't be resolved on GitHub, map to null.
 */
export async function getIssueTitles(ids: number[]): Promise<Record<number, string | null>> {
  const result: Record<number, string | null> = {};
  for (const id of ids) result[id] = null;

  // Resolve ids -> refs from OUR DB (never trust client-supplied repo/number).
  const rows = await prisma.issue.findMany({
    where: { id: { in: ids }, repository: { is: { enabled: true } } },
    select: {
      id: true,
      githubNumber: true,
      repository: { select: { owner: true, name: true } },
    },
  });
  const refs: Ref[] = rows.map((r) => ({
    id: r.id,
    owner: r.repository.owner,
    name: r.repository.name,
    number: r.githubNumber,
  }));

  // Cache pass.
  const misses: Ref[] = [];
  for (const r of refs) {
    const hit = titleCache.get(refKey(r));
    if (hit !== undefined) result[r.id] = hit;
    else misses.push(r);
  }
  const token = (process.env.GITHUB_TOKEN ?? "").trim();
  if (misses.length === 0 || !token) return result;

  // Fetch misses in batches; a failed batch degrades to nulls, never a 500.
  for (let i = 0; i < misses.length; i += BATCH) {
    const chunk = misses.slice(i, i + BATCH);
    try {
      const fetched = await fetchBatch(chunk);
      for (const r of chunk) {
        const title = fetched.get(refKey(r)) ?? null;
        titleCache.set(refKey(r), title);
        result[r.id] = title;
      }
    } catch {
      // leave chunk as null; do not cache failures so a later request can retry
    }
  }
  return result;
}
