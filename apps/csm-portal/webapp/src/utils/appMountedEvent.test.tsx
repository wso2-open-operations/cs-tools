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

import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { APP_MOUNTED_EVENT, useAppMountedSignal } from "./appMountedEvent";

function TestHost(): null {
  useAppMountedSignal();
  return null;
}

describe("useAppMountedSignal", () => {
  it(`dispatches a window "${APP_MOUNTED_EVENT}" event on mount`, () => {
    const handler = vi.fn();
    window.addEventListener(APP_MOUNTED_EVENT, handler);
    try {
      render(<TestHost />);
      expect(handler).toHaveBeenCalledTimes(1);
    } finally {
      window.removeEventListener(APP_MOUNTED_EVENT, handler);
    }
  });

  it("does not re-dispatch on a re-render", () => {
    const handler = vi.fn();
    window.addEventListener(APP_MOUNTED_EVENT, handler);
    try {
      const { rerender } = render(<TestHost />);
      rerender(<TestHost />);
      expect(handler).toHaveBeenCalledTimes(1);
    } finally {
      window.removeEventListener(APP_MOUNTED_EVENT, handler);
    }
  });
});
