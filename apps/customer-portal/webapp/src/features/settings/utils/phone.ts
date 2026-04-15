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

import { getCountries, getCountryCallingCode, type Country } from "react-phone-number-input";

/** Country dial code and name for phone input. */
export interface PhoneCountryOption {
  dialCode: string;
  countryCode: string;
  label: string;
  flag: string;
}

/**
 * Get country flag emoji from country code.
 * Converts ISO 3166-1 alpha-2 country code to flag emoji.
 *
 * @param countryCode - ISO 3166-1 alpha-2 country code (e.g., "US", "GB").
 * @returns Flag emoji or empty string.
 */
function getCountryFlag(countryCode: string): string {
  if (!countryCode || countryCode.length !== 2) return "";
  const codePoints = countryCode
    .toUpperCase()
    .split("")
    .map((char) => 127397 + char.charCodeAt(0));
  return String.fromCodePoint(...codePoints);
}

/**
 * Initialize country options dynamically from react-phone-number-input.
 * Returns formatted list with flags, dial codes, and country labels.
 *
 * @returns Array of phone country options.
 */
function initializeCountryOptions(): PhoneCountryOption[] {
  const countries = getCountries() || [];
  return countries
    .map((countryCode: string) => {
      try {
        const dialCode = getCountryCallingCode(countryCode as Country);
        if (!dialCode) return null;

        const displayName = new Intl.DisplayNames(["en"], {
          type: "region",
        }).of(countryCode);

        return {
          dialCode: `+${dialCode}`,
          countryCode,
          label: `${displayName} (+${dialCode})`,
          flag: getCountryFlag(countryCode),
        };
      } catch {
        return null;
      }
    })
    .filter((option): option is PhoneCountryOption => option !== null)
    .sort((a, b) => a.label.localeCompare(b.label));
}

/** Common country options for phone input (E.164) - dynamically loaded. */
export const PHONE_COUNTRY_OPTIONS: PhoneCountryOption[] =
  initializeCountryOptions();

/**
 * Get dial code from ISO country code.
 *
 * @param isoCountryCode - ISO 3166-1 alpha-2 country code (e.g., "US", "GB").
 * @returns Dial code with + prefix (e.g., "+1", "+44"), or empty string if not found.
 */
export function getDialCodeFromCountryCode(isoCountryCode: string): string {
  const option = PHONE_COUNTRY_OPTIONS.find(
    (o) => o.countryCode === isoCountryCode,
  );
  return option?.dialCode || "";
}

/**
 * Get ISO country code from dial code.
 *
 * @param dialCode - Dial code with + prefix (e.g., "+1", "+44").
 * @returns ISO country code (e.g., "US", "GB"), or empty string if not found.
 */
export function getCountryCodeFromDialCode(dialCode: string): string {
  const option = PHONE_COUNTRY_OPTIONS.find((o) => o.dialCode === dialCode);
  return option?.countryCode || "";
}

/**
 * Format E.164 phone for display (e.g. "+1 555 123 4567").
 *
 * @param e164 - E.164 string (e.g. "+15551234567").
 * @returns Formatted string or empty.
 */
export function formatPhoneForDisplay(e164: string | null | undefined): string {
  if (!e164?.trim()) return "";
  const digits = e164.replace(/\D/g, "");
  if (digits.length < 10) return e164;
  const match = e164.match(/^(\+\d{1,4})(\d+)$/);
  if (match) {
    const [, code, rest] = match;
    const grouped = rest.replace(/(\d{3})(?=\d)/g, "$1 ");
    return `${code} ${grouped}`.trim();
  }
  return e164;
}

/**
 * Parse E.164 to { countryCode, nationalNumber }.
 *
 * @param e164 - E.164 string.
 * @returns Parsed parts.
 */
export function parseE164(
  e164: string | null | undefined,
): { countryCode: string; nationalNumber: string } {
  if (!e164?.trim()) return { countryCode: "+1", nationalNumber: "" };
  const digits = e164.replace(/\D/g, "");
  if (digits.length === 0) return { countryCode: "+1", nationalNumber: "" };
  for (let len = 4; len >= 1; len--) {
    const code = "+" + digits.slice(0, len);
    const rest = digits.slice(len);
    const opt = PHONE_COUNTRY_OPTIONS.find((o) => o.dialCode === code);
    if (opt && rest.length >= 5) {
      return { countryCode: code, nationalNumber: rest };
    }
  }
  return { countryCode: "+1", nationalNumber: digits };
}

/**
 * Parse E.164 to { countryCode (ISO), nationalNumber }.
 * Returns ISO country code instead of dial code.
 *
 * @param e164 - E.164 string.
 * @returns Parsed parts with ISO country code.
 */
export function parseE164ToCountryCode(
  e164: string | null | undefined,
): { countryCode: string; nationalNumber: string } {
  if (!e164?.trim()) return { countryCode: "US", nationalNumber: "" };
  const digits = e164.replace(/\D/g, "");
  if (digits.length === 0) return { countryCode: "US", nationalNumber: "" };
  for (let len = 4; len >= 1; len--) {
    const code = "+" + digits.slice(0, len);
    const rest = digits.slice(len);
    const dialCodeOption = PHONE_COUNTRY_OPTIONS.find(
      (o) => o.dialCode === code,
    );
    if (dialCodeOption && rest.length >= 5) {
      return {
        countryCode: dialCodeOption.countryCode,
        nationalNumber: rest,
      };
    }
  }
  return { countryCode: "US", nationalNumber: digits };
}

/**
 * Build E.164 from ISO country code and national number.
 *
 * @param isoCountryCode - ISO 3166-1 alpha-2 country code (e.g., "US", "GB").
 * @param nationalNumber - National number without country code.
 * @returns E.164 formatted phone number.
 */
export function toE164FromCountryCode(
  isoCountryCode: string,
  nationalNumber: string,
): string {
  const dialCode = getDialCodeFromCountryCode(isoCountryCode);
  const digits = nationalNumber.replace(/\D/g, "");
  return digits ? dialCode + digits : "";
}

/** E.164 regex: + followed by 10–15 digits. */
const E164_REGEX = /^\+[1-9]\d{9,14}$/;

/**
 * Validate E.164 phone. Empty is allowed (optional field).
 *
 * @param value - E.164 string.
 * @returns Error message or empty if valid.
 */
export function validatePhoneE164(value: string | null | undefined): string {
  if (!value?.trim()) return "";
  const trimmed = value.trim();
  if (!E164_REGEX.test(trimmed)) {
    return "Please enter a valid phone number in E.164 format (e.g. +1234567890)";
  }
  return "";
}
