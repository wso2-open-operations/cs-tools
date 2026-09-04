// src/server/auth/index.test.ts
// jose generateKeyPair + createLocalJWKSet injected via AsgardeoOptions.getKey.
// No network, no real Asgardeo tenant.
import { NextRequest } from "next/server";
import { createLocalJWKSet, exportJWK, generateKeyPair, SignJWT } from "jose";
import { afterEach, beforeAll, describe, expect, it } from "vitest";
import { authenticate, authorizeSync, requireAuth, type AsgardeoOptions } from "./index";

const BASE_URL = "https://issuer.example/t/test";
const ISSUER = `${BASE_URL}/oauth2/token`;
const AUDIENCE = "test-client";
const ALLOWED_GROUP = "sla-users";
const KID = "test-key";

let privateKey: Awaited<ReturnType<typeof generateKeyPair>>["privateKey"];
let getKey: AsgardeoOptions["getKey"];

beforeAll(async () => {
  const { publicKey, privateKey: priv } = await generateKeyPair("RS256");
  privateKey = priv;
  const jwk = await exportJWK(publicKey);
  getKey = createLocalJWKSet({ keys: [{ ...jwk, kid: KID, alg: "RS256", use: "sig" }] });
});

function signToken(opts: {
  issuer?: string;
  audience?: string;
  groups?: string[] | string;
  expiresInSeconds?: number;
}): Promise<string> {
  const now = Math.floor(Date.now() / 1000);
  let jwt = new SignJWT({ groups: opts.groups })
    .setProtectedHeader({ alg: "RS256", kid: KID })
    .setIssuedAt(now)
    .setSubject("user-123")
    .setIssuer(opts.issuer ?? ISSUER)
    .setAudience(opts.audience ?? AUDIENCE);
  jwt = jwt.setExpirationTime(now + (opts.expiresInSeconds ?? 3600));
  return jwt.sign(privateKey);
}

function req(token?: string): NextRequest {
  return new NextRequest("https://app.example/api/protected", {
    headers: token ? { authorization: `Bearer ${token}` } : {},
  });
}

describe("authenticate — asgardeo mode", () => {
  const opts: AsgardeoOptions = {
    baseUrl: BASE_URL,
    audience: AUDIENCE,
    allowedGroup: ALLOWED_GROUP,
    groupsClaim: "groups",
    get getKey() {
      return getKey;
    },
  };
  const originalMode = process.env.AUTH_MODE;
  beforeAll(() => {
    process.env.AUTH_MODE = "asgardeo";
  });
  afterEach(() => {
    process.env.AUTH_MODE = "asgardeo";
  });

  it("passes a valid token with the allowed group in an array", async () => {
    const token = await signToken({ groups: [ALLOWED_GROUP, "other-group"] });
    const result = await authenticate(req(token), opts);
    expect(result.ok).toBe(true);
    if (result.ok) expect(result.user).toMatchObject({ sub: "user-123", groups: [ALLOWED_GROUP, "other-group"] });
  });

  it("passes a valid token with the group as a comma-separated string", async () => {
    const token = await signToken({ groups: `other-group, ${ALLOWED_GROUP}` });
    const result = await authenticate(req(token), opts);
    expect(result.ok).toBe(true);
  });

  it("401s on a wrong issuer", async () => {
    const token = await signToken({ groups: [ALLOWED_GROUP], issuer: "https://not-the-issuer.example/oauth2/token" });
    const result = await authenticate(req(token), opts);
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.response.status).toBe(401);
  });

  it("401s on a wrong audience", async () => {
    const token = await signToken({ groups: [ALLOWED_GROUP], audience: "someone-else" });
    const result = await authenticate(req(token), opts);
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.response.status).toBe(401);
  });

  it("401s on an expired token", async () => {
    const token = await signToken({ groups: [ALLOWED_GROUP], expiresInSeconds: -10 });
    const result = await authenticate(req(token), opts);
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.response.status).toBe(401);
  });

  it("403s with the exact user-facing message when the group is absent", async () => {
    const token = await signToken({ groups: ["some-other-group"] });
    const result = await authenticate(req(token), opts);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.response.status).toBe(403);
      expect(await result.response.json()).toEqual({
        error: "not_authorized",
        message: "You are not authorized to access this application.",
      });
    }
  });

  it("401s when the Authorization header is missing", async () => {
    const result = await authenticate(req(), opts);
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.response.status).toBe(401);
  });
});

describe("requireAuth in stub mode", () => {
  const originalMode = process.env.AUTH_MODE;

  afterEach(() => {
    process.env.AUTH_MODE = originalMode;
  });

  it("passes every request through with the stub user", async () => {
    process.env.AUTH_MODE = "stub";
    const handler = requireAuth(async (_req, _ctx, user) => Response.json({ sub: user.sub }));
    const res = await handler(req(), { params: Promise.resolve({}) });
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ sub: "stub@local" });
  });
});

describe("authorizeSync", () => {
  const originalMode = process.env.AUTH_MODE;
  const originalGroup = process.env.AUTH_SYNC_GROUP;

  afterEach(() => {
    process.env.AUTH_MODE = originalMode;
    process.env.AUTH_SYNC_GROUP = originalGroup;
  });

  it("allows any authenticated user when AUTH_SYNC_GROUP is unset (default, unchanged behavior)", () => {
    process.env.AUTH_MODE = "asgardeo";
    delete process.env.AUTH_SYNC_GROUP;
    expect(authorizeSync({ sub: "user-123", groups: [] })).toBe(true);
  });

  it("is a no-op outside asgardeo mode even when AUTH_SYNC_GROUP is set", () => {
    process.env.AUTH_MODE = "stub";
    process.env.AUTH_SYNC_GROUP = "sla-operators";
    expect(authorizeSync({ sub: "stub@local", groups: [] })).toBe(true);
  });

  it("requires membership in AUTH_SYNC_GROUP when set, in asgardeo mode", () => {
    process.env.AUTH_MODE = "asgardeo";
    process.env.AUTH_SYNC_GROUP = "sla-operators";
    expect(authorizeSync({ sub: "user-123", groups: ["sla-users"] })).toBe(false);
    expect(authorizeSync({ sub: "user-123", groups: ["sla-users", "sla-operators"] })).toBe(true);
  });
});
