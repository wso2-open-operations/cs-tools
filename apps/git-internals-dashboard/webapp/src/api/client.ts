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

// Typed fetch wrapper: bearer attachment, error-envelope parsing, 401 ->
// sign-in. Port of v3's src/lib/api.ts `request()`, adapted for D3's uniform
// error envelope ({"error":{"code","message"}}) and a real cross-origin
// backend base URL (v3 called its own same-origin Next.js API routes;
// this app calls a separate Go service, so every request needs an absolute
// URL — SPEC §10's GID_BACKEND_BASE_URL).
import { windowConfig } from "@config/windowConfig";

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

// Module-level bridges so this plain-fetch client doesn't need React
// context (mirrors v3's src/lib/auth-token.ts) — wired up once from
// AuthGuard's AuthBridge, which does have access to useAsgardeo().
type TokenGetter = () => Promise<string>;
let tokenGetter: TokenGetter | null = null;
export function setAccessTokenGetter(fn: TokenGetter | null): void {
  tokenGetter = fn;
}
async function getAccessToken(): Promise<string | null> {
  if (!tokenGetter) return null;
  try {
    return await tokenGetter();
  } catch {
    return null;
  }
}

type UnauthorizedHandler = () => void;
let unauthorizedHandler: UnauthorizedHandler | null = null;
export function setUnauthorizedHandler(fn: UnauthorizedHandler | null): void {
  unauthorizedHandler = fn;
}

interface ErrorEnvelope {
  error?: { code?: string; message?: string };
}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = await getAccessToken();
  const res = await fetch(`${windowConfig.backendBaseUrl()}${path}`, {
    ...init,
    headers: {
      // AUDIT-FINDINGS B8: only a request with a body has a body type to
      // declare — a bodyless GET shouldn't send Content-Type.
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  });

  if (res.status === 401) {
    unauthorizedHandler?.();
  }

  if (!res.ok) {
    const body: ErrorEnvelope | null = await res.json().catch(() => null);
    throw new ApiError(
      res.status,
      body?.error?.code ?? "unknown",
      body?.error?.message ?? `API ${res.status} on ${path}`,
    );
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export function qs(params: Record<string, string | number | undefined>): string {
  const sp = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") sp.set(key, String(value));
  }
  const s = sp.toString();
  return s ? `?${s}` : "";
}
