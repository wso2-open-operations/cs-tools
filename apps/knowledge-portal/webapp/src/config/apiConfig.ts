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

// Extend the window interface with the runtime configuration that
// public/config.js sets before the app boots.
declare global {
  interface Window {
    config?: {
      /** Base URL of the knowledge portal backend. Required. */
      KNOWLEDGE_PORTAL_BACKEND_BASE_URL?: string;
      /** Where "Contact support" sends a visitor. Optional; the callouts are hidden without it. */
      KNOWLEDGE_PORTAL_SUPPORT_URL?: string;
    };
  }
}

export const BACKEND_BASE_URL_KEY = "KNOWLEDGE_PORTAL_BACKEND_BASE_URL";
export const SUPPORT_URL_KEY = "KNOWLEDGE_PORTAL_SUPPORT_URL";

/** Thrown when the runtime configuration is missing or malformed. */
export class ConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ConfigError";
  }
}

/**
 * Validates a raw backend base URL and returns it without a trailing slash.
 * There is deliberately no default: a production build that silently called
 * localhost would fail in every visitor's browser with an opaque network
 * error, so a missing value stops the app with a message naming the key.
 */
export function resolveBackendBaseUrl(raw: unknown): string {
  if (typeof raw !== "string" || raw.trim() === "") {
    throw new ConfigError(
      `Missing required configuration: ${BACKEND_BASE_URL_KEY}. ` +
        "Serve a config.js that sets window.config (see public/config.js.example).",
    );
  }
  const value = raw.trim();
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new ConfigError(
      `Invalid configuration: ${BACKEND_BASE_URL_KEY} must be an absolute http(s) URL.`,
    );
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new ConfigError(
      `Invalid configuration: ${BACKEND_BASE_URL_KEY} must be an absolute http(s) URL.`,
    );
  }
  return value.replace(/\/+$/, "");
}

/** The configured backend base URL; throws {@link ConfigError} if unusable. */
export function getBackendBaseUrl(): string {
  return resolveBackendBaseUrl(window.config?.[BACKEND_BASE_URL_KEY]);
}

/**
 * Validates the optional support URL: an absolute http(s) URL, or undefined
 * when it is unset. A malformed value is dropped (with a console warning)
 * rather than rendered, so a typo in config.js cannot produce a broken link.
 */
export function resolveSupportUrl(raw: unknown, warn: (message: string) => void = console.warn): string | undefined {
  if (typeof raw !== "string" || raw.trim() === "") return undefined;
  const value = raw.trim();
  try {
    const url = new URL(value);
    if (url.protocol === "http:" || url.protocol === "https:") return value;
  } catch {
    // fall through
  }
  warn(`Ignoring ${SUPPORT_URL_KEY}: it must be an absolute http(s) URL.`);
  return undefined;
}

/** The configured support URL, or undefined when none is set or it is unusable. */
export function getSupportUrl(): string | undefined {
  return resolveSupportUrl(window.config?.[SUPPORT_URL_KEY]);
}
