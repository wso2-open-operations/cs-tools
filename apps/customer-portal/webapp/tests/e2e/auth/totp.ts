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

//
// TOTP codes for the sign-in flow's second factor (RFC 6238).
//
// Implemented on node:crypto rather than pulled from npm: the algorithm is
// thirty lines, and this runs with a real credential in scope — a dependency
// here would be one more package with access to the seed, updated by someone
// else, for no functional gain.
//

import crypto from "node:crypto";

/** Seconds each code is valid for. The standard step, and what Asgardeo uses. */
const STEP_SECONDS = 30;

/** Digits in a generated code. */
const DIGITS = 6;

const BASE32_ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";

/**
 * Decodes a base32 secret, as authenticator enrolment screens display it.
 *
 * Tolerates the spaces and lowercase such screens use for readability, and the
 * `=` padding some emit — so a seed can be pasted exactly as shown rather than
 * normalised by hand, which is where transcription errors come from.
 *
 * @param secret - Base32 secret, with or without spaces and padding.
 * @returns The decoded bytes.
 */
function base32Decode(secret: string): Buffer {
  const normalised = secret.replace(/[\s-]/g, "").replace(/=+$/, "").toUpperCase();

  const invalid = normalised.split("").find((c) => !BASE32_ALPHABET.includes(c));
  if (invalid) {
    throw new Error(
      `TOTP secret contains "${invalid}", which is not valid base32. ` +
        "Check E2E_TOTP_SECRET — it should be only A-Z and 2-7.",
    );
  }

  let bits = 0;
  let value = 0;
  const bytes: number[] = [];

  for (const char of normalised) {
    value = (value << 5) | BASE32_ALPHABET.indexOf(char);
    bits += 5;
    if (bits >= 8) {
      bytes.push((value >>> (bits - 8)) & 0xff);
      bits -= 8;
    }
  }

  return Buffer.from(bytes);
}

/**
 * Generates the TOTP code for a given moment.
 *
 * @param secret - Base32 secret.
 * @param atMs - Epoch milliseconds to generate for; defaults to now.
 * @returns A zero-padded 6-digit code.
 */
export function generateTotp(secret: string, atMs: number = Date.now()): string {
  const counter = Math.floor(atMs / 1000 / STEP_SECONDS);

  // The counter is a 64-bit big-endian integer. Written via BigInt because a
  // JS number cannot hold the full width, and a silent truncation here would
  // produce codes that are wrong only occasionally — the worst way to fail.
  const counterBytes = Buffer.alloc(8);
  counterBytes.writeBigUInt64BE(BigInt(counter));

  const hmac = crypto
    .createHmac("sha1", base32Decode(secret))
    .update(counterBytes)
    .digest();

  // Dynamic truncation, per RFC 4226 §5.4.
  const offset = hmac[hmac.length - 1] & 0x0f;
  const binary =
    ((hmac[offset] & 0x7f) << 24) |
    ((hmac[offset + 1] & 0xff) << 16) |
    ((hmac[offset + 2] & 0xff) << 8) |
    (hmac[offset + 3] & 0xff);

  return (binary % 10 ** DIGITS).toString().padStart(DIGITS, "0");
}

/**
 * Milliseconds until the current code expires.
 *
 * Used to avoid submitting a code that is about to roll over: a code accepted
 * at generation time can be rejected by the time the form is submitted, which
 * presents as an intermittent "invalid code" and is maddening to diagnose.
 *
 * @param atMs - Epoch milliseconds to measure from; defaults to now.
 * @returns Milliseconds remaining in the current step.
 */
export function millisUntilNextTotp(atMs: number = Date.now()): number {
  const stepMs = STEP_SECONDS * 1000;
  return stepMs - (atMs % stepMs);
}
