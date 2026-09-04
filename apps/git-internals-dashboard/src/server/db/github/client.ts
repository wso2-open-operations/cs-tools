// src/server/db/github/client.ts
// =============================================================================
// Minimal GitHub GraphQL client shared by the seed and the incremental sync.
//   - fetchRepoIssues(): open issues + issues closed within a lookback window
//   - fetchIssueDetail(): per-issue Status-change timeline + current Status,
//                         the latter scoped to the configured project id
// PRIVACY: these queries deliberately do NOT request titles, assignees, or
// event actors. Labels are requested only to derive `priority` and are
// discarded after extraction.
// =============================================================================

const GITHUB_GRAPHQL = "https://api.github.com/graphql";

export interface GhIssueNode {
  number: number;
  state: "OPEN" | "CLOSED";
  url: string;
  createdAt: string;
  updatedAt: string;
  closedAt: string | null;
  labels: { nodes: { name: string }[] };
}
export interface GhStatusEvent {
  createdAt: string;
  previousStatus: string | null;
  status: string | null;
}
export interface GhProjectStatus {
  projectId: string;
  status: string | null;
  statusUpdatedAt: string | null;
  itemCreatedAt: string | null; // board-add time; assumed to resolve
}
export interface GhIssueDetail {
  number: number;
  events: GhStatusEvent[]; // ascending by createdAt
  projectStatuses: GhProjectStatus[]; // one per project the issue sits in
}

interface GraphQLResponse<T> {
  data?: T;
  errors?: Array<{ message: string; type?: string }>;
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

async function gql<T>(
  token: string,
  query: string,
  variables: Record<string, unknown>,
  attempt = 0,
): Promise<T> {
  try {
    const res = await fetch(GITHUB_GRAPHQL, {
      method: "POST",
      signal: AbortSignal.timeout(60_000), // avoids HTTP/2 stream timeout
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
        "User-Agent": "sla-tracker-seed",
      },
      body: JSON.stringify({ query, variables }),
    });

    if (!res.ok) {
      const body = await res.text();
      throw new Error(`GitHub GraphQL HTTP ${res.status}: ${body.slice(0, 500)}`);
    }

    const json = (await res.json()) as GraphQLResponse<T>;
    if (json.errors?.length) {
      throw new Error(`GitHub GraphQL error: ${json.errors.map((e) => e.message).join("; ")}`);
    }
    if (!json.data) throw new Error("GitHub GraphQL: empty response (no data)");
    return json.data;
  } catch (err) {
    if (attempt < 3) {
      const backoff = 2000 * (attempt + 1);
      console.warn(`[seed] gql retry ${attempt + 1}/3 in ${backoff}ms — ${(err as Error).message}`);
      await sleep(backoff);
      return gql<T>(token, query, variables, attempt + 1);
    }
    throw err;
  }
}

// ---------------------------------------------------------------------------
// Search: issues matching a repo filter, paginated.
// ---------------------------------------------------------------------------

const SEARCH_QUERY = `
query ($q: String!, $after: String) {
  search(type: ISSUE, query: $q, first: 50, after: $after) {
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on Issue {
        number
        state
        url
        createdAt
        updatedAt
        closedAt
        labels(first: 30) { nodes { name } }
      }
    }
  }
  rateLimit { remaining }
}`;

interface SearchData {
  search: {
    pageInfo: { hasNextPage: boolean; endCursor: string | null };
    nodes: Array<Partial<GhIssueNode>>;
  };
  rateLimit: { remaining: number };
}

// Exported for the incremental sync, which composes its own `updated:>=`
// query rather than the seed's separate open/closed queries.
export async function searchAll(token: string, q: string): Promise<GhIssueNode[]> {
  const out: GhIssueNode[] = [];
  let after: string | null = null;
  // GitHub Search returns at most 1000 results per query; our filters stay well under.
  while (true) {
    const data: SearchData = await gql<SearchData>(token, SEARCH_QUERY, { q, after });
    for (const n of data.search.nodes) {
      if (typeof n.number === "number") out.push(n as GhIssueNode);
    }
    if (!data.search.pageInfo.hasNextPage) break;
    after = data.search.pageInfo.endCursor;
    await sleep(250); // gentle on the secondary rate limiter
  }
  return out;
}

