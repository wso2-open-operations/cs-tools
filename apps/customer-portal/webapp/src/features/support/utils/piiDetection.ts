// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

/**
 * Client-side PII (Personally Identifiable Information) detection.
 *
 * Scans plain text for structured sensitive data (credit cards, national IDs,
 * email, phone, IBAN) so the UI can warn a customer before their input is
 * posted. Runs synchronously in the browser — no network call, no data leaves
 * the client before the warning is shown.
 *
 * Out of scope: unstructured PII such as names and postal addresses, which
 * regex cannot reliably detect. Those would require a backend/DLP service.
 */

export enum PiiType {
  // Secrets / credentials — the highest-value category for a technical
  // support tool, where customers paste logs and config snippets.
  PRIVATE_KEY = "PRIVATE_KEY",
  CREDENTIALS_IN_URI = "CREDENTIALS_IN_URI",
  ACCESS_TOKEN = "ACCESS_TOKEN",
  JWT = "JWT",
  PASSWORD = "PASSWORD",
  // Classic personal data.
  CREDIT_CARD = "CREDIT_CARD",
  DANISH_CPR = "DANISH_CPR",
  NATIONAL_ID = "NATIONAL_ID",
  UK_NINO = "UK_NINO",
  PASSPORT = "PASSPORT",
  EMAIL = "EMAIL",
  PHONE = "PHONE",
  IBAN = "IBAN",
}

export interface PiiMatch {
  /** The category of PII detected. */
  type: PiiType;
  /** Human-readable label for the detected type (used in the warning dialog). */
  label: string;
  /** The matched substring. */
  snippet: string;
  /** Start index of the match within the scanned text. */
  index: number;
}

interface PiiPattern {
  type: PiiType;
  label: string;
  regex: RegExp;
  /**
   * Optional extra check to confirm a raw regex match is really PII.
   * Returning false drops the candidate (e.g. Luhn check for card numbers).
   */
  validate?: (match: string) => boolean;
}

/**
 * Luhn checksum — validates credit-card-like digit sequences and filters out
 * most random numbers (order IDs, reference numbers) that merely look like cards.
 */
export const passesLuhn = (value: string): boolean => {
  const digits = value.replace(/\D/g, "");
  if (digits.length < 13 || digits.length > 19) {
    return false;
  }
  let sum = 0;
  let double = false;
  for (let i = digits.length - 1; i >= 0; i -= 1) {
    let digit = digits.charCodeAt(i) - 48;
    if (double) {
      digit *= 2;
      if (digit > 9) {
        digit -= 9;
      }
    }
    sum += digit;
    double = !double;
  }
  return sum % 10 === 0;
};

/**
 * Detection patterns. Kept as a single exported config so the rule set can be
 * tuned (added to / relaxed) as false-positive feedback comes in.
 *
 * Order matters: it defines priority when two patterns match the same span
 * (e.g. a card number also looks like a phone number). Earlier = more specific,
 * and wins. See {@link detectPii}.
 */
