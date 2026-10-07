/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { JSX } from "react";

/** One rota as the card head offers it. */
export interface RotaOption {
  code: string;
  label: string;
}

interface RotaPickerProps {
  rotas?: readonly RotaOption[];
  rotaCode?: string;
  onRotaChange?: (code: string) => void;
}

/**
 * Which rota of the family on screen -- SRE's SaaS or IaaS, one SME product.
 *
 * Beside the family switch in every card head, the same way the team picker
 * sits at the other end of it. Nothing when the family has one rota or none:
 * CRE, and SRE until a second rota has teams, read exactly as they always did.
 */
export default function RotaPicker({ rotas, rotaCode, onRotaChange }: RotaPickerProps): JSX.Element | null {
  if (!rotas || rotas.length < 2 || !onRotaChange) return null;
  return (
    <select
      className="teampick rotapick"
      aria-label="Show one rota"
      value={rotaCode ?? rotas[0].code}
      onChange={(e) => onRotaChange(e.target.value)}
    >
      {rotas.map((r) => (
        <option key={r.code} value={r.code}>
          {r.label}
        </option>
      ))}
    </select>
  );
}
