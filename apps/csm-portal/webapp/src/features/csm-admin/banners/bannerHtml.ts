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

import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";

/**
 * An image banner is a picture. A text banner is a message, in one of the
 * four maintenance colours or in colours the engineer chooses.
 * The kind is written into the HTML, not into its own column.
 */
export const TEXT_BANNER_TYPES = ["info", "warning", "error", "success", "custom"] as const;
export type TextBannerType = (typeof TEXT_BANNER_TYPES)[number];
export const BANNER_TYPES = ["image", ...TEXT_BANNER_TYPES] as const;
export type BannerType = (typeof BANNER_TYPES)[number];

export const BANNER_TYPE_LABEL: Record<BannerType, string> = {
  image: "Image",
  info: "Info",
  warning: "Warning",
  error: "Error",
  success: "Success",
  custom: "Custom",
};

/** One Oxygen alert: pale background, dark text, icon in the severity colour. */
export interface AlertTone {
  backgroundColor: string;
  textColor: string;
  iconColor: string;
}

/** Colours taken from the Oxygen alert for the four notification kinds. */
export interface SeverityColors {
  info: AlertTone;
  warning: AlertTone;
  error: AlertTone;
  success: AlertTone;
}

const TOP_COLORS = { textColor: "#ffffff", backgroundColor: "#000000" };
const ANNOUNCEMENT_COLORS = { textColor: "#f9fafb", backgroundColor: "#1f2937" };

/** The same paths Oxygen's Alert uses for info, warning, error, and success. */
const ALERT_ICON_PATH = {
  info: "M11,9H13V7H11M12,20C7.59,20 4,16.41 4,12C4,7.59 7.59,4 12,4C16.41,4 20,7.59 20,12C20,16.41 16.41,20 12,20M12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12A10,10 0 0,0 12,2M11,17H13V11H11V17Z",
  warning: "M12 5.99L19.53 19H4.47L12 5.99M12 2L1 21h22L12 2zm1 14h-2v2h2v-2zm0-6h-2v4h2v-4z",
  error: "M11 15h2v2h-2zm0-8h2v6h-2zm.99-5C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zM12 20c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8z",
  success: "M20,12A8,8 0 0,1 12,20A8,8 0 0,1 4,12A8,8 0 0,1 12,4C12.76,4 13.5,4.11 14.2,4.31L15.77,2.74C14.61,2.26 13.34,2 12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12M7.91,10.08L6.5,11.5L11,16L21,6L19.59,4.58L11,13.17L7.91,10.08Z",
} as const;

function alertIcon(type: keyof typeof ALERT_ICON_PATH, color: string): string {
  return `<svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" aria-hidden="true" style="flex:none;fill:${color}"><path d="${ALERT_ICON_PATH[type]}"/></svg>`;
}

export function colorsForBannerType(
  type: BannerType,
  severity: SeverityColors,
): { textColor: string; backgroundColor: string; iconColor: string } {
  if (type === "image") return { ...TOP_COLORS, iconColor: "" };
  if (type === "custom") return { ...ANNOUNCEMENT_COLORS, iconColor: "" };
  return { ...severity[type] };
}

/** The parts of a banner the engineer edits. Saving turns them into one HTML string. */
export interface BannerContent {
  type: BannerType;
  /** The name shown on the banner card. Stored in the HTML, not in its own column. */
  title: string;
  /** Rich text from the banner editor, stored as HTML. */
  message: string;
  textColor: string;
  backgroundColor: string;
  /** Severity colour of the Oxygen alert icon. Empty for an image or a custom banner. */
  iconColor: string;
  imageUrl: string;
  linkUrl: string;
  closeable: boolean;
}

export const EMPTY_BANNER_CONTENT: BannerContent = {
  type: "info",
  title: "",
  message: "",
  textColor: ANNOUNCEMENT_COLORS.textColor,
  backgroundColor: ANNOUNCEMENT_COLORS.backgroundColor,
  iconColor: "",
  imageUrl: "",
  linkUrl: "",
  closeable: false,
};

