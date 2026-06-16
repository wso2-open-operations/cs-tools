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

import { colors } from "@wso2/oxygen-ui";

/**
 * Resolve a Material colour-palette hue + shade from oxygen-ui's `colors`, with
 * a hard fallback so a missing hue never breaks a chart.
 */
export function paletteColor(
  hue: keyof typeof colors,
  shade: number,
  fallback: string,
): string {
  const ramp = colors[hue] as Record<number, string> | undefined;
  return ramp?.[shade] ?? fallback;
}