const isoDateDaysAgo = (days: number): string =>
  new Date(Date.now() - days * 86_400_000).toISOString().slice(0, 10);

/**
 * Every open issue matching the repo filter, plus issues closed within the
 * lookback window. Deduplicated by issue number (open wins if both appear).
 */
export async function fetchRepoIssues(
  token: string,
  owner: string,
  name: string,
  issueQuery: string,
  closedLookbackDays: number,
): Promise<GhIssueNode[]> {
  const base = `repo:${owner}/${name} ${issueQuery}`;
  const openQ = `${base} is:open sort:updated-desc`;
  const closedQ = `${base} is:closed closed:>=${isoDateDaysAgo(closedLookbackDays)} sort:updated-desc`;

  const closed = await searchAll(token, closedQ);
  const open = await searchAll(token, openQ);

  const byNumber = new Map<number, GhIssueNode>();
  for (const issue of [...closed, ...open]) byNumber.set(issue.number, issue); // open overwrites closed
  return [...byNumber.values()];
}

// ---------------------------------------------------------------------------
// Detail: per-issue Status timeline + current Status per project.
// ---------------------------------------------------------------------------

const DETAIL_QUERY = `
query ($owner: String!, $name: String!, $number: Int!, $tlCursor: String) {
  repository(owner: $owner, name: $name) {
    issue(number: $number) {
      number
      timelineItems(
        first: 100
        after: $tlCursor
        itemTypes: [PROJECT_V2_ITEM_STATUS_CHANGED_EVENT]
      ) {
        pageInfo { hasNextPage endCursor }
        nodes {
          __typename
          ... on ProjectV2ItemStatusChangedEvent {
            createdAt
            previousStatus
            status
          }
        }
      }
      projectItems(first: 20) {
        nodes {
          createdAt
          project { id }
          fieldValueByName(name: "Status") {
            ... on ProjectV2ItemFieldSingleSelectValue { name updatedAt }
          }
        }
      }
    }
  }
  rateLimit { remaining }
}`;

interface DetailData {
  repository: {
    issue: {
      number: number;
      timelineItems: {
        pageInfo: { hasNextPage: boolean; endCursor: string | null };
        nodes: Array<{
          __typename: string;
          createdAt?: string;
          previousStatus?: string | null;
          status?: string | null;
        }>;
      };
      projectItems: {
        nodes: Array<{
          createdAt: string;
          project: { id: string };
          fieldValueByName: { name: string; updatedAt: string } | null;
        }>;
      };
    } | null;
  } | null;
}

export async function fetchIssueDetail(
  token: string,
  owner: string,
  name: string,
  number: number,
): Promise<GhIssueDetail | null> {
  const events: GhStatusEvent[] = [];
  let projectStatuses: GhProjectStatus[] = [];
  let tlCursor: string | null = null;

  while (true) {
    const data: DetailData = await gql<DetailData>(token, DETAIL_QUERY, {
      owner,
      name,
      number,
      tlCursor,
    });
    const issue = data.repository?.issue;
    if (!issue) return null;

    for (const node of issue.timelineItems.nodes) {
      if (node.__typename !== "ProjectV2ItemStatusChangedEvent") continue;
      events.push({
        createdAt: node.createdAt as string,
        previousStatus: node.previousStatus ?? null,
        status: node.status ?? null,
      });
    }

    // projectItems (first: 20) is plenty; capture it once on the first page.
    if (projectStatuses.length === 0) {
      projectStatuses = issue.projectItems.nodes.map((p) => ({
        projectId: p.project.id,
        status: p.fieldValueByName?.name ?? null,
        statusUpdatedAt: p.fieldValueByName?.updatedAt ?? null,
        itemCreatedAt: p.createdAt ?? null,
      }));
    }

    if (!issue.timelineItems.pageInfo.hasNextPage) break;
    tlCursor = issue.timelineItems.pageInfo.endCursor;
    await sleep(200);
  }

  // Ascending by time — the SLA interval walk depends on this.
  events.sort((a, b) => a.createdAt.localeCompare(b.createdAt));
  return { number, events, projectStatuses };
}
