// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

/**
 * A code dashboard's customer health call (`portal.searchCustomerHealth`):
 * the Customer Health page's own list, `POST /customer-health/summary`. It
 * covers customer accounts with a project, each with its health status and
 * risk flags, and can name the viewer's own accounts, a product, a team or
 * sub-regions. An unknown product or team matches nothing rather than
 * failing, so both are checked against the page's own lists first.
 */

const HEALTH_RISKS = [
  "hasNoGoLive",
  "hasGoneLive",
  "hasRecentCases",
  "noSupportCases6mo",
  "hasEolProduct",
  "hasAbandonedMigrations",
  "hasMigrationDelays",
  "hasRecentEscalations",
] as const;

/**
 * The statuses a search can filter by. Not `to_be_reviewed`: an account with
 * no review yet is listed as to be reviewed but matches no status filter, so
 * that filter would leave most of them out.
 */
const HEALTH_STATUS_FILTERS = ["at_risk", "healthy"] as const;

/** The sub-regions the Customer Health page filters by (its RegionFilterSelect). */
const HEALTH_SUB_REGIONS = [
  "AFRICA",
  "ANZ",
  "ASIA",
  "Benelux + France",
  "LATAM",
  "ME",
  "NA - CENTRAL",
  "NA - EAST",
  "NA - SOUTH",
  "NA - WEST",
  "ROE",
  "RoA",
  "SAARC",
  "UK",
  "UK + Ireland",
] as const;

/** The page read when the code does not say, and the summary's own cap. */
const SEARCH_HEALTH_DEFAULT_LIMIT = 10;
const SEARCH_HEALTH_MAX_LIMIT = 50;

/** Body of POST /customer-health/summary. */
export interface CustomerHealthSummaryRequest {
  email?: string;
  risks?: string;
  region?: string[];
  product?: string;
  abtTeam?: string;
  healthStatus?: string;
  offset: 0;
  limit: number;
}

/**
 * Turns `portal.searchCustomerHealth(args)` into a `POST
 * /customer-health/summary` body, or throws (the code sees the message).
 * Args: `mine` (the viewer's own accounts: the backend uses the signed-in
 * user's email whatever is sent, so no other person can be named), `risks`
 * (any of them), `healthStatus`, `product` and `team` (one name each, checked
 * by `createNameLookup`), `subRegions` and `limit`. The summary has no sort.
 */
export function buildSearchCustomerHealthRequest(
  args: unknown,
  userEmail: string | undefined,
): CustomerHealthSummaryRequest {
  const fn = "searchCustomerHealth";
  const all = (args ?? {}) as Record<string, unknown>;
  for (const key of Object.keys(all)) {
    if (!["mine", "risks", "healthStatus", "product", "team", "subRegions", "limit"].includes(key)) {
      throw new Error(`${fn}: unknown argument "${key}"`);
    }
  }
  const { mine, risks, healthStatus, product, team, subRegions, limit } = all;
  const body: CustomerHealthSummaryRequest = { offset: 0, limit: SEARCH_HEALTH_DEFAULT_LIMIT };
  if (mine !== undefined && typeof mine !== "boolean") throw new Error(`${fn}: mine must be true or false`);
  if (mine) {
    if (!userEmail) throw new Error(`${fn}: the signed-in user is not loaded`);
    body.email = userEmail;
  }
  if (risks !== undefined) {
    if (!Array.isArray(risks) || risks.length === 0 || !risks.every((r) => HEALTH_RISKS.includes(r))) {
      throw new Error(`${fn}: risks must be a non-empty list of ${HEALTH_RISKS.join(", ")}`);
    }
    body.risks = risks.join(",");
  }
  if (healthStatus !== undefined) {
    if (healthStatus === "to_be_reviewed") {
      throw new Error(`${fn}: healthStatus cannot be to_be_reviewed; count all accounts less at_risk and healthy`);
    }
    if (!HEALTH_STATUS_FILTERS.includes(healthStatus as (typeof HEALTH_STATUS_FILTERS)[number])) {
      throw new Error(`${fn}: healthStatus must be one of ${HEALTH_STATUS_FILTERS.join(", ")}`);
    }
    body.healthStatus = healthStatus as string;
  }
  for (const [key, value, field] of [
    ["product", product, "product"],
    ["team", team, "abtTeam"],
  ] as const) {
    if (value === undefined) continue;
    if (typeof value !== "string" || value.trim() === "") throw new Error(`${fn}: ${key} must be one name`);
    body[field] = value;
  }
  if (subRegions !== undefined) {
    if (!Array.isArray(subRegions) || subRegions.length === 0 || !subRegions.every((r) => HEALTH_SUB_REGIONS.includes(r))) {
      throw new Error(`${fn}: subRegions must be a non-empty list of ${HEALTH_SUB_REGIONS.join(", ")}`);
    }
    body.region = subRegions as string[];
  }
  if (limit !== undefined) {
    if (!(Number.isInteger(limit) && (limit as number) >= 1 && (limit as number) <= SEARCH_HEALTH_MAX_LIMIT)) {
      throw new Error(`${fn}: limit must be an integer from 1 to ${SEARCH_HEALTH_MAX_LIMIT}`);
    }
    body.limit = limit as number;
  }
  return body;
}

