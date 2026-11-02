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
import GenerateChatPanel, {
  type GenerateChatMessage,
} from "@features/csm-custom-dashboards/components/GenerateChatPanel";

function renderPanel(props: Partial<Parameters<typeof GenerateChatPanel>[0]> = {}) {
  const onSend = vi.fn();
  const onClose = vi.fn();
  render(<GenerateChatPanel messages={[]} onSend={onSend} onClose={onClose} {...props} />);
  return { onSend, onClose, box: screen.getByRole("textbox", { name: "Message" }) };
}

describe("GenerateChatPanel", () => {
  it("greets, and offers example prompts until the first message", () => {
    renderPanel();
    expect(screen.getByText(/Describe the dashboard you'd like/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "My open cases by severity" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Message" })).toHaveAttribute("placeholder", "Describe a dashboard…");
  });

  it("hides the example prompts once there are messages", () => {
    const messages: GenerateChatMessage[] = [{ id: "1", role: "user", text: "Hello there" }];
    renderPanel({ messages });
    expect(screen.queryByRole("button", { name: "My open cases by severity" })).not.toBeInTheDocument();
    expect(screen.getByText("Hello there")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Message" })).toHaveAttribute("placeholder", "Ask for a change…");
  });

  it("fills the box from an example prompt without sending it", () => {
    const { onSend, box } = renderPanel();
    fireEvent.click(screen.getByRole("button", { name: "My open cases by severity" }));
    expect(box).toHaveValue("My open cases by severity");
    expect(onSend).not.toHaveBeenCalled();
  });

  it("sends the trimmed message with Enter and clears the box", () => {
    const { onSend, box } = renderPanel();
    fireEvent.change(box, { target: { value: "  Open cases  " } });
    fireEvent.keyDown(box, { key: "Enter" });
    expect(onSend).toHaveBeenCalledWith("Open cases");
    expect(box).toHaveValue("");
  });

  it("does not send with Shift+Enter (a new line)", () => {
    const { onSend, box } = renderPanel();
    fireEvent.change(box, { target: { value: "Open cases" } });
    fireEvent.keyDown(box, { key: "Enter", shiftKey: true });
    expect(onSend).not.toHaveBeenCalled();
  });

  it("sends with the Send button, which is disabled for a blank message", () => {
    const { onSend, box } = renderPanel();
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    fireEvent.change(box, { target: { value: "   " } });
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    fireEvent.change(box, { target: { value: "Open cases" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(onSend).toHaveBeenCalledWith("Open cases");
  });

  it("shows Generating… and blocks sending while a reply is on its way", () => {
    const { onSend, box } = renderPanel({ busy: true });
    expect(screen.getByText("Generating…")).toBeInTheDocument();
    fireEvent.change(box, { target: { value: "Open cases" } });
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    fireEvent.keyDown(box, { key: "Enter" });
    expect(onSend).not.toHaveBeenCalled();
  });

  it("closes with its ✕", () => {
    const { onClose } = renderPanel();
    fireEvent.click(screen.getByRole("button", { name: "Close chat" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("has its own title and greeting, and no example prompts, when editing", () => {
    renderPanel({ editing: true });
    expect(screen.getByText("Edit with AI")).toBeInTheDocument();
    expect(screen.getByText(/Ask for a change to this dashboard/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "My open cases by severity" })).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Message" })).toHaveAttribute("placeholder", "Ask for a change…");
  });
});