export const PII_PATTERNS: readonly PiiPattern[] = [
  {
    // PEM-encoded private keys (RSA/EC/OpenSSH/PGP/DSA). Near-zero false rate.
    type: PiiType.PRIVATE_KEY,
    label: "Private key",
    regex: /-----BEGIN (?:[A-Z0-9]+ )?PRIVATE KEY-----/g,
  },
  {
    // Credentials embedded in a URL or DB connection string, e.g.
    // https://user:pass@host or jdbc:mysql://user:pass@host. Kept above EMAIL
    // so "pass@host.com" isn't mis-reported as an email address.
    type: PiiType.CREDENTIALS_IN_URI,
    label: "Credentials in a URL or connection string",
    regex: /\b(?:jdbc:)?[a-z][a-z0-9+.-]*:\/\/[^\s:@/]+:[^\s:@/]+@[^\s/]+/g,
  },
  {
    // Well-known service/cloud access tokens with distinctive prefixes.
    type: PiiType.ACCESS_TOKEN,
    label: "API key or access token",
    regex:
      /\b(?:AKIA[0-9A-Z]{16}|gh[posru]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{22,}|xox[baprs]-[A-Za-z0-9-]{10,}|AIza[0-9A-Za-z_-]{35}|[sr]k_(?:live|test)_[A-Za-z0-9]{16,}|npm_[A-Za-z0-9]{36})\b/g,
  },
  {
    // JSON Web Tokens (header.payload.signature, base64url).
    type: PiiType.JWT,
    label: "Authentication token",
    regex: /\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g,
  },
  {
    // A password/secret assigned in a config or log line, e.g.
    // password="s3cret", clientSecret: abc123, api_key=xxxx, pass: hunter2,
    // AccountKey=... (Azure). The keyword must be followed by ':' or '=' and a
    // value, so prose like "forgot their password" is not flagged.
    type: PiiType.PASSWORD,
    label: "Password or secret",
    regex:
      /\b(?:passphrase|password|passwd|pwd|pass|pin|secret|credential|token|bearer|client[_-]?secret|api[_-]?key|access[_-]?key|account[_-]?key|shared[_-]?access[_-]?key|auth[_-]?token)\b\s*[:=]\s*["']?[^\s"'<>{}]{4,}["']?/gi,
  },
  {
    type: PiiType.CREDIT_CARD,
    label: "Credit card number",
    // 13-19 digits allowing spaces or hyphens between groups.
    regex: /\b(?:\d[ -]*?){13,19}\b/g,
    validate: passesLuhn,
  },
  {
    type: PiiType.IBAN,
    label: "IBAN",
    regex: /\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b/g,
  },
  {
    type: PiiType.DANISH_CPR,
    label: "Danish CPR number",
    // DDMMYY-XXXX (optionally without the hyphen).
    regex: /\b(0[1-9]|[12]\d|3[01])(0[1-9]|1[0-2])\d{2}-?\d{4}\b/g,
  },
  {
    type: PiiType.NATIONAL_ID,
    label: "National ID / SSN",
    // US SSN style: 3-2-4 with separators.
    regex: /\b\d{3}-\d{2}-\d{4}\b/g,
  },
  {
    // UK National Insurance number: 2 letters, 6 digits, 1 suffix letter.
    type: PiiType.UK_NINO,
    label: "UK National Insurance number",
    regex: /\b[A-Z]{2} ?\d{2} ?\d{2} ?\d{2} ?[A-D]\b/g,
  },
  {
    // Passport number — only when the word "passport" is present, since a bare
    // 6-9 char alphanumeric alone is indistinguishable from an ID/order code.
    type: PiiType.PASSPORT,
    label: "Passport number",
    regex: /\bpassport\s*(?:no\.?|number|#|id)?\s*[:=-]?\s*[A-Z0-9]{6,9}\b/gi,
  },
  {
    type: PiiType.EMAIL,
    label: "Email address",
    regex: /\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b/g,
  },
  {
    // Most generic numeric pattern — kept last so more specific matches
    // (card, IBAN, CPR, SSN) win any overlap.
    type: PiiType.PHONE,
    label: "Phone number",
    // International/local numbers, 9-15 digits with common separators.
    regex: /(?:\+?\d[\d ().-]{7,}\d)/g,
    validate: (match) => match.replace(/\D/g, "").length >= 9,
  },
];

/**
 * Scans text for potential PII and returns all matches.
 *
 * @param text Plain text to scan (convert rich-text/HTML to plain text first).
 * @returns Matches sorted by position; empty array when nothing is found.
 */
export const detectPii = (text: string): PiiMatch[] => {
  if (!text) {
    return [];
  }

  // Collect every candidate, tagged with its pattern priority (array order).
  const candidates: (PiiMatch & { priority: number; end: number })[] = [];

  PII_PATTERNS.forEach((pattern, priority) => {
    // Fresh regex per scan to avoid shared lastIndex state on the /g flag.
    const regex = new RegExp(pattern.regex.source, pattern.regex.flags);
    let result: RegExpExecArray | null = regex.exec(text);
    while (result !== null) {
      const snippet = result[0];
      if (!pattern.validate || pattern.validate(snippet)) {
        candidates.push({
          type: pattern.type,
          label: pattern.label,
          snippet,
          index: result.index,
          end: result.index + snippet.length,
          priority,
        });
      }
      // Guard against zero-length matches causing an infinite loop.
      if (result.index === regex.lastIndex) {
        regex.lastIndex += 1;
      }
      result = regex.exec(text);
    }
  });

  // Resolve overlaps: when spans intersect, the higher-priority (more specific)
  // pattern wins, so a card number is not also reported as a phone number.
  const kept: typeof candidates = [];
  candidates
    .sort((a, b) => a.priority - b.priority)
    .forEach((candidate) => {
      const overlaps = kept.some(
        (k) => candidate.index < k.end && k.index < candidate.end
      );
      if (!overlaps) {
        kept.push(candidate);
      }
    });

  return kept
    .sort((a, b) => a.index - b.index)
    .map(({ type, label, snippet, index }) => ({ type, label, snippet, index }));
};

/**
 * Convenience predicate for submit handlers that only need a yes/no.
 */
export const containsPii = (text: string): boolean => detectPii(text).length > 0;

/**
 * Distinct PII type labels present in the text, for display in the warning.
 */
export const getDetectedPiiLabels = (text: string): string[] => {
  const labels = new Set<string>();
  for (const match of detectPii(text)) {
    labels.add(match.label);
  }
  return [...labels];
};
