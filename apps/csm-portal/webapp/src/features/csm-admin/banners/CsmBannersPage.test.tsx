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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { EMPTY_BANNER_CONTENT, buildBannerHtml } from "@features/csm-admin/banners/bannerHtml";
import CsmBannersPage from "@features/csm-admin/banners/CsmBannersPage";

const api = vi.hoisted(() => ({
  banners: [] as Array<Record<string, unknown>>,
  post: vi.fn(async (_path: string, body: Record<string, unknown>) => ({
    id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    ...body,
  })),
  put: vi.fn(async (_path: string, body: Record<string, unknown>) => body),
}));

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({
    get: async () => ({ banners: api.banners }),
    post: api.post,
    put: api.put,
  }),
}));

vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({
    value,
    onChange,
  }: {
    value?: string;
    onChange?: (html: string) => void;
  }) => (
    <textarea aria-label="Message" value={value ?? ""} onChange={(event) => onChange?.(event.target.value)} />
  ),
}));

function banner(partial: {
  id: string;
  status: string;
  sortOrder: number;
  visibleInCustomerPortal: boolean;
  visibleInCsmPortal: boolean;
  startsAt: string | null;
  expiresAt: string | null;
  html: string;
}): Record<string, unknown> {
  return partial;
}

api.banners = [
  banner({
    id: "11111111-1111-4111-8111-111111111111",
    status: "published",
    sortOrder: 0,
    visibleInCustomerPortal: true,
    visibleInCsmPortal: false,
    startsAt: "2020-01-01T00:00:00.000Z",
    expiresAt: "2099-01-01T00:00:00.000Z",
    html: buildBannerHtml(
      {
        ...EMPTY_BANNER_CONTENT,
        type: "image",
        title: "Event banner",
        textColor: "#ffffff",
        backgroundColor: "#000000",
        imageUrl: "https://cdn.example.com/event-banner.png",
        linkUrl: "https://example.com",
      },
      "11111111-1111-4111-8111-111111111111",
    ),
  }),
  banner({
    id: "22222222-2222-4222-8222-222222222222",
    status: "draft",
    sortOrder: 1,
    visibleInCustomerPortal: true,
    visibleInCsmPortal: true,
    startsAt: null,
    expiresAt: null,
    html: buildBannerHtml(
      {
        ...EMPTY_BANNER_CONTENT,
        type: "info",
        title: "Portal upgrade",
        message: "Portal upgrade this weekend.",
        closeable: true,
      },
      "22222222-2222-4222-8222-222222222222",
    ),
  }),
  banner({
    id: "33333333-3333-4333-8333-333333333333",
    status: "unpublished",
    sortOrder: 2,
    visibleInCustomerPortal: false,
    visibleInCsmPortal: false,
    startsAt: "2026-06-01T00:00:00.000Z",
    expiresAt: "2026-06-15T00:00:00.000Z",
    html: buildBannerHtml(
      {
        ...EMPTY_BANNER_CONTENT,
        type: "error",
        title: "June sale",
        message: "June sale — ended",
      },
      "33333333-3333-4333-8333-333333333333",
    ),
  }),
];

function renderPage(): void {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <CsmBannersPage />
    </QueryClientProvider>,
  );
}

