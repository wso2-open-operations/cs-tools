import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { CompanyName, MASKED_EMAIL, MaskedEmail, RevealButton } from "./CompanyIdentity";

const postMock = vi.fn();
vi.mock("@features/plg/api/client", () => ({
  usePlgApi: () => ({ post: postMock }),
}));

const ORG = "11111111-1111-1111-1111-111111111111";

function Row({ companyId = "cloud-1", onRowClick }: { companyId?: string | null; onRowClick?: () => void }) {
  return (
    <div onClick={onRowClick}>
      <CompanyName organizationId={ORG} companyId={companyId} />
      <MaskedEmail organizationId={ORG} />
      <RevealButton organizationId={ORG} />
    </div>
  );
}

function renderRow(companyId?: string | null, onRowClick?: () => void) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Row companyId={companyId} onRowClick={onRowClick} />
    </QueryClientProvider>,
  );
}

function answer(identity: Record<string, unknown> | null) {
  if (identity) postMock.mockResolvedValue({ organizationId: ORG, ...identity });
  else postMock.mockImplementation(async () => {
    throw Object.assign(new Error("organisation not found"), { status: 404 });
  });
}

describe("the reveal", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("shows the company id and stars, and asks for nothing until the eye is clicked", () => {
    renderRow();
    expect(screen.getByText("cloud-1")).toBeInTheDocument();
    expect(screen.getByText(MASKED_EMAIL)).toBeInTheDocument();
    expect(postMock).not.toHaveBeenCalled();
  });

  it("replaces the company id and stars with the cloud's answer, and hides them again", async () => {
    answer({ companyId: "cloud-1", organizationName: "Acme", email: "admin@acme.io", source: "CLOUD" });
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: "Reveal name and email" }));
    expect(await screen.findByText("Acme")).toBeInTheDocument();
    expect(screen.getByText("admin@acme.io")).toBeInTheDocument();
    expect(screen.queryByText("cloud-1")).not.toBeInTheDocument();
    expect(postMock).toHaveBeenCalledWith(`/organizations/${ORG}/reveal`);

    fireEvent.click(screen.getByRole("button", { name: "Hide name and email" }));
    expect(await screen.findByText("cloud-1")).toBeInTheDocument();
    expect(screen.getByText(MASKED_EMAIL)).toBeInTheDocument();
  });

  it("keeps the values hidden when the cloud could not be asked, and lets the eye try again", async () => {
    answer({ companyId: "cloud-1", organizationName: null, email: null, source: "FAILED" });
    renderRow();

    fireEvent.click(screen.getByRole("button", { name: "Reveal name and email" }));
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(1));
    expect(screen.getByText("cloud-1")).toBeInTheDocument();
    expect(screen.getByText(MASKED_EMAIL)).toBeInTheDocument();

    const eye = screen.getByRole("button", { name: "Reveal name and email" });
    await waitFor(() => expect(eye).toBeEnabled());
    fireEvent.click(eye);
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(2));
  });

  it("disables the eye for an organisation that no longer exists", async () => {
    answer(null);
    renderRow();
    fireEvent.click(screen.getByRole("button", { name: "Reveal name and email" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Reveal name and email" })).toBeDisabled());
    expect(screen.getByText(MASKED_EMAIL)).toBeInTheDocument();
  });

  it("never opens the row, even when the eye is disabled", async () => {
    answer(null);
    const onRowClick = vi.fn();
    renderRow(undefined, onRowClick);
    fireEvent.click(screen.getByRole("button", { name: "Reveal name and email" }));
    const eye = screen.getByRole("button", { name: "Reveal name and email" });
    await waitFor(() => expect(eye).toBeDisabled());
    // A disabled button lets the click land on its wrapper.
    fireEvent.click(eye.parentElement as HTMLElement);
    expect(onRowClick).not.toHaveBeenCalled();
  });

  it("says so when there is no company id", () => {
    renderRow(null);
    expect(screen.getByText("No company id")).toBeInTheDocument();
  });
});
