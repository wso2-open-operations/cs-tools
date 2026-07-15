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
  CREDIT_CARD = "CREDIT_CARD",
  DANISH_CPR = "DANISH_CPR",
  NATIONAL_ID = "NATIONAL_ID",
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
