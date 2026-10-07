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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import RotaPicker from "./RotaPicker";

const ROTAS = [
  { code: "SRE_SAAS", label: "SaaS · Apollo & Artemis" },
  { code: "SRE_IAAS", label: "IaaS" },
];

describe("RotaPicker", () => {
  it("is not there for a family with one rota or none, so CRE and SaaS-only SRE read as before", () => {
    const { container, rerender } = render(<RotaPicker rotas={[ROTAS[0]]} rotaCode="SRE_SAAS" onRotaChange={vi.fn()} />);
    expect(container).toBeEmptyDOMElement();
    rerender(<RotaPicker rotas={[]} onRotaChange={vi.fn()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the rota on screen and reports the one picked", () => {
    const onRotaChange = vi.fn();
    render(<RotaPicker rotas={ROTAS} rotaCode="SRE_SAAS" onRotaChange={onRotaChange} />);
    const select = screen.getByLabelText("Show one rota") as HTMLSelectElement;
    expect(select.value).toBe("SRE_SAAS");
    fireEvent.change(select, { target: { value: "SRE_IAAS" } });
    expect(onRotaChange).toHaveBeenCalledWith("SRE_IAAS");
  });
});
