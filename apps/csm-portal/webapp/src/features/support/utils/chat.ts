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

// Split long server `token` payloads so each tick appends a small, readable slice.
export function splitTokenForTyping(
  text: string,
  maxCharsPerTick: number,
): string[] {
  const cap = Math.max(1, maxCharsPerTick);
  if (text.length <= cap) return [text];
  const parts: string[] = [];
  for (let i = 0; i < text.length; i += cap) {
    parts.push(text.slice(i, i + cap));
  }
  return parts;
}

// Remove `description` key noise from streamed token chunks (partial JSON fragments).
export function sanitizeStreamToken(token: string): string {
  return token
    .replace(/"description"\s*:\s*"((?:\\.|[^"\\])*)"\s*,?\s*/gi, "")
    .replace(/'description'\s*:\s*'((?:\\.|[^'\\])*)'\s*,?\s*/gi, "")
    .replace(/\{\s*"description"\s*:\s*"((?:\\.|[^"\\])*)"\s*,\s*/gi, "{")
    .replace(/\{description\s*:\s*/gi, "")
    .replace(/\bdescription\s*:\s*/gi, "")
    .replace(/\*\*/g, "")
    .replace(/\n{3,}/g, "\n\n");
}

// REST conversation history sometimes stores bot content as JSON; show `message` only.
export function displayTextFromConversationContent(
  raw: string,
  isBot: boolean,
): string {
  if (!isBot) return raw;
  const trimmed = raw.trim();
  if (!trimmed.startsWith("{")) return raw;
  try {
    const parsed = JSON.parse(trimmed) as Record<string, unknown>;
    const onlyMessage = getFinalMessageFromPayload(parsed);
    return onlyMessage || raw;
  } catch {
    return raw;
  }
}

// Use only assistant `message` for the completed turn; ignore `description`.
export function getFinalMessageFromPayload(
  payload: Record<string, unknown>,
): string {
  const raw = payload.message;
  if (typeof raw === "string") return raw;
  if (raw != null && typeof raw === "object" && "message" in raw) {
    const inner = (raw as { message?: unknown }).message;
    if (typeof inner === "string") return inner;
  }
  return "";
}
