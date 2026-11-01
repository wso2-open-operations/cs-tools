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
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "0771234567" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(mutateMock).not.toHaveBeenCalled();
    expect(showErrorMock).toHaveBeenCalledTimes(1);
  });
});

describe("UserProfileModal: time zone required mark", () => {
  beforeEach(() => {
    mutateMock.mockReset();
    showErrorMock.mockReset();
    showSuccessMock.mockReset();
  });

  const captionMark = (): Element | null =>
    screen.getByText("Time zone").querySelector(".MuiFormLabel-asterisk");

  it("marks a stored time zone required while it is being edited, since it cannot be cleared", () => {
    userMe = { email: "jane@example.com", timeZone: "UTC", phoneNumber: PHONE };
    openDialog();
    expect(captionMark()).toBeNull();

    fireEvent.click(screen.getByLabelText("Edit time zone"));

    expect(captionMark()).not.toBeNull();
    expect(screen.getByRole("combobox")).toBeRequired();
  });

  it("does not mark it when no time zone is stored, where leaving it empty changes nothing", () => {
    userMe = { email: "jane@example.com", phoneNumber: PHONE };
    openDialog();
    fireEvent.click(screen.getByLabelText("Edit time zone"));

    expect(captionMark()).toBeNull();
    expect(screen.getByRole("combobox")).not.toBeRequired();
  });
});
