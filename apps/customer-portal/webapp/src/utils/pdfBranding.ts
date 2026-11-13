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

import type { jsPDF } from "jspdf";
import { reportLogoUrl, reportSupportLine } from "@config/reportBrandingConfig";
import { toPdfSafeText } from "@utils/pdfText";

/** A logo ready for `jsPDF.addImage`: a PNG data URL and its pixel size. */
export type ReportLogo = {
  dataUrl: string;
  width: number;
  height: number;
};

/** A logo that has not loaded by then is left out, so the download is not held up. */
const LOGO_LOAD_TIMEOUT_MS = 5000;
/** Height the logo is drawn at before it is embedded, so an SVG stays sharp. */
const LOGO_RENDER_HEIGHT_PX = 96;
/** Footer logo height, and the widest a long logo may get, in mm. */
const FOOTER_LOGO_HEIGHT_MM = 5;
const FOOTER_LOGO_MAX_WIDTH_MM = 30;
/** Space between the logo and the footer text, in mm. */
const FOOTER_LOGO_GAP_MM = 2;
/** Left and right page margin of the footers, in mm. */
const FOOTER_MARGIN_MM = 14;

const logoCache = new Map<string, Promise<ReportLogo | null>>();

/**
 * Draws a loaded image onto a canvas and returns it as a PNG data URL. jsPDF
 * cannot embed an SVG, and an image from another origin that does not allow
 * CORS taints the canvas, so that case gives null.
 */
function rasterizeLogo(image: HTMLImageElement): ReportLogo | null {
  const naturalWidth = image.naturalWidth || image.width;
  const naturalHeight = image.naturalHeight || image.height;
  if (!naturalWidth || !naturalHeight) return null;

  const height = LOGO_RENDER_HEIGHT_PX;
  const width = Math.max(1, Math.round((naturalWidth * height) / naturalHeight));
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext("2d");
  if (!context) return null;
  try {
    context.drawImage(image, 0, 0, width, height);
    return { dataUrl: canvas.toDataURL("image/png"), width, height };
  } catch {
    return null;
  }
}

/** Loads one logo URL, null when it fails, is not drawable or takes too long. */
function fetchLogo(url: string): Promise<ReportLogo | null> {
  return new Promise((resolve) => {
    if (typeof Image === "undefined" || typeof document === "undefined") {
      resolve(null);
      return;
    }
    const image = new Image();
    let settled = false;
    const finish = (logo: ReportLogo | null): void => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      image.onload = null;
      image.onerror = null;
      resolve(logo);
    };
    const timer = setTimeout(() => finish(null), LOGO_LOAD_TIMEOUT_MS);
    image.crossOrigin = "anonymous";
    image.onload = () => finish(rasterizeLogo(image));
    image.onerror = () => finish(null);
    image.src = url;
  });
}

/**
 * The configured report logo (see {@link reportLogoUrl}), loaded once per URL.
 * Resolves to null when no logo is configured or it cannot be loaded; the
 * report is then printed without one. A failed load is tried again next time.
 *
 * @returns {Promise<ReportLogo | null>} The logo, or null.
 */
export function loadReportLogo(): Promise<ReportLogo | null> {
  const url = reportLogoUrl();
  if (!url) return Promise.resolve(null);
  const cached = logoCache.get(url);
  if (cached) return cached;
  const pending = fetchLogo(url).then((logo) => {
    if (!logo) logoCache.delete(url);
    return logo;
  });
  logoCache.set(url, pending);
  return pending;
}

/** Forgets the loaded logos. For tests. */
export function clearReportLogoCache(): void {
  logoCache.clear();
}

/**
 * Draws the logo at the left of a footer line, its bottom just under the text
 * baseline, and returns how far the text after it has to move right.
 *
 * @param doc - The PDF.
 * @param logo - The logo, or null for none.
 * @param x - Left edge, in mm.
 * @param baselineY - Baseline of the footer text, in mm.
 * @returns {number} Width taken by the logo and the gap after it, 0 when none was drawn.
 */
export function drawReportFooterLogo(
  doc: jsPDF,
  logo: ReportLogo | null,
  x: number,
  baselineY: number,
): number {
  if (!logo) return 0;
  let height = FOOTER_LOGO_HEIGHT_MM;
  let width = (height * logo.width) / logo.height;
  if (width > FOOTER_LOGO_MAX_WIDTH_MM) {
    width = FOOTER_LOGO_MAX_WIDTH_MM;
    height = (width * logo.height) / logo.width;
  }
  try {
    doc.addImage(logo.dataUrl, "PNG", x, baselineY + 1 - height, width, height);
  } catch {
    return 0;
  }
  return width + FOOTER_LOGO_GAP_MM;
}

/**
 * Draws the support footer on every page: logo and "<company> Support 24x7" on
 * the left, the page text in the middle and the generated time on the right.
 * With nothing configured this is the "WSO2 Support 24x7" footer the reports
 * always had.
 *
 * @param doc - The PDF, after all of its pages are written.
 * @param logo - The logo from {@link loadReportLogo}, or null.
 * @param pageText - Middle text for a page, given its number and the page count.
 * @param createdDateTime - When the report was made, printed on the right.
 */
export function drawReportSupportFooters(
  doc: jsPDF,
  logo: ReportLogo | null,
  pageText: (page: number, totalPages: number) => string,
  createdDateTime: string,
): void {
  const totalPages = doc.getNumberOfPages();
  const pageWidth = doc.internal.pageSize.getWidth();
  const supportLine = toPdfSafeText(reportSupportLine());

  for (let i = 1; i <= totalPages; i++) {
    doc.setPage(i);
    doc.setFontSize(8);
    doc.setFont("helvetica", "normal");
    const footerY = doc.internal.pageSize.getHeight() - 10;

    const logoWidth = drawReportFooterLogo(doc, logo, FOOTER_MARGIN_MM, footerY);
    doc.text(supportLine, FOOTER_MARGIN_MM + logoWidth, footerY);

    const middle = pageText(i, totalPages);
    doc.text(middle, (pageWidth - doc.getTextWidth(middle)) / 2, footerY);

    const dateWidth = doc.getTextWidth(createdDateTime);
    doc.text(createdDateTime, pageWidth - FOOTER_MARGIN_MM - dateWidth, footerY);
  }
}
