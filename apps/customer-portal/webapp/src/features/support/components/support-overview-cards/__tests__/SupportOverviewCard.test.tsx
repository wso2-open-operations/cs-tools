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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import SupportOverviewCard from "@features/support/components/support-overview-cards/SupportOverviewCard";
import { SupportOverviewIconVariant } from "@features/support/types/supportOverview";

const MockIcon = () => <span data-testid="mock-icon">Icon</span>;

describe("SupportOverviewCard", () => {
  it("should render title, subtitle and footer button", () => {
    render(
      <SupportOverviewCard
        title="Outstanding Cases"
        subtitle="Latest 5 outstanding support tickets"
        icon={MockIcon}
        footerButtonLabel="View all cases"
        onFooterClick={vi.fn()}
      >
        <div>List content</div>
      </SupportOverviewCard>,
    );

    expect(screen.getByText("Outstanding Cases")).toBeInTheDocument();
    expect(screen.getByText("Latest 5 outstanding support tickets")).toBeInTheDocument();
    expect(screen.getByText("View all cases")).toBeInTheDocument();
    expect(screen.getByText("List content")).toBeInTheDocument();
    expect(screen.getByTestId("mock-icon")).toBeInTheDocument();
  });

  it("should call onFooterClick when footer button is clicked", () => {
    const onFooterClick = vi.fn();

    render(
      <SupportOverviewCard
        title="Chat History"
        subtitle="Recent conversations"
        icon={MockIcon}
        iconVariant={SupportOverviewIconVariant.Blue}
        footerButtonLabel="View all chat history"
        onFooterClick={onFooterClick}
      >
        <div>Chat list</div>
      </SupportOverviewCard>,
    );

    fireEvent.click(
      screen.getByRole("button", { name: /view all chat history/i }),
    );
    expect(onFooterClick).toHaveBeenCalledTimes(1);
  });
});
