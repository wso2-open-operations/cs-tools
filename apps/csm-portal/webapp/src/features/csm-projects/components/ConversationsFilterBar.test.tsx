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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactNode } from "react";
import ConversationsFilterBar from "@features/csm-projects/components/ConversationsFilterBar";
import { DEFAULT_CONVERSATION_FILTERS } from "@features/csm-projects/utils/conversationState";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, get: vi.fn() }),
}));

vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

type BarProps = React.ComponentProps<typeof ConversationsFilterBar>;

function renderBar(
  props: Partial<BarProps> = {},
): { onChange: ReturnType<typeof vi.fn>; rerender: (next?: Partial<BarProps>) => void } {
  const onChange = vi.fn<BarProps["onChange"]>();
  const baseProps: BarProps = {
    filters: DEFAULT_CONVERSATION_FILTERS,
    onReset: vi.fn(),
    isFiltersOpen: false,
    onFiltersToggle: vi.fn(),
    ...props,
    onChange: props.onChange ?? onChange,
  };
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const renderResult = render(
    <QueryClientProvider client={queryClient}>
      <ConversationsFilterBar {...baseProps} />
    </QueryClientProvider> as ReactNode,
  );
  return {
    onChange: baseProps.onChange as ReturnType<typeof vi.fn>,
    rerender: (next = {}) =>
      renderResult.rerender(
        <QueryClientProvider client={queryClient}>
          <ConversationsFilterBar {...baseProps} {...next} />
        </QueryClientProvider> as ReactNode,
      ),
  };
}

describe("ConversationsFilterBar", () => {
  it("calls onChange with the updated search text", () => {
    const { onChange } = renderBar();

    fireEvent.change(screen.getByPlaceholderText(/Search by conversation/i), {
      target: { value: "billing" },
    });

    expect(onChange).toHaveBeenCalledWith({ ...DEFAULT_CONVERSATION_FILTERS, search: "billing" });
  });

  it("shows the filter grid only when open, and toggles the createdByMe checkbox", () => {
    const { onChange, rerender } = renderBar();

    expect(screen.queryByText("My conversations")).not.toBeInTheDocument();

    rerender({ isFiltersOpen: true });

    fireEvent.click(screen.getByLabelText("My conversations"));

    expect(onChange).toHaveBeenCalledWith({ ...DEFAULT_CONVERSATION_FILTERS, createdByMe: true });
  });

  it("shows the active filter count and a Clear filters action once a filter is set", () => {
    const onReset = vi.fn();
    renderBar({
      filters: { ...DEFAULT_CONVERSATION_FILTERS, states: ["ACTIVE"] },
      isFiltersOpen: true,
      onReset,
    });

    expect(screen.getByRole("button", { name: "Filters (1)" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Clear filters/i }));

    expect(onReset).toHaveBeenCalled();
  });

  it("calls onChange with the updated number filter", () => {
    const { onChange } = renderBar({ isFiltersOpen: true });

    fireEvent.change(screen.getByLabelText("Number"), {
      target: { value: "CHAT0000012345" },
    });

    expect(onChange).toHaveBeenCalledWith({
      ...DEFAULT_CONVERSATION_FILTERS,
      number: "CHAT0000012345",
    });
  });

  it("counts an explicit number in the active filter badge", () => {
    renderBar({
      filters: { ...DEFAULT_CONVERSATION_FILTERS, number: "CHAT0000012345" },
      isFiltersOpen: true,
    });

    expect(screen.getByRole("button", { name: "Filters (1)" })).toBeInTheDocument();
  });

  it("renders the initiator field with any already-selected emails", () => {
    renderBar({
      filters: { ...DEFAULT_CONVERSATION_FILTERS, createdBy: ["jane.doe@example.com"] },
      isFiltersOpen: true,
    });

    expect(screen.getByText("jane.doe@example.com")).toBeInTheDocument();
  });
});
