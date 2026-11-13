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

import { jsPDF } from "jspdf";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { CustomerPortalWindowConfig } from "@config/portalConfig";
import {
  clearReportLogoCache,
  drawReportSupportFooters,
  loadReportLogo,
  type ReportLogo,
} from "@utils/pdfBranding";

/** A 1x1 PNG, enough for jsPDF to embed. */
const PNG_DATA_URL =
  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=";

const originalConfig = window.config;
const OriginalImage = globalThis.Image;

function setConfig(values: Partial<CustomerPortalWindowConfig>): void {
  window.config = { ...originalConfig, ...values } as CustomerPortalWindowConfig;
}

/** An Image that loads (or fails) as soon as its src is set. */
function stubImage(outcome: "load" | "error", size = { width: 200, height: 50 }) {
  const created: string[] = [];
  class StubImage {
    crossOrigin = "";
    naturalWidth = size.width;
    naturalHeight = size.height;
    width = size.width;
    height = size.height;
    onload: (() => void) | null = null;
    onerror: (() => void) | null = null;
    set src(value: string) {
      created.push(value);
      queueMicrotask(() => (outcome === "load" ? this.onload?.() : this.onerror?.()));
    }
  }
  globalThis.Image = StubImage as unknown as typeof Image;
  return created;
}

describe("loadReportLogo", () => {
  beforeEach(() => {
    clearReportLogoCache();
    setConfig({
      CUSTOMER_PORTAL_LOGO_LIGHT_MODE_URL: undefined,
      CUSTOMER_PORTAL_LOGO_DARK_MODE_URL: undefined,
    });
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue({
      drawImage: vi.fn(),
    } as unknown as CanvasRenderingContext2D);
    vi.spyOn(HTMLCanvasElement.prototype, "toDataURL").mockReturnValue(PNG_DATA_URL);
  });

  afterEach(() => {
    window.config = originalConfig;
    globalThis.Image = OriginalImage;
    vi.restoreAllMocks();
  });

  it("is null when no logo is configured", async () => {
    const created = stubImage("load");
    await expect(loadReportLogo()).resolves.toBeNull();
    expect(created).toHaveLength(0);
  });

  it("loads the configured logo as a PNG at the render height, once", async () => {
    setConfig({ CUSTOMER_PORTAL_LOGO_LIGHT_MODE_URL: "https://cdn.test/logo.svg" });
    const created = stubImage("load");

    const logo = await loadReportLogo();
    expect(logo).toEqual({ dataUrl: PNG_DATA_URL, width: 384, height: 96 });
    await loadReportLogo();
    expect(created).toEqual(["https://cdn.test/logo.svg"]);
  });

  it("is null when the logo cannot be loaded, and tries again next time", async () => {
    setConfig({ CUSTOMER_PORTAL_LOGO_LIGHT_MODE_URL: "https://cdn.test/missing.svg" });
    const created = stubImage("error");

    await expect(loadReportLogo()).resolves.toBeNull();
    await expect(loadReportLogo()).resolves.toBeNull();
    expect(created).toHaveLength(2);
  });

  it("is null when the image cannot be drawn (another origin without CORS)", async () => {
    setConfig({ CUSTOMER_PORTAL_LOGO_LIGHT_MODE_URL: "https://other.test/logo.png" });
    stubImage("load");
    vi.spyOn(HTMLCanvasElement.prototype, "toDataURL").mockImplementation(() => {
      throw new DOMException("Tainted canvas", "SecurityError");
    });

    await expect(loadReportLogo()).resolves.toBeNull();
  });
});

describe("drawReportSupportFooters", () => {
  afterEach(() => {
    window.config = originalConfig;
  });

  function footerPdf(logo: ReportLogo | null): string {
    const doc = new jsPDF();
    doc.addPage();
    drawReportSupportFooters(doc, logo, (page, total) => `Page ${page} of ${total}`, "Oct 14, 2026");
    return doc.output();
  }

  it("prints the WSO2 support footer on every page when nothing is configured", () => {
    setConfig({ CUSTOMER_PORTAL_COMPANY_NAME: undefined });
    const pdf = footerPdf(null);

    expect(pdf.match(/\(WSO2 Support 24x7\) Tj/g)).toHaveLength(2);
    expect(pdf).toContain("(Page 2 of 2) Tj");
    expect(pdf).toContain("(Oct 14, 2026) Tj");
    expect(pdf).not.toContain("/Subtype /Image");
  });

  it("prints the configured company name and the logo", () => {
    setConfig({ CUSTOMER_PORTAL_COMPANY_NAME: "Acme Corp" });
    const pdf = footerPdf({ dataUrl: PNG_DATA_URL, width: 384, height: 96 });

    expect(pdf.match(/\(Acme Corp Support 24x7\) Tj/g)).toHaveLength(2);
    expect(pdf).not.toContain("WSO2 Support 24x7");
    expect(pdf).toContain("/Subtype /Image");
  });
});
