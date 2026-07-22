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

const navigateMock = vi.fn();
const postProblemMutateMock = vi.fn();
const showErrorMock = vi.fn();
const isPending = false;

vi.mock("react-router", () => ({
  useNavigate: () => navigateMock,
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-operations/api/usePostProblem", () => ({
  usePostProblem: () => ({
    mutate: postProblemMutateMock,
    get isPending() {
      return isPending;
    },
  }),
}));
// CreateProblemPage imports BackendApiError from the real API client module,
// which reads window.config at module load and throws outside a configured
// runtime. Mock it with a real class (so `instanceof` still works) rather
// than exercising the real config — mirrors the pattern in
// useTimeSheets.test.ts.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.status = status;
    }
  },
}));

// Imported after the mocks above so the module picks them up.
import CreateProblemPage from "@features/csm-operations/pages/CreateProblemPage";

describe("CreateProblemPage", () => {
  it("disables submit until Subject is filled in", () => {
    render(<CreateProblemPage />);
    expect(screen.getByRole("button", { name: /create problem/i })).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Recurring gateway 502s" },
    });
    expect(screen.getByRole("button", { name: /create problem/i })).not.toBeDisabled();
  });

  it("shows a required error once Subject is touched and left blank", () => {
    render(<CreateProblemPage />);
    fireEvent.blur(screen.getByLabelText(/subject/i));
    expect(screen.getByText("Required")).toBeInTheDocument();
  });

  it("never renders a Priority field — priority is not settable on create", () => {
    render(<CreateProblemPage />);
    expect(screen.queryByLabelText(/priority/i)).not.toBeInTheDocument();
  });

  it("submits only the fields the user filled in, with subject trimmed", () => {
    render(<CreateProblemPage />);
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "  Recurring gateway 502s  " },
    });
    fireEvent.change(screen.getByLabelText(/^category$/i), {
      target: { value: "software" },
    });
    fireEvent.click(screen.getByRole("button", { name: /create problem/i }));
    expect(postProblemMutateMock).toHaveBeenCalledWith(
      { subject: "Recurring gateway 502s", category: "software" },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) }),
    );
  });

  it("includes the optional linking IDs when provided", () => {
    render(<CreateProblemPage />);
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Recurring gateway 502s" },
    });
    fireEvent.change(screen.getByLabelText(/origin case id/i), {
      target: { value: "case-123" },
    });
    fireEvent.change(screen.getByLabelText(/primary incident id/i), {
      target: { value: "inc-456" },
    });
    fireEvent.click(screen.getByRole("button", { name: /create problem/i }));
    expect(postProblemMutateMock).toHaveBeenCalledWith(
      {
        subject: "Recurring gateway 502s",
        originCaseId: "case-123",
        primaryIncidentId: "inc-456",
      },
      expect.anything(),
    );
  });

  it("navigates to the new problem's detail page on success", () => {
    render(<CreateProblemPage />);
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Recurring gateway 502s" },
    });
    fireEvent.click(screen.getByRole("button", { name: /create problem/i }));
    const [, options] = postProblemMutateMock.mock.calls[0];
    options.onSuccess({ id: "prb-1", number: "PRB0040200" });
    expect(navigateMock).toHaveBeenCalledWith("/operations/problems/prb-1");
  });

  it("surfaces a mutation error via the shared error banner", () => {
    render(<CreateProblemPage />);
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Recurring gateway 502s" },
    });
    fireEvent.click(screen.getByRole("button", { name: /create problem/i }));
    const [, options] = postProblemMutateMock.mock.calls[0];
    options.onError(new Error("network down"));
    expect(showErrorMock).toHaveBeenCalledWith(
      "Could not create the problem. Please try again.",
      expect.any(Error),
    );
  });
});