/** A name as compared here: lower case, letters and digits only ("SaaS Night cover" and "saas-night-cover" match). */
function nameKey(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]/g, "");
}

/**
 * Checks a name against a list read once (`GET /products`, `GET /abt-teams`)
 * and returns the list's own spelling, or throws with the names that exist.
 * A failed read is not kept: the next call reads the list again.
 */
export function createNameLookup(
  read: () => Promise<string[] | null | undefined>,
): (fn: string, key: string, name: string) => Promise<string> {
  let names: Promise<string[]> | undefined;
  return async (fn, key, name) => {
    if (!names) {
      const reading = read().then((list) => list ?? []);
      names = reading;
      reading.catch(() => {
        if (names === reading) names = undefined;
      });
    }
    const list = await names;
    const match = list.find((n) => nameKey(n) === nameKey(name));
    if (!match) throw new Error(`${fn}: no ${key} is called ${name}; the ${key}s are ${list.join(", ")}`);
    return match;
  };
}

/** An account as `POST /customer-health/summary` returns it (the fields read here). */
interface AccountSummaryView {
  accountSysId: string;
  accountName?: string | null;
  hasNoGoLive?: { status?: string | null; isRisk?: boolean } | null;
  hasRecentCases?: boolean;
  hasEolProduct?: boolean;
  hasAbandonedMigrations?: boolean;
  hasMigrationDelays?: boolean;
  hasRecentEscalations?: boolean;
  noSupportCases6mo?: boolean;
  healthStatus?: string | null;
}

/** One account as `portal.searchCustomerHealth` returns it: its status and every risk flag. */
export interface CodeDashboardAccountHealth {
  id: string;
  name: string | null;
  healthStatus: string | null;
  /** The go-live state as the Customer Health page words it, and whether it is a risk. */
  goLive: string | null;
  noGoLive: boolean;
  hasRecentCases: boolean;
  noSupportCases6mo: boolean;
  hasEolProduct: boolean;
  hasAbandonedMigrations: boolean;
  hasMigrationDelays: boolean;
  hasRecentEscalations: boolean;
}

/** Reshapes a `POST /customer-health/summary` response into what `portal.searchCustomerHealth` returns. */
export function toSearchCustomerHealthResult(res: { data?: AccountSummaryView[] | null; totalCount?: number }): {
  total: number;
  items: CodeDashboardAccountHealth[];
} {
  const items = (res.data ?? []).map(
    (a): CodeDashboardAccountHealth => ({
      id: a.accountSysId,
      name: a.accountName ?? null,
      healthStatus: a.healthStatus ?? null,
      goLive: a.hasNoGoLive?.status ?? null,
      noGoLive: a.hasNoGoLive?.isRisk === true,
      hasRecentCases: a.hasRecentCases === true,
      noSupportCases6mo: a.noSupportCases6mo === true,
      hasEolProduct: a.hasEolProduct === true,
      hasAbandonedMigrations: a.hasAbandonedMigrations === true,
      hasMigrationDelays: a.hasMigrationDelays === true,
      hasRecentEscalations: a.hasRecentEscalations === true,
    }),
  );
  return { total: res.totalCount ?? items.length, items };
}
