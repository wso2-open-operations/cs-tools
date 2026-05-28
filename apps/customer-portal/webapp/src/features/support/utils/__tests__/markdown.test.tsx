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

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import ReactMarkdown from "react-markdown";
import {
  buildBotMarkdownComponents,
  isSafeHref,
  SAFE_PROTOCOLS,
  TextWithLinks,
} from "@features/support/utils/markdown";

describe("isSafeHref", () => {
  it("allows only http and https URLs", () => {
    expect(SAFE_PROTOCOLS).toEqual(["http:", "https:"]);
    expect(isSafeHref("https://example.com")).toBe(true);
    expect(isSafeHref("http://example.com")).toBe(true);
    expect(isSafeHref("javascript:alert(1)")).toBe(false);
    expect(isSafeHref("/relative-path")).toBe(false);
  });
});

describe("TextWithLinks", () => {
  it("renders safe bare URLs as links and leaves other text untouched", () => {
    render(
      <TextWithLinks text="Visit https://wso2.com and ftp://internal for details" />,
    );

    expect(
      screen.getByRole("link", { name: "https://wso2.com" }),
    ).toHaveAttribute("href", "https://wso2.com");
    expect(screen.getByText(/ftp:\/\/internal/)).toBeInTheDocument();
  });
});

describe("buildBotMarkdownComponents", () => {
  it("renders unsafe markdown links as plain text spans", () => {
    const components = buildBotMarkdownComponents();

    render(
      <ReactMarkdown components={components}>
        {"[safe](https://wso2.com) [unsafe](javascript:alert(1))"}
      </ReactMarkdown>,
    );

    expect(screen.getByRole("link", { name: "safe" })).toHaveAttribute(
      "href",
      "https://wso2.com",
    );
    expect(screen.queryByRole("link", { name: "unsafe" })).toBeNull();
    expect(screen.getByText("unsafe")).toBeInTheDocument();
  });
});
