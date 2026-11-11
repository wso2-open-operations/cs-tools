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

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { UsersMeResponse } from "@features/settings/api/useGetUsersMe";

const mutateMock = vi.fn();
const showErrorMock = vi.fn();
const showSuccessMock = vi.fn();
let userMe: UsersMeResponse | undefined;

vi.mock("@features/settings/api/useGetUsersMe", () => ({
  useGetUsersMe: () => ({ data: userMe, isLoading: false }),
}));
vi.mock("@features/settings/api/usePatchUsersMe", () => ({
  usePatchUsersMe: () => ({ mutate: mutateMock, isPending: false }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: showSuccessMock }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ name: "Jane Doe", email: "jane@example.com" }),
}));

import { ApiError } from "@utils/ApiError";
import UserProfileModal from "@components/header/UserProfileModal";

const PHONE = "+15550001111";

function openDialog() {
  return render(<UserProfileModal open onClose={vi.fn()} />);
}

describe("UserProfileModal: phone number", () => {
  beforeEach(() => {
    mutateMock.mockReset();
    showErrorMock.mockReset();
    showSuccessMock.mockReset();
    userMe = {
      email: "jane@example.com",
      timeZone: "UTC",
      phoneNumber: PHONE,
    };
  });

  it("shows the phone number it is given and no alert-number warning", () => {
    openDialog();
    expect(screen.getByText(PHONE)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Use this number for alerts" }),
    ).not.toBeInTheDocument();
  });

  it("Save with the number unchanged sends nothing", () => {
    openDialog();
    fireEvent.click(screen.getByLabelText("Edit phone number"));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(mutateMock).not.toHaveBeenCalled();
  });

  it("Save sends an edited number", () => {
    openDialog();
    fireEvent.click(screen.getByLabelText("Edit phone number"));
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "+15550002222" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(mutateMock).toHaveBeenCalledTimes(1);
    expect(mutateMock.mock.calls[0][0]).toEqual({ phoneNumber: "+15550002222" });
  });

  it("rejects a number that is not E.164", () => {
    openDialog();
    fireEvent.click(screen.getByLabelText("Edit phone number"));
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "0555010000" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(mutateMock).not.toHaveBeenCalled();
    expect(showErrorMock).toHaveBeenCalledTimes(1);
  });

  function saveWith(value: string) {
    openDialog();
    fireEvent.click(screen.getByLabelText("Edit phone number"));
    fireEvent.change(screen.getByRole("textbox"), { target: { value } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
  }

  it.each(["+1 555-010-0000", "0555010000"])(
    "rejects %s client-side without calling the API",
    (value) => {
      saveWith(value);
      expect(mutateMock).not.toHaveBeenCalled();
      expect(showErrorMock).toHaveBeenCalledWith(
        expect.stringContaining("E.164"),
      );
    },
  );

  it("accepts +15550100000", () => {
    saveWith("+15550100000");
    expect(showErrorMock).not.toHaveBeenCalled();
    expect(mutateMock.mock.calls[0][0]).toEqual({ phoneNumber: "+15550100000" });
  });

  it("empty clears the number by sending an empty phoneNumber", () => {
    saveWith("   ");
    expect(showErrorMock).not.toHaveBeenCalled();
    expect(mutateMock.mock.calls[0][0]).toEqual({ phoneNumber: "" });
  });

  it("surfaces the backend's 400 message when the save fails", () => {
    const msg = "phone must be in E.164 format, e.g. +14155552671";
    mutateMock.mockImplementation(
      (_payload: unknown, opts: { onError: (e: Error) => void }) =>
        opts.onError(new ApiError(400, "Bad Request", msg)),
    );
    saveWith("+15550100000");
    expect(showErrorMock).toHaveBeenCalledWith(msg);
  });

  it("keeps the generic message for a non-400 failure", () => {
    mutateMock.mockImplementation(
      (_payload: unknown, opts: { onError: (e: Error) => void }) =>
        opts.onError(new ApiError(500, "Internal Server Error", "boom")),
    );
    saveWith("+15550100000");
    expect(showErrorMock).toHaveBeenCalledWith(
      "Could not update profile. Please try again.",
    );
  });

  it("tells the user the phone rule in the helper text", () => {
    openDialog();
    fireEvent.click(screen.getByLabelText("Edit phone number"));
    expect(screen.getByText(/8 to 15 digits, no spaces or dashes/)).toBeInTheDocument();
  });
});