describe("CsmBannersPage", () => {
  beforeEach(() => {
    api.post.mockClear();
    api.put.mockClear();
  });

  it("lists each banner as an open card with its preview", async () => {
    renderPage();
    expect(screen.queryByText(/Preview only/)).not.toBeInTheDocument();
    expect(screen.queryByTestId("portal-preview")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
    expect(await screen.findByRole("tab", { name: "Published (1)" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("Image")).toBeInTheDocument();
    expect(screen.getByTestId("banner-body-11111111-1111-4111-8111-111111111111")).toBeVisible();
    expect(screen.queryByText("Portal upgrade this weekend.")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: "Draft (1)" }));
    expect(screen.getAllByText("Portal upgrade this weekend.").length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole("tab", { name: "Unpublished (1)" }));
    expect(screen.getAllByText("June sale — ended").length).toBeGreaterThan(0);
  });

  it("previews a text banner while the engineer types, and fills the next place in the stack", async () => {
    renderPage();
    await screen.findByRole("tab", { name: "Published (1)" });
    fireEvent.click(screen.getByRole("button", { name: "New banner" }));
    expect(screen.queryByRole("textbox", { name: "HTML" })).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "Image URL" })).not.toBeInTheDocument();
    expect(screen.getByRole("spinbutton", { name: "Place in the stack" })).toHaveValue(2);
    expect(screen.getByRole("button", { name: "Banner Text" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Banner type Info" })).toHaveAttribute("aria-pressed", "true");
    fireEvent.change(screen.getByRole("textbox", { name: "Message" }), { target: { value: "Hello engineers" } });
    const preview = screen.getByTestId("banner-preview");
    expect(preview).toHaveTextContent("Hello engineers");
    expect(preview.innerHTML).toContain('data-banner-type="info"');
    fireEvent.click(screen.getByRole("button", { name: "Banner type Warning" }));
    expect(preview.innerHTML).toContain('data-banner-type="warning"');
    expect(preview.innerHTML).not.toContain('data-banner-type="info"');
  });

  it("lets a six-digit colour be typed and expands a short hex on blur", async () => {
    renderPage();
    await screen.findByRole("tab", { name: "Published (1)" });
    fireEvent.click(screen.getByRole("button", { name: "New banner" }));
    fireEvent.click(screen.getByRole("button", { name: "Banner type Custom" }));
    const hex = screen.getByRole("textbox", { name: "Text hex" });
    fireEvent.change(hex, { target: { value: "#112" } });
    expect(hex).toHaveValue("#112");
    fireEvent.blur(hex);
    expect(hex).toHaveValue("#111122");
    fireEvent.change(hex, { target: { value: "#112233" } });
    expect(hex).toHaveValue("#112233");
  });

  it("marks a missing message as required when save is pressed", async () => {
    renderPage();
    await screen.findByRole("tab", { name: "Published (1)" });
    fireEvent.click(screen.getByRole("button", { name: "New banner" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByRole("heading", { name: "New banner" })).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "Title" })).not.toBeInTheDocument();
    expect(screen.getAllByText("Required").length).toBeGreaterThanOrEqual(1);
    fireEvent.change(screen.getByRole("textbox", { name: "Message" }), { target: { value: "Hello engineers" } });
    expect(screen.queryByText("Required")).not.toBeInTheDocument();
    expect(api.post).not.toHaveBeenCalled();
  });

  it("saves a new banner through the API", async () => {
    renderPage();
    await screen.findByRole("tab", { name: "Published (1)" });
    fireEvent.click(screen.getByRole("button", { name: "New banner" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Message" }), { target: { value: "Weekend notice" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(api.post).toHaveBeenCalledTimes(1));
    const [path, body] = api.post.mock.calls[0] as [string, { status: string; html: string }];
    expect(path).toBe("/portal-banners");
    expect(body.status).toBe("draft");
    expect(body.html).toContain("Weekend notice");
    expect(body.html).not.toContain("data-title=");
    expect(body.html).not.toContain("data-id=");
    await waitFor(() => expect(screen.queryByRole("heading", { name: "New banner" })).not.toBeInTheDocument());
  });

  it("opens an image banner with the picture and the link, and no message", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    expect(screen.getByRole("heading", { name: "Edit banner" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Banner Image" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.queryByRole("textbox", { name: "Message" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Banner type Info" })).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Image URL" })).toHaveValue(
      "https://cdn.example.com/event-banner.png",
    );
    expect(screen.getByRole("textbox", { name: "Link URL" })).toHaveValue("https://example.com");
    expect(screen.getByRole("checkbox", { name: "Closeable" })).toBeInTheDocument();
  });
});
