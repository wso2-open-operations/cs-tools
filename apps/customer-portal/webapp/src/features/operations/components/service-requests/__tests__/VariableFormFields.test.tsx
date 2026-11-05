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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import VariableFormFields from "@features/operations/components/service-requests/VariableFormFields";

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@components/rich-text-editor/Editor", () => ({
  default: () => <div data-testid="rich-text-editor" />,
}));

describe("VariableFormFields", () => {
  it("renders variable field label", () => {
    render(
      <VariableFormFields
        variables={[{ id: "v1", questionText: "Summary", type: "text" } as never]}
        isLoading={false}
        values={{}}
        onChange={() => {}}
        contextValues={{
          projectDisplay: "Proj",
          deploymentDisplay: "Dep",
          productDisplay: "Prod",
        }}
      />,
    );
    expect(screen.getByText("Summary")).toBeInTheDocument();
  });

  describe("date and time variables", () => {
    const context = { projectDisplay: "Proj", deploymentDisplay: "Dep", productDisplay: "Prod" };
    const dateTimeVariable = (id: string, questionText: string) =>
      ({ id, questionText, type: "datetime" }) as never;
    const pickerInput = (name: string) =>
      screen
        .getAllByLabelText(name)
        .find((el) => el.getAttribute("role") === "group")
        ?.querySelector("input") as HTMLInputElement;

    it("shows a stored YYYY-MM-DDTHH:mm value in the picker", () => {
      render(
        <VariableFormFields
          variables={[dateTimeVariable("v1", "Preferred time")]}
          isLoading={false}
          values={{ v1: "2030-06-10T14:30" }}
          onChange={() => {}}
          contextValues={context}
        />,
      );
      expect(pickerInput("Preferred time").value).toBe("06/10/2030 02:30 PM");
    });

    it("reports a picked time to the form as YYYY-MM-DDTHH:mm", () => {
      const onChange = vi.fn();
      render(
        <VariableFormFields
          variables={[dateTimeVariable("v1", "Preferred time")]}
          isLoading={false}
          values={{}}
          onChange={onChange}
          contextValues={context}
        />,
      );

      fireEvent.change(pickerInput("Preferred time"), { target: { value: "07/01/2031 09:15 PM" } });

      expect(onChange).toHaveBeenCalledWith("v1", "2031-07-01T21:15");
    });

    it("flags a start that is not before the end", () => {
      render(
        <VariableFormFields
          variables={[dateTimeVariable("s", "Start date"), dateTimeVariable("e", "End date")]}
          isLoading={false}
          values={{ s: "2030-06-10T14:30", e: "2030-06-10T13:00" }}
          onChange={() => {}}
          contextValues={context}
        />,
      );
      expect(screen.getByText("Start time must be before end time.")).toBeInTheDocument();
      expect(screen.getByText("End time must be after start time.")).toBeInTheDocument();
    });
  });
});
