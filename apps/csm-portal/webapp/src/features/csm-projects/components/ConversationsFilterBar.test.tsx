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
import "@testing-library/jest-dom/vitest";
import ConversationsFilterBar from "@features/csm-projects/components/ConversationsFilterBar";
import { DEFAULT_CONVERSATION_FILTERS } from "@features/csm-projects/utils/conversationState";

describe("ConversationsFilterBar", () => {
  it("calls onChange with the updated search text", () => {
    const onChange = vi.fn();
    render(
      <ConversationsFilterBar
        filters={DEFAULT_CONVERSATION_FILTERS}
        onChange={onChange}
        onReset={vi.fn()}
        isFiltersOpen={false}
        onFiltersToggle={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByPlaceholderText(/Search by conversation/i), {
      target: { value: "billing" },
    });

    expect(onChange).toHaveBeenCalledWith({ ...DEFAULT_CONVERSATION_FILTERS, search: "billing" });
  });

  it("shows the filter grid only when open, and toggles the createdByMe checkbox", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <ConversationsFilterBar
        filters={DEFAULT_CONVERSATION_FILTERS}
        onChange={onChange}
        onReset={vi.fn()}
        isFiltersOpen={false}
        onFiltersToggle={vi.fn()}
      />,
    );

    expect(screen.queryByText("My conversations")).not.toBeInTheDocument();

    rerender(
      <ConversationsFilterBar
        filters={DEFAULT_CONVERSATION_FILTERS}
        onChange={onChange}
        onReset={vi.fn()}
        isFiltersOpen
        onFiltersToggle={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByLabelText("My conversations"));

    expect(onChange).toHaveBeenCalledWith({ ...DEFAULT_CONVERSATION_FILTERS, createdByMe: true });
  });

  it("shows the active filter count and a Clear filters action once a filter is set", () => {
    const onReset = vi.fn();
    render(
      <ConversationsFilterBar
        filters={{ ...DEFAULT_CONVERSATION_FILTERS, states: ["ACTIVE"] }}
        onChange={vi.fn()}
        onReset={onReset}
        isFiltersOpen
        onFiltersToggle={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "Filters (1)" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Clear filters/i }));

    expect(onReset).toHaveBeenCalled();
  });
});
