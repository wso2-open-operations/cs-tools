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
import {
  REPORT_SUPPORT_LINE_SUFFIX,
  reportCompanyName,
  reportLogoUrl,
  reportSupportLine,
} from "@config/reportBrandingConfig";
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
/** Least space kept between two texts on the footer line, in mm. */
export const FOOTER_TEXT_GAP_MM = 4;
/** Marks text that was shortened to fit. */
const ELLIPSIS = "...";

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
 * The text cut at the end, with "...", to fit `maxWidth` at the document's
 * current font size. Empty when not even "..." fits.
 *
 * @param doc - The PDF, with the footer font already set.
 * @param text - Text to fit.
 * @param maxWidth - Room for it, in mm.
 * @returns {string} The text, shortened when needed.
 */
export function fitTextToWidth(doc: jsPDF, text: string, maxWidth: number): string {
  if (doc.getTextWidth(text) <= maxWidth) return text;
  for (let end = text.length - 1; end > 0; end--) {
    const candidate = `${text.slice(0, end).trimEnd()}${ELLIPSIS}`;
    if (doc.getTextWidth(candidate) <= maxWidth) return candidate;
  }
  return doc.getTextWidth(ELLIPSIS) <= maxWidth ? ELLIPSIS : "";
}

/**
 * The "<company> Support 24x7" footer line, fitted to `maxWidth` at the current
 * font size. A long company name is shortened with "..." so " Support 24x7"
 * stays whole; only when even that does not fit is the whole line cut.
 *
 * @param doc - The PDF, with the footer font already set.
 * @param maxWidth - Room for the line, in mm.
 * @returns {string} The line to draw, possibly shortened, or empty.
 */
export function fitSupportLine(doc: jsPDF, maxWidth: number): string {
  const full = toPdfSafeText(reportSupportLine());
  if (doc.getTextWidth(full) <= maxWidth) return full;
  const name = toPdfSafeText(reportCompanyName());
  for (let end = name.length - 1; end > 0; end--) {
    const candidate = `${name.slice(0, end).trimEnd()}${ELLIPSIS}${REPORT_SUPPORT_LINE_SUFFIX}`;
    if (doc.getTextWidth(candidate) <= maxWidth) return candidate;
  }
  return fitTextToWidth(doc, full, maxWidth);
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
 * always had. The support line is shortened so it never runs into the page
 * text ({@link fitSupportLine}).
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

  for (let i = 1; i <= totalPages; i++) {
    doc.setPage(i);
    doc.setFontSize(8);
    doc.setFont("helvetica", "normal");
    const footerY = doc.internal.pageSize.getHeight() - 10;

    const middle = pageText(i, totalPages);
    const middleX = (pageWidth - doc.getTextWidth(middle)) / 2;

    const logoWidth = drawReportFooterLogo(doc, logo, FOOTER_MARGIN_MM, footerY);
    const supportX = FOOTER_MARGIN_MM + logoWidth;
    const supportLine = fitSupportLine(doc, middleX - FOOTER_TEXT_GAP_MM - supportX);
    if (supportLine) doc.text(supportLine, supportX, footerY);

    doc.text(middle, middleX, footerY);

    const dateWidth = doc.getTextWidth(createdDateTime);
    doc.text(createdDateTime, pageWidth - FOOTER_MARGIN_MM - dateWidth, footerY);
  }
}
