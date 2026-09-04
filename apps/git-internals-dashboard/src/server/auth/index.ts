// src/server/auth/index.ts
// AUTH_MODE=stub    -> every request is a stub user (local dev / CI). Loud boot warning.
// AUTH_MODE=asgardeo-> Bearer access-token verification (JWKS sig, iss, aud) + group check.
// 401 = no/invalid token. 403 = valid token, required group absent.
//
// Applied as a wrapper HOF around each Route Handler (requireAuth), not as
// Next.js proxy.ts/middleware — that runs on the Edge runtime by default,
// which doesn't suit jose's createRemoteJWKSet + our Prisma-touching
// handlers. A wrapper mirrors the old Fastify onRequest hook closely while
// keeping every handler on the Node.js runtime.
import { NextResponse, type NextRequest } from "next/server";
import { createRemoteJWKSet, jwtVerify, type JWTVerifyGetKey } from "jose";

export interface CurrentUser {
  sub: string;
  groups: string[];
}

export interface AsgardeoOptions {
  baseUrl: string;
  audience: string;
  allowedGroup: string;
  groupsClaim: string;
  getKey?: JWTVerifyGetKey; // test injection; defaults to remote JWKS
}

export function asgardeoOptionsFromEnv(): AsgardeoOptions {
  const baseUrl = (process.env.ASGARDEO_BASE_URL ?? "").trim().replace(/\/+$/, "");
  const clientId = (process.env.ASGARDEO_CLIENT_ID ?? "").trim();
  const allowedGroup = (process.env.AUTH_ALLOWED_GROUP ?? "").trim();
  const missing = [
    !baseUrl && "ASGARDEO_BASE_URL",
    !clientId && "ASGARDEO_CLIENT_ID",
    !allowedGroup && "AUTH_ALLOWED_GROUP",
  ].filter(Boolean);
  if (missing.length) throw new Error(`AUTH_MODE=asgardeo requires: ${missing.join(", ")}`);
  return {
    baseUrl,
    audience: (process.env.ASGARDEO_AUDIENCE ?? "").trim() || clientId,
    allowedGroup,
    groupsClaim: (process.env.AUTH_GROUPS_CLAIM ?? "groups").trim(),
  };
}

function extractGroups(payload: Record<string, unknown>, claim: string): string[] {
  const raw = payload[claim];
  if (Array.isArray(raw)) return raw.filter((g): g is string => typeof g === "string");
  if (typeof raw === "string") return raw.split(",").map((s) => s.trim()).filter(Boolean);
  return [];
}

let cachedJwks: JWTVerifyGetKey | null = null;
let claimNamesLogged = false;

function getJwks(opts: AsgardeoOptions): JWTVerifyGetKey {
  if (opts.getKey) return opts.getKey;
  if (!cachedJwks) cachedJwks = createRemoteJWKSet(new URL(`${opts.baseUrl}/oauth2/jwks`));
  return cachedJwks;
}

export type AuthResult = { ok: true; user: CurrentUser } | { ok: false; response: NextResponse };

/**
 * Verifies the request's Bearer token (or synthesizes the stub user), given
 * explicit options. Split out from requireAuth() so tests can inject a local
 * JWKS via `opts.getKey` without any env vars or network access.
 */
export async function authenticate(req: NextRequest, opts?: AsgardeoOptions): Promise<AuthResult> {
  const mode = (process.env.AUTH_MODE ?? "stub").trim();

  if (mode !== "asgardeo") {
    // Fail-open stub identity is only ever safe outside production: it must
    // never be reachable just because AUTH_MODE was left unset or typo'd on
    // a real deployment. Gate it on the same NODE_ENV signal Next.js itself
    // uses to distinguish `next build`/`next start` from dev and test runs.
    if (process.env.NODE_ENV === "production") {
      throw new Error(
        "AUTH_MODE must be set to \"asgardeo\" in production; refusing to fall back to the stub identity.",
      );
    }
    return { ok: true, user: { sub: "stub@local", groups: [] } };
  }

  const asgardeo = opts ?? asgardeoOptionsFromEnv();
  const header = req.headers.get("authorization") ?? "";
  const token = header.startsWith("Bearer ") ? header.slice(7).trim() : "";
  if (!token) {
    return { ok: false, response: NextResponse.json({ error: "unauthorized" }, { status: 401 }) };
  }

  let payload: Record<string, unknown>;
  try {
    const issuer = `${asgardeo.baseUrl}/oauth2/token`;
    const verified = await jwtVerify(token, getJwks(asgardeo), { issuer, audience: asgardeo.audience });
    payload = verified.payload as Record<string, unknown>;
  } catch (err) {
    console.info("[auth] token verification failed", (err as Error).message);
    return { ok: false, response: NextResponse.json({ error: "unauthorized" }, { status: 401 }) };
  }

  if (process.env.AUTH_DEBUG_CLAIMS === "1" && !claimNamesLogged) {
    claimNamesLogged = true;
    console.info("[auth] verified token claim names (values withheld)", Object.keys(payload));
  }

  const groups = extractGroups(payload, asgardeo.groupsClaim);
  if (!groups.includes(asgardeo.allowedGroup)) {
    return {
      ok: false,
      response: NextResponse.json(
        { error: "not_authorized", message: "You are not authorized to access this application." },
        { status: 403 },
      ),
    };
  }

  return { ok: true, user: { sub: typeof payload.sub === "string" ? payload.sub : "", groups } };
}

type RouteContext = { params: Promise<Record<string, string>> };
type AuthedHandler = (req: NextRequest, ctx: RouteContext, user: CurrentUser) => Promise<Response> | Response;

/** Wraps a Route Handler so it only runs once `authenticate()` succeeds. */
export function requireAuth(handler: AuthedHandler) {
  return async (req: NextRequest, ctx: RouteContext): Promise<Response> => {
    const result = await authenticate(req);
    if (!result.ok) return result.response;
    return handler(req, ctx, result.user);
  };
}

/**
 * Second, optional authorization tier for state-changing/egress-causing
 * routes (currently just POST /api/sync/manual), on top of the single
 * AUTH_ALLOWED_GROUP check every requireAuth() route already gets.
 *
 * Unset AUTH_SYNC_GROUP (the default) preserves today's behavior: every
 * member of AUTH_ALLOWED_GROUP may call the route. Setting it narrows that
 * to members of the named group, without any other code change — the group
 * itself must still be created and populated on the Asgardeo tenant.
 *
 * Deliberately a no-op outside AUTH_MODE=asgardeo: stub-mode users carry no
 * groups at all, and stub mode is dev/CI-only, not a place to enforce this.
 */
export function authorizeSync(user: CurrentUser): boolean {
  const syncGroup = (process.env.AUTH_SYNC_GROUP ?? "").trim();
  if (!syncGroup) return true;
  if ((process.env.AUTH_MODE ?? "stub").trim() !== "asgardeo") return true;
  return user.groups.includes(syncGroup);
}

