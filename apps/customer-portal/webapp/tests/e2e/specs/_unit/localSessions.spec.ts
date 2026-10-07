import { test, expect } from "@playwright/test";
import fs from "node:fs";
import {
  LOCAL_PERSONAS,
  isLocalPersona,
  localSessionName,
  mintCommand,
  sessionMinutesLeft,
} from "../../auth/localSessions";
import { sessionPath } from "../../fixtures/test";

/** A JWT-shaped string carrying only an expiry; the signature is never checked. */
function jwtExpiringIn(seconds: number): string {
  const part = (v: object) => Buffer.from(JSON.stringify(v)).toString("base64url");
  const exp = Math.floor(Date.now() / 1000) + seconds;
  return `${part({ alg: "RS256", typ: "JWT", kid: "mock-oidc-1" })}.${part({ exp })}.c2ln`;
}

/** Writes a throwaway bundle, runs `body`, and removes it. */
function withBundle(name: string, bundle: object, body: () => void): void {
  const file = sessionPath(name);
  fs.mkdirSync(file.slice(0, file.lastIndexOf("/")), { recursive: true });
  fs.writeFileSync(file, JSON.stringify(bundle));
  try {
    body();
  } finally {
    fs.rmSync(file, { force: true });
  }
}

test("local sessions are named per persona and never collide with the staging bundle", () => {
  expect(localSessionName("dave")).toBe("local-dave");
  expect(localSessionName("noel")).toBe("local-noel");
  for (const persona of Object.keys(LOCAL_PERSONAS)) {
    expect(isLocalPersona(persona)).toBe(true);
    expect(localSessionName(persona as keyof typeof LOCAL_PERSONAS)).not.toBe("session");
  }
  expect(isLocalPersona("alice")).toBe(false);
  expect(isLocalPersona("toString")).toBe(false);
  expect(mintCommand("mira")).toContain("E2E_LOCAL_PERSONA=mira");
});

test("session life is read from the tokens in the bundle", () => {
  const name = "unit-local-sessions";

  expect(sessionMinutesLeft(name), "no bundle").toBeNull();

  withBundle(name, { origin: "http://x", sessionStorage: { a: "{}" } }, () => {
    expect(sessionMinutesLeft(name), "a bundle with no token").toBeNull();
  });

  withBundle(
    name,
    { origin: "http://x", sessionStorage: { s: JSON.stringify({ access_token: jwtExpiringIn(3600) }) } },
    () => {
      const left = sessionMinutesLeft(name)!;
      expect(left).toBeGreaterThanOrEqual(58);
      expect(left).toBeLessThanOrEqual(60);
    },
  );

  withBundle(
    name,
    { origin: "http://x", sessionStorage: { s: JSON.stringify({ id_token: jwtExpiringIn(-600) }) } },
    () => {
      expect(sessionMinutesLeft(name)!, "expired tokens read as negative").toBeLessThan(0);
    },
  );

  // The LATEST expiry decides, as in auth.setup.ts: one fresh token keeps the bundle alive.
  withBundle(
    name,
    {
      origin: "http://x",
      sessionStorage: { s: JSON.stringify({ a: jwtExpiringIn(-600), b: jwtExpiringIn(1800) }) },
    },
    () => {
      expect(sessionMinutesLeft(name)!).toBeGreaterThanOrEqual(28);
    },
  );
});