function escapeText(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function normalizeHex(value: string): string {
  const hex = value.toLowerCase();
  if (/^#[0-9a-f]{6}$/.test(hex)) return hex;
  if (/^#[0-9a-f]{3}$/.test(hex)) {
    const [r, g, b] = hex.slice(1).split("");
    return `#${r}${r}${g}${g}${b}${b}`;
  }
  return "";
}

/** Accepts #rgb, #rrggbb, rgb(), or a CSS var that still carries a hex fallback. */
export function cssColorToHex(value: string, fallback: string): string {
  const trimmed = value.trim();
  const direct = normalizeHex(trimmed);
  if (direct) return direct;
  const embedded = trimmed.match(/#(?:[0-9a-fA-F]{6}|[0-9a-fA-F]{3})\b/);
  if (embedded) {
    const hex = normalizeHex(embedded[0]);
    if (hex) return hex;
  }
  const rgb = trimmed.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/i);
  if (rgb) {
    const channel = (n: string): string => Math.max(0, Math.min(255, Number(n))).toString(16).padStart(2, "0");
    return `#${channel(rgb[1])}${channel(rgb[2])}${channel(rgb[3])}`;
  }
  return fallback;
}

function safeColor(value: string | undefined, fallback: string): string {
  return cssColorToHex(value ?? "", fallback);
}

function bannerTypeOf(value: string | null, backgroundColor: string): BannerType {
  if (value === "top") return "image";
  if (value === "announcement") return "custom";
  if (value && (BANNER_TYPES as readonly string[]).includes(value)) return value as BannerType;
  const bg = backgroundColor.toLowerCase();
  if (bg === "#000" || bg === "#000000") return "image";
  if (bg === "#1f2937") return "custom";
  return "custom";
}

/** http and https only, so a link or image cannot carry a script URL. */
/** Drops the paragraph margin the rich-text editor adds, so a banner stays one strip. */
function compactMessageHtml(html: string): string {
  const doc = new DOMParser().parseFromString(`<div id="banner-message">${html}</div>`, "text/html");
  const root = doc.getElementById("banner-message");
  if (!root) return html;
  root.querySelectorAll("p, div, h1, h2, h3, ul, ol, li").forEach((el) => {
    const current = el.getAttribute("style") ?? "";
    if (!/margin\s*:/.test(current)) el.setAttribute("style", current ? `margin:0;${current}` : "margin:0");
  });
  return root.innerHTML;
}

function safeUrl(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return "";
  try {
    const url = new URL(trimmed);
    if (url.protocol !== "http:" && url.protocol !== "https:") return "";
    return trimmed;
  } catch {
    return "";
  }
}

/**
 * Builds the HTML stored in portal_banner.html. Colours, the image, the link,
 * and the close button are repeated in data attributes. The message is rich
 * text and lives in [data-banner-text], so Edit can put it back in the editor.
 */
export function buildBannerHtml(content: BannerContent, id = ""): string {
  const message = content.message.trim();
  const textColor = safeColor(content.textColor, EMPTY_BANNER_CONTENT.textColor);
  const backgroundColor = safeColor(content.backgroundColor, EMPTY_BANNER_CONTENT.backgroundColor);
  const imageUrl = safeUrl(content.imageUrl);
  const linkUrl = safeUrl(content.linkUrl);
  const closeable = content.closeable;
  const type = content.type;
  const bannerId = id.trim();
  const isImage = type === "image";
  const isNotice = type === "info" || type === "warning" || type === "error" || type === "success";
  const messageHtml = compactMessageHtml(sanitizeRichTextHtml(message));
  const hasMessage = !isBlankHtml(messageHtml) && !isImage;

  const image = isImage && imageUrl
    ? `<img src="${escapeText(imageUrl)}" alt="Banner" style="object-fit:cover;height:100%;width:100%;display:block">`
    : "";
  const linkedImage =
    image && linkUrl
      ? `<a href="${escapeText(linkUrl)}" target="_blank" rel="noopener noreferrer" style="display:block;height:100%">${image}</a>`
      : image;
  const iconColor = safeColor(content.iconColor, textColor);
  const icon = isNotice ? alertIcon(type as keyof typeof ALERT_ICON_PATH, iconColor) : "";
  const text = hasMessage ? `<div data-banner-text style="margin:0;color:${textColor}">${messageHtml}</div>` : "";
  const close = closeable
    ? `<button type="button" data-close-banner="true" aria-label="Close banner" style="position:absolute;top:50%;right:12px;transform:translateY(-50%);background:rgba(0,0,0,.55);border:1px solid rgba(255,255,255,.6);color:#fff;width:24px;height:24px;cursor:pointer;border-radius:4px;z-index:1;">×</button>`
    : "";
  const shell = isImage
    ? `position:relative;background-color:${backgroundColor};color:${textColor};height:3rem;overflow:hidden;`
    : `box-sizing:border-box;position:relative;width:100%;height:3rem;overflow:hidden;display:flex;align-items:center;justify-content:center;flex-wrap:nowrap;gap:8px;padding:0 48px;background-color:${backgroundColor};background-image:none;color:${textColor};font:${isNotice ? "15px/1.2" : "14px/1.2"} -apple-system,BlinkMacSystemFont,Roboto,sans-serif;text-align:center;`;

  return (
    `<div data-banner="1"${bannerId ? ` data-id="${escapeText(bannerId)}"` : ""} data-banner-type="${type}" data-text-color="${escapeText(textColor)}" data-bg="${escapeText(backgroundColor)}" data-image="${escapeText(imageUrl)}" data-link="${escapeText(linkUrl)}" data-closeable="${closeable ? "true" : "false"}" ` +
    `style="${shell}">` +
    `${linkedImage}${icon}${text}${close}</div>`
  );
}

/** Reads a banner saved by buildBannerHtml. Anything else comes back blank. */
export function parseBannerHtml(html: string): BannerContent {
  const root = new DOMParser().parseFromString(html, "text/html").querySelector("[data-banner]");
  if (!root) return { ...EMPTY_BANNER_CONTENT };
  const text = root.querySelector("[data-banner-text]");
  const backgroundColor = safeColor(root.getAttribute("data-bg") ?? "", EMPTY_BANNER_CONTENT.backgroundColor);
  return {
    type: bannerTypeOf(root.getAttribute("data-banner-type"), backgroundColor),
    title: (root.getAttribute("data-title") ?? "").trim(),
    message: text ? text.innerHTML : (root.getAttribute("data-message") ?? ""),
    textColor: safeColor(root.getAttribute("data-text-color") ?? "", EMPTY_BANNER_CONTENT.textColor),
    backgroundColor,
    iconColor: "",
    imageUrl: root.getAttribute("data-image") ?? "",
    linkUrl: root.getAttribute("data-link") ?? "",
    closeable: root.getAttribute("data-closeable") === "true",
  };
}

/** Puts a title saved the old way into the message, so Edit does not drop those words. */
export function messageKeepingTitle(message: string, title: string): string {
  const name = title.replace(/\s+/g, " ").trim();
  if (!name) return message;
  const plain = new DOMParser().parseFromString(message, "text/html").body.textContent ?? "";
  if (plain.includes(name)) return message;
  const safe = `<strong>${escapeText(name)}</strong>`;
  return message.trim() ? `${safe} ${message}` : `<p>${safe}</p>`;
}

/** The one line shown on a collapsed banner. Formatting is dropped so the row stays one line. */
export function bannerSummary(html: string): string {
  const content = parseBannerHtml(html);
  if (content.title) return content.title;
  const text = new DOMParser()
    .parseFromString(content.message, "text/html")
    .body.textContent?.replace(/\s+/g, " ")
    .trim();
  if (text) return text;
  if (content.imageUrl) return "Image banner";
  return "Empty banner";
}
