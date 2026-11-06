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

const WORDS_PER_MINUTE = 200;

/** Whole minutes to read `plainText` at an average pace; 0 for empty text, never less than 1 otherwise. */
export function readingTimeMinutes(plainText: string): number {
  const words = plainText.trim().split(/\s+/).filter((w) => w.length > 0).length;
  if (words === 0) return 0;
  return Math.max(1, Math.round(words / WORDS_PER_MINUTE));
}

/** "N min read", or undefined when there is nothing to read. */
export function formatReadingTime(minutes: number): string | undefined {
  return minutes > 0 ? `${minutes} min read` : undefined;
}
