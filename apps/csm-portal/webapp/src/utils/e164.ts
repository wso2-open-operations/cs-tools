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

/**
 * E.164 phone validator: leading +, a non-zero country-code digit, then 7-14
 * more digits (8-15 digits in total, no spaces or dashes). Mirrors the
 * backend rule for user phone numbers; values are never normalized.
 */
export const E164_PHONE_REGEX = /^\+[1-9]\d{7,14}$/;

/** User-facing statement of the rule, for helper text. */
export const E164_PHONE_HINT =
  "Start with +, then the country code and number: 8 to 15 digits, no spaces or dashes.";
