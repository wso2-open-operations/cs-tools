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

// The SLA report PDF, built the way the CS report's is (CsReportPDF.ts): the
// pages are laid out as sheets (a front page, the content, a thank-you page),
// each sheet is captured on its own inside a small iframe that carries only the
// styles it needs, and jsPDF and html2canvas are called directly (html2pdf.js
// is not a dependency, for the same reason as there). The sheets are built from
// the report data, not copied from the screen, so the PDF always holds every
// case whatever page the on-screen table is on.
import jsPDF from "jspdf";
import { addElementPages } from "./reportPdfCapture";
import { formatPeriodDate } from "../utils/period";
import type { SlaReportCase, SlaReportClock, SlaReportDetails, SlaReportMetric } from "../api/reportTypes";
import logo from "../assets/wso2-logo-orange.png";
import "../styles/CSReportPDF.css";
import "../styles/SLAReportPDF.css";

const DEFAULT_ACCENT = "#fa7b3f";

// A4 landscape. Each sheet is that shape, so a section lands on a page instead
// of being sliced out of one long screenshot (same size the CS report uses).
const PAGE_W = 1120;
const PAGE_H = 790;
// How long a capture waits for one of the page's stylesheets to load in the frame.
const STYLESHEET_WAIT_MS = 5000;

// Names and dates come from the data. Escaping first means a project or case
// name can never be read as markup when the sheets are put together.
function escapeHtml(text: string): string {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

function display(value?: string | null): string {
  const trimmed = value?.trim();
  return trimmed ? escapeHtml(trimmed) : "—";
}

function asPage(el: HTMLElement, fixed = false): HTMLElement {
  el.classList.add("pdf-sheet");
  el.style.width = `${PAGE_W}px`;
  el.style.maxWidth = `${PAGE_W}px`;
  el.style.boxSizing = "border-box";
  el.style.margin = "0";
  if (fixed) {
    el.style.height = `${PAGE_H}px`;
    el.style.overflow = "hidden";
  } else {
    el.style.minHeight = `${PAGE_H}px`;
    el.style.height = "auto";
  }
  return el;
}

function sheet(className: string, html: string, fixed = false): HTMLElement {
  const el = document.createElement("div");
  el.className = className;
  el.innerHTML = html;
  return asPage(el, fixed);
}

const STATUS_CLASS: Record<string, string> = {
  Met: "met",
  Missed: "missed",
  Running: "running",
  Paused: "paused",
  Cancelled: "cancelled",
};

function statusPill(status: string): string {
  const cls = STATUS_CLASS[status];
  return cls ? `<span class="sla-pdf-status ${cls}">${escapeHtml(status)}</span>` : "";
}

function clockCell(clock: SlaReportClock): string {
  const hasTime = !!clock.elapsed?.trim() && clock.elapsed.trim() !== "—";
  const elapsed = hasTime ? display(clock.elapsed) : clock.status ? "Not recorded" : "—";
  return `<td><span class="sla-pdf-elapsed">${elapsed}</span>${statusPill(clock.status)}</td>`;
}

function metricCell(metric: SlaReportMetric): string {
  return `<td><div class="sla-pdf-elapsed">${display(metric.elapsed)}</div>
    <div class="sla-pdf-sub">Target: ${display(metric.target)}</div>
    <div class="sla-pdf-counts">${metric.met} met · ${metric.missed} missed · ${metric.running} running</div></td>`;
}

function totals(data: SlaReportDetails): { cases: number; met: number; missed: number; running: number } {
  const sum = { cases: data.cases.length, met: 0, missed: 0, running: 0 };
  for (const row of data.summary) {
    for (const metric of [row.response, row.workaround, row.resolution]) {
      sum.met += metric.met;
      sum.missed += metric.missed;
      sum.running += metric.running;
    }
  }
  return sum;
}

function tile(label: string, value: number, tone = ""): string {
  return `<div class="sla-pdf-tile"><span class="sla-pdf-tile-label">${label}</span><span class="sla-pdf-tile-value ${tone}">${value}</span></div>`;
}

function frontPage(data: SlaReportDetails, formattedDate: string): HTMLElement {
  const period = `${display(formatPeriodDate(data.periodStart))} to ${display(formatPeriodDate(data.periodEnd))}`;
  return sheet(
    "front-page sla-pdf-front",
    `<img src="${logo}" alt="Logo" class="front-page-logo" />
    <h1 class="header-title">${display(data.projectName)} - SLA Report</h1>
    <p>Accelerating Success Together</p>
    <hr class="section-divider"/>
    <p class="current-date">Project key: ${display(data.projectKey)}</p>
    <p class="current-date">Cases opened: ${period}</p>
    <p class="current-date">Generated on: ${escapeHtml(formattedDate)}</p>`,
    true,
  );
}

function summarySheet(data: SlaReportDetails): HTMLElement {
  const sum = totals(data);
  const rows =
    data.summary.length === 0
      ? `<tr><td colspan="5" class="sla-pdf-empty">No cases were opened in this period.</td></tr>`
      : data.summary
          .map(
            (row) => `<tr>
              <td>${display(row.label)}</td><td>${row.cases}</td>
              ${metricCell(row.response)}${metricCell(row.workaround)}${metricCell(row.resolution)}
            </tr>`,
          )
          .join("");
  return sheet(
    "sla-pdf-body",
    `<h2 class="sla-title">SLA report</h2>
    <p class="sla-pdf-note">How the cases opened in this period did against their SLA targets. The 95th percentile is of the
    clocks that finished. Missed counts clocks past their target, including ones still running.</p>
    <div class="sla-pdf-tiles">
      ${tile("Cases", sum.cases)}${tile("Met", sum.met, "met")}${tile("Missed", sum.missed, "missed")}${tile("Running", sum.running, "running")}
    </div>
    <h2 class="sla-title">95th percentile of elapsed time</h2>
    <table class="sla-pdf-table sla-pdf-summary">
      <colgroup><col class="c-label"/><col class="c-cases"/><col/><col/><col/></colgroup>
      <thead><tr><th>Case type / severity</th><th>Cases</th><th>Response</th><th>Workaround</th><th>Resolution</th></tr></thead>
      <tbody>${rows}</tbody>
    </table>`,
  );
}

function caseRow(row: SlaReportCase): string {
  return `<tr>
    <td>${display(row.caseNumber)}</td><td>${display(row.caseType)}</td><td>${display(row.severity)}</td>
    <td>${display(row.state)}</td><td>${display(formatPeriodDate(row.openedOn))}</td>
    ${clockCell(row.response)}${clockCell(row.workaround)}${clockCell(row.resolution)}
  </tr>`;
}

function casesSheet(data: SlaReportDetails): HTMLElement {
  const rows =
    data.cases.length === 0
      ? `<tr><td colspan="8" class="sla-pdf-empty">No cases were opened in this period.</td></tr>`
      : data.cases.map(caseRow).join("");
  return sheet(
    "sla-pdf-body",
    `<h2 class="sla-title">Cases</h2>
    <table class="sla-pdf-table sla-pdf-cases">
      <colgroup><col class="c-case"/><col class="c-type"/><col class="c-severity"/><col class="c-state"/><col class="c-opened"/><col/><col/><col/></colgroup>
      <thead><tr><th>Case</th><th>Type</th><th>Severity</th><th>State</th><th>Opened</th><th>Response</th><th>Workaround</th><th>Resolution</th></tr></thead>
      <tbody>${rows}</tbody>
    </table>`,
  );
}

function thankPage(): HTMLElement {
  return sheet("thank-page sla-pdf-thanks", `<h1>Thanks!</h1><hr class="section-divider"/>`, true);
}

/** The pages of the PDF, in order. Exported so a test can read what is on them. */
export function buildSlaReportSheets(data: SlaReportDetails, generatedOn = new Date()): HTMLElement[] {
  const formattedDate = generatedOn.toLocaleDateString(undefined, { year: "numeric", month: "long", day: "numeric" });
  return [frontPage(data, formattedDate), summarySheet(data), casesSheet(data), thankPage()];
}

/** Keeps the file name to characters every file system accepts. */
export function slaReportFileName(data: SlaReportDetails): string {
  const key = (data.projectKey || "project").replace(/[^A-Za-z0-9_-]/g, "_");
  return `SLA_Report_${key}_${data.periodStart}_to_${data.periodEnd}.pdf`;
}

export async function slaReportPDF(data: SlaReportDetails, accent = DEFAULT_ACCENT): Promise<void> {
  const safeAccent = /^#[0-9a-f]{3,8}$/i.test(accent) ? accent : DEFAULT_ACCENT;
  const sheets = buildSlaReportSheets(data);
  sheets.forEach((el) => {
    el.querySelectorAll<HTMLElement>(".section-divider").forEach((divider) => {
      divider.style.borderTopColor = safeAccent;
    });
  });

  const pdf = new jsPDF("l", "mm", "a4");
  // html2canvas clones the element's whole document. Capturing from the app
  // page re-fetches every icon and chart SVG, once per page. This frame holds
  // one sheet and the styles it needs, so each capture stays that small.
  const frame = document.createElement("iframe");
  frame.setAttribute("data-html2canvas-ignore", "true");
  frame.style.cssText = `position:fixed;left:-99999px;top:0;width:${PAGE_W}px;height:${PAGE_H}px;border:0;`;
  document.body.appendChild(frame);
  const frameDoc = frame.contentDocument;
  if (!frameDoc) {
    document.body.removeChild(frame);
    throw new Error("Could not prepare the PDF.");
  }
  frameDoc.open();
  frameDoc.write('<!DOCTYPE html><html><head></head><body style="margin:0"></body></html>');
  frameDoc.close();
  const base = frameDoc.createElement("base");
  base.href = document.baseURI;
  frameDoc.head.appendChild(base);
  const sheetsLoaded: Promise<void>[] = [];
  document.querySelectorAll('style, link[rel="stylesheet"]').forEach((node) => {
    const copy = node.cloneNode(true) as HTMLElement;
    // A cloned <link> fetches its sheet again inside the frame; capturing
    // before it lands draws the pages unstyled. One that never answers is
    // given up on rather than holding the download.
    if (copy instanceof HTMLLinkElement) {
      sheetsLoaded.push(
        new Promise((resolve) => {
          const done = () => {
            clearTimeout(timer);
            resolve();
          };
          const timer = setTimeout(done, STYLESHEET_WAIT_MS);
          copy.addEventListener("load", done, { once: true });
          copy.addEventListener("error", done, { once: true });
        }),
      );
    }
    frameDoc.head.appendChild(copy);
  });
  frameDoc.documentElement.style.setProperty("--report-accent", safeAccent);

  try {
    await Promise.all(sheetsLoaded);
    let started = false;
    for (const el of sheets) {
      frameDoc.body.replaceChildren(el);
      started = await addElementPages(pdf, el, started, {
        width: PAGE_W,
        windowWidth: PAGE_W,
        pageCssHeight: PAGE_H,
      });
    }
    pdf.save(slaReportFileName(data));
  } finally {
    document.body.removeChild(frame);
  }
}
