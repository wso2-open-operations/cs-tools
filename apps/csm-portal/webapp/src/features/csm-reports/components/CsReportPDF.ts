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

// Ported from the source app's components/CSReportPDF.tsx. The DOM-building
// logic below (front page, agenda, one section per widget, SLA table, thank-
// you page) is unchanged from the source — it's framework-agnostic DOM
// manipulation. The ONE thing that changed is how that built DOM becomes a
// PDF: the source hands it to `html2pdf.js`'s `.from(...).set(...).save()`,
// which internally rasterizes with html2canvas and paginates with jsPDF.
// `html2pdf.js` carries a known critical XSS CVE and is deliberately not a
// dependency here — this calls html2canvas + jsPDF directly instead (append
// the built content off-screen, rasterize, slice into page-height images,
// same technique SLAReport/TimelogsReport already use for their own PDF
// export), which is exactly what html2pdf.js does under the hood.
import html2canvas from "html2canvas";
import jsPDF from "jspdf";
import type { Dayjs } from "dayjs";
import dayjs from "dayjs";
import logo from "../assets/wso2-logo-orange.png";
import "../styles/CSReportPDF.css";

/* eslint-disable @typescript-eslint/no-explicit-any -- mirrors the source's own loosely-typed report records */

// Business Overview and Extra Notes are free-text textareas a CS engineer
// types into, then rendered here via innerHTML (so the \n -> <br> conversion
// below actually produces line breaks). Escaping first means that raw text
// can never be interpreted as markup -- without it, a literal "<img
// onerror=...>" typed into either textarea would execute in every viewer's
// browser when the report renders.
function escapeHtml(text: string): string {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

// A5 landscape is 210mm by 148mm. Each sheet is that shape, so a section
// lands on a page instead of being sliced out of one long screenshot.
const PAGE_W = 1120;
const PAGE_H = 790;

function asPage(el: HTMLElement, fixed = false) {
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
}

function hexToRgb(color: string): [number, number, number] {
  const match = color.trim().match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i);
  if (!match) return [96, 125, 139];
  const hex = match[1].length === 3 ? match[1].split("").map((c) => c + c).join("") : match[1];
  return [parseInt(hex.slice(0, 2), 16), parseInt(hex.slice(2, 4), 16), parseInt(hex.slice(4, 6), 16)];
}

// One graph per page, as a screenshot of the chart already on screen. The
// vector conversion dropped legends and warped the recharts drawing.
async function addChartPage(
  pdf: jsPDF,
  chart: HTMLElement | null,
  title: string,
  started: boolean,
  accent: string,
  frameDoc: Document,
): Promise<boolean> {
  if (started) pdf.addPage();
  const pageW = pdf.internal.pageSize.getWidth();
  const pageH = pdf.internal.pageSize.getHeight();
  const [accentR, accentG, accentB] = hexToRgb(accent);
  pdf.setFont("helvetica", "bold");
  pdf.setFontSize(16);
  pdf.setTextColor(accentR, accentG, accentB);
  pdf.text(title, 14, 16);

  if (!chart) {
    pdf.setFont("helvetica", "normal");
    pdf.setFontSize(12);
    pdf.setTextColor(80, 80, 80);
    pdf.text("No data to show", 14, 28);
    return true;
  }

  const clone = chart.cloneNode(true) as HTMLElement;
  clone.querySelector("p")?.remove();
  const width = Math.max(chart.getBoundingClientRect().width, 640);
  clone.style.width = `${width}px`;
  clone.style.maxWidth = `${width}px`;
  clone.style.background = "#ffffff";
  clone.style.margin = "0";
  frameDoc.body.replaceChildren(clone);
  const shot = await html2canvas(clone, {
    scale: 2,
    backgroundColor: "#ffffff",
    logging: false,
    imageTimeout: 2000,
  });
  const boxW = pageW - 28;
  const boxH = pageH - 28;
  const imgRatio = shot.width / Math.max(shot.height, 1);
  let drawW = boxW;
  let drawH = drawW / imgRatio;
  if (drawH > boxH) {
    drawH = boxH;
    drawW = drawH * imgRatio;
  }
  pdf.addImage(shot.toDataURL("image/jpeg", 0.92), "JPEG", 14 + (boxW - drawW) / 2, 22, drawW, drawH);
  return true;
}

// The on-screen EOL block is an accordion. Copying that markup into the PDF
// keeps the expand icon and can hide the dates, so the sheet is rebuilt from
// the data attributes on each product.
function eolPdfHtml(element: HTMLElement): string {
  const title = element.querySelector(".widget-title")?.textContent?.trim() || "Product versions and EOL";
  const products = Array.from(element.querySelectorAll<HTMLElement>("[data-eol-product]"));
  if (products.length === 0) {
    return `<h2 class="widget-title">${escapeHtml(title)}</h2><p>No data to show</p>`;
  }
  let html = `<h2 class="widget-title">${escapeHtml(title)}</h2>`;
  let lastEnv = "";
  for (const node of products) {
    const env = node.dataset.eolEnvironment ?? "";
    if (env !== lastEnv) {
      lastEnv = env;
      html += `<h3 class="eol-env">${escapeHtml(env ? `${env} environment` : "Environment")}</h3>`;
    }
    const name = node.dataset.eolProduct ?? "";
    const version = node.dataset.eolVersion ?? "";
    const status = node.dataset.eolStatus ?? "";
    const earliest = node.dataset.eolEarliest || "—";
    const eol = node.dataset.eolDate || "—";
    const heading = version ? `${name} v${version}` : name;
    html += `<div class="eol-card"><div class="eol-card-head"><span class="eol-name">${escapeHtml(heading)}</span>${
      status ? `<span class="eol-status">${escapeHtml(status)}</span>` : ""
    }</div><div class="eol-dates"><div><span class="eol-label">Earliest support EOL</span><span>${escapeHtml(
      earliest,
    )}</span></div><div><span class="eol-label">Support EOL</span><span>${escapeHtml(eol)}</span></div></div></div>`;
  }
  return html;
}

function generateAgendaPage(widgets: { selector: string; subTopic?: string }[], pdfContent: HTMLElement) {
  const agendaPage = document.createElement("div");
  agendaPage.classList.add("agenda-page");

  const agendaTitle = document.createElement("h1");
  agendaTitle.textContent = "Table of Contents";
  agendaPage.appendChild(agendaTitle);

  widgets.forEach(({ subTopic }) => {
    if (subTopic) {
      const agendaItem = document.createElement("p");
      agendaItem.textContent = subTopic;
      agendaItem.classList.add("agenda-item");
      agendaPage.appendChild(agendaItem);
    }
  });

  if (agendaPage.children.length > 1) {
    asPage(agendaPage, true);
    pdfContent.appendChild(agendaPage);
  }
}

function buildPdfContent(
  contentElement: HTMLElement,
  account: string,
  records: any[],
  notesText: string,
  notesImage: string,
  deploymentImage: string,
  businessOverviewText: string,
): HTMLElement {
  const pdfContent = document.createElement("div");

  const frontPage = document.createElement("div");
  frontPage.classList.add("front-page");
  const formattedDate = new Date().toLocaleDateString(undefined, {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
  frontPage.innerHTML = `
    <img src="${logo}" alt="Logo" class="front-page-logo" />
    <h1 class="header-title">${escapeHtml(account)} - Customer Success Report </h1>
    <p>Accelerating Success Together</p>
    <hr class="section-divider"/>
    <p class="current-date">Generated on: ${formattedDate}</p>
  `;
  asPage(frontPage, true);
  pdfContent.appendChild(frontPage);

  const widgets = [
    { selector: ".business-overview-section", subTopic: "Client Business Overview" },
    { selector: ".deployment-image-section", subTopic: "Current Deployment" },
    { selector: ".subscription-widget", subTopic: "Support Update" },
    { selector: ".current-products-widget" },
    { selector: ".product-versions-widget" },
    { selector: ".cases-charts-widget" },
    { selector: ".cases-volume-widget" },
    { selector: ".incident-sla-compliance-widget" },
    { selector: ".product-updates-widget" },
    { selector: ".engagements-widget", subTopic: "Engagements" },
    { selector: ".extra-notes-section", subTopic: "Notes" },
  ];

  generateAgendaPage(widgets, pdfContent);

  widgets.forEach(({ selector, subTopic }) => {
    const element = contentElement.querySelector(selector);
    if (!element) return;

    if (
      (selector === ".extra-notes-section" && !notesText && !notesImage) ||
      (selector === ".deployment-image-section" && !deploymentImage)
    ) {
      return;
    }
    if (subTopic && selector === ".business-overview-section" && businessOverviewText === "") {
      return;
    }

    if (subTopic) {
      if (element.innerHTML.trim()) {
        const pageBreak = document.createElement("div");
        pageBreak.classList.add("page-break");
        pdfContent.appendChild(pageBreak);

        const widgetTitlePage = document.createElement("div");
        widgetTitlePage.classList.add("sub-topic-page");
        widgetTitlePage.innerHTML = `
              <h1 class="header-title">${subTopic}</h1>
              <hr class="section-divider"/>
          `;
        asPage(widgetTitlePage, true);
        pdfContent.appendChild(widgetTitlePage);
      }
    }

    const widgetContainer = document.createElement("div");
    widgetContainer.classList.add("page-break");

    if (selector === ".cases-charts-widget" || selector === ".cases-volume-widget") {
      // Drawn later as one screenshot per graph.
      widgetContainer.classList.add("pdf-chart-slot");
      widgetContainer.dataset.source = selector;
      widgetContainer.dataset.title = selector === ".cases-charts-widget" ? "Cases" : "Case volume by month";
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".subscription-widget") {
      const dataContent = element.innerHTML.trim();
      widgetContainer.classList.remove("page-break");
      widgetContainer.innerHTML = `<div class="subscription-content"><div>${dataContent || "<p>No data to show</p>"}</div></div>`;
      asPage(widgetContainer);
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".current-products-widget") {
      const dataContent = element.innerHTML.trim();
      widgetContainer.innerHTML = `<div>${dataContent || "<p>No data to show</p>"}</div>`;
      asPage(widgetContainer);
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".product-versions-widget") {
      widgetContainer.innerHTML = `<div class="product-versions-content eol-pdf">${eolPdfHtml(element)}</div>`;
      asPage(widgetContainer);
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".incident-sla-compliance-widget") {
      const dataContent = element.innerHTML.trim();
      widgetContainer.innerHTML = `<div class="incident-sla-compliance-widget"><div>${dataContent || "<p>No data to show</p>"}</div></div>`;
      asPage(widgetContainer);
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".product-updates-widget") {
      const dataContent = element.innerHTML.trim();
      widgetContainer.innerHTML = `<div>${dataContent || "<p>No data to show</p>"}</div>`;
      asPage(widgetContainer);
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".engagements-widget") {
      const clone = element.cloneNode(true) as HTMLElement;
      clone.querySelectorAll<HTMLElement>(".engagement-description").forEach((node) => {
        node.style.display = "block";
        node.style.setProperty("-webkit-line-clamp", "unset");
        node.style.setProperty("-webkit-box-orient", "unset");
        node.style.overflow = "visible";
        node.style.maxHeight = "none";
      });
      clone.querySelectorAll(".engagement-show-more").forEach((node) => node.remove());
      const dataContent = clone.innerHTML.trim();
      widgetContainer.classList.remove("page-break");
      widgetContainer.innerHTML = `<div class="engagements-content"><div>${dataContent || "<p>No data to show</p>"}</div></div>`;
      asPage(widgetContainer);
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".business-overview-section" && businessOverviewText) {
      const title = element.querySelector(".widget-title")?.textContent || "Business Overview";
      const textarea = element.querySelector("textarea") as HTMLTextAreaElement | null;
      const formattedText = textarea ? escapeHtml(textarea.value).replace(/\n/g, "<br>") : "No data to show";
      widgetContainer.innerHTML = `<h2 class="widget-title">${title}</h2><p>${formattedText}</p>`;
      asPage(widgetContainer);
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".deployment-image-section") {
      const title = element.querySelector(".widget-title")?.textContent || "Deployment";
      const imageElement = element.querySelector("img");
      if (imageElement) {
        const deploymentElement = document.createElement("div");
        deploymentElement.classList.add("styled-text");
        deploymentElement.innerHTML = `<h2 class="widget-title">${title}</h2>`;

        const imageWrapperDiv = document.createElement("div");
        imageWrapperDiv.style.display = "flex";
        imageWrapperDiv.style.justifyContent = "center";
        imageWrapperDiv.style.alignItems = "center";
        imageWrapperDiv.style.width = "100%";
        imageWrapperDiv.style.maxHeight = "620px";
        const deploymentPicture = imageElement.cloneNode(true) as HTMLImageElement;
        deploymentPicture.style.maxWidth = "100%";
        deploymentPicture.style.maxHeight = "620px";
        deploymentPicture.style.objectFit = "contain";
        imageWrapperDiv.appendChild(deploymentPicture);

        deploymentElement.appendChild(imageWrapperDiv);
        asPage(deploymentElement, true);
        pdfContent.appendChild(deploymentElement);
      }
    } else if (selector === ".extra-notes-section") {
      const title = element.querySelector(".widget-title")?.textContent || "Notes";
      const image = element.querySelector("img") as HTMLImageElement | null;
      const textarea = element.querySelector("textarea") as HTMLTextAreaElement | null;
      const formattedText = textarea ? escapeHtml(textarea.value).replace(/\n/g, "<br>") : "No data to show";

      const textElement = document.createElement("div");
      textElement.classList.add("styled-text");
      textElement.innerHTML = `<h2 class="widget-title">${title}</h2><p>${formattedText}</p>`;
      asPage(textElement);
      pdfContent.appendChild(textElement);

      if (image) {
        const imageElement = document.createElement("div");
        imageElement.classList.add("page-break");
        imageElement.innerHTML = `<h2 class="widget-title">${title}</h2>`;

        const imageWrapperDiv = document.createElement("div");
        imageWrapperDiv.style.display = "flex";
        imageWrapperDiv.style.justifyContent = "center";
        imageWrapperDiv.style.alignItems = "center";
        imageWrapperDiv.style.width = "100%";
        imageWrapperDiv.style.maxHeight = "620px";

        const clonedImage = new Image();
        clonedImage.src = image.src;
        clonedImage.style.maxWidth = "100%";
        clonedImage.style.maxHeight = "620px";
        clonedImage.style.objectFit = "contain";
        imageWrapperDiv.appendChild(clonedImage);
        imageElement.appendChild(imageWrapperDiv);
        asPage(imageElement, true);
        pdfContent.appendChild(imageElement);
      }
    }
  });

  const pageBreak = document.createElement("div");
  pageBreak.classList.add("page-break");
  pdfContent.appendChild(pageBreak);

  const recordsPage = document.createElement("div");
  recordsPage.classList.add("records-page");
  recordsPage.innerHTML = `<h1> SLA & Other Information </h1><hr class="section-divider"/>`;
  asPage(recordsPage, true);
  pdfContent.appendChild(recordsPage);

  const slaTablePage = document.createElement("div");
  slaTablePage.classList.add("sla-table-page");
  const slaTitle = document.createElement("h2");
  slaTitle.classList.add("sla-title");
  slaTitle.textContent = "SLA Report";
  slaTablePage.appendChild(slaTitle);

  const slaTable = document.createElement("table");
  slaTable.classList.add("sla-table");
  if (records.length === 0) {
    slaTable.innerHTML = `
        <thead><tr><th>Task</th><th>SLA Definition</th><th>Business Elapsed Percentage(%)</th></tr></thead>
        <tbody><tr><td colspan="3" style="text-align: center; color: gray;">No SLA data available</td></tr></tbody>
    `;
  } else {
    slaTable.innerHTML = `
        <thead><tr><th>Task</th><th>SLA Definition</th><th>Business Elapsed Percentage(%)</th></tr></thead>
        <tbody>
            ${records
              .map(
                (record: { task: string; slaDefinition: string; businessElapsedPercentage: string }) => `
                <tr><td>${escapeHtml(record.task)}</td><td>${escapeHtml(record.slaDefinition)}</td><td>${escapeHtml(record.businessElapsedPercentage)}</td></tr>
            `,
              )
              .join("")}
        </tbody>
    `;
  }
  slaTablePage.appendChild(slaTable);
  asPage(slaTablePage);
  pdfContent.appendChild(slaTablePage);

  const pageBreak1 = document.createElement("div");
  pageBreak1.classList.add("page-break");
  pdfContent.appendChild(pageBreak1);

  const thankPage = document.createElement("div");
  thankPage.classList.add("thank-page");
  thankPage.innerHTML = `<h1>Thanks!</h1><hr class="section-divider"/>`;
  asPage(thankPage, true);
  pdfContent.appendChild(thankPage);

  return pdfContent;
}

// Preview images are blob: URLs. html2canvas reloads those with
// crossOrigin and the resulting canvas is tainted, so toDataURL throws.
// Read them into data URLs first — that is what the earlier uploader stored.
async function inlineBlobImages(root: ParentNode): Promise<void> {
  const images = Array.from(root.querySelectorAll("img"));
  await Promise.all(
    images.map(async (img) => {
      if (!img.src.startsWith("blob:")) return;
      const response = await fetch(img.src);
      const blob = await response.blob();
      const dataUrl = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(String(reader.result));
        reader.onerror = () => reject(reader.error);
        reader.readAsDataURL(blob);
      });
      img.src = dataUrl;
    }),
  );
}

export async function csReportPDF(
  contentId: string,
  account: string,
  key: string,
  records: any[],
  from: Dayjs | null,
  to: Dayjs | null,
  notesText: string,
  notesImage: string,
  deploymentImage: string,
  businessOverviewText: string,
  accent = "#fa7b3f",
): Promise<void> {
  const contentElement = document.getElementById(contentId);
  if (!contentElement) {
    console.error(`Element with id "${contentId}" not found.`);
    return;
  }

  const pdfContent = buildPdfContent(
    contentElement,
    account,
    records,
    notesText,
    notesImage,
    deploymentImage,
    businessOverviewText,
  );

  await inlineBlobImages(pdfContent);
  const safeAccent = /^#[0-9a-f]{3,8}$/i.test(accent) ? accent : "#fa7b3f";
  pdfContent.querySelectorAll<HTMLElement>(".section-divider").forEach((el) => {
    el.style.borderTopColor = safeAccent;
  });
  pdfContent.querySelectorAll<HTMLElement>(".sla-title").forEach((el) => {
    el.style.color = safeAccent;
  });
  const agenda = pdfContent.querySelector(".agenda-page");
  if (agenda) {
    const bullet = document.createElement("style");
    bullet.textContent = `.agenda-item::before { color: ${safeAccent} !important; }`;
    agenda.prepend(bullet);
  }

  const pdf = new jsPDF("l", "mm", "a4");
  const pdfW = pdf.internal.pageSize.getWidth();
  const pdfH = pdf.internal.pageSize.getHeight();
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
  document.querySelectorAll('style, link[rel="stylesheet"]').forEach((node) => {
    frameDoc.head.appendChild(node.cloneNode(true));
  });
  frameDoc.documentElement.style.setProperty("--report-accent", safeAccent);

  try {
    let started = false;
    for (const child of Array.from(pdfContent.children)) {
      if (!(child instanceof HTMLElement)) continue;
      if (child.classList.contains("pdf-chart-slot")) {
        const source = contentElement.querySelector(child.dataset.source ?? "");
        const charts = source ? Array.from(source.querySelectorAll<HTMLElement>(".report-chart")) : [];
        const sectionTitle = child.dataset.title || "Chart";
        if (charts.length === 0) {
          started = await addChartPage(pdf, null, sectionTitle, started, safeAccent, frameDoc);
        } else {
          for (const chart of charts) {
            const title = chart.querySelector("p")?.textContent?.trim() || sectionTitle;
            started = await addChartPage(pdf, chart, title, started, safeAccent, frameDoc);
          }
        }
        continue;
      }
      if (!child.classList.contains("pdf-sheet")) continue;
      const sheet = child;
      frameDoc.body.replaceChildren(sheet);
      const canvas = await html2canvas(sheet, {
        scale: 2,
        backgroundColor: "#ffffff",
        logging: false,
        imageTimeout: 2000,
        width: PAGE_W,
        windowWidth: PAGE_W,
      });
      const slicePx = Math.max(1, Math.round(PAGE_H * (canvas.height / Math.max(sheet.offsetHeight, 1))));
      let y = 0;
      do {
        const h = Math.min(slicePx, canvas.height - y);
        if (h <= 1) break;
        const slice = document.createElement("canvas");
        slice.width = canvas.width;
        slice.height = h;
        const ctx = slice.getContext("2d");
        if (!ctx) break;
        ctx.fillStyle = "#ffffff";
        ctx.fillRect(0, 0, slice.width, slice.height);
        ctx.drawImage(canvas, 0, y, canvas.width, h, 0, 0, canvas.width, h);
        if (started) pdf.addPage();
        started = true;
        const drawH = (h / canvas.width) * pdfW;
        pdf.addImage(slice.toDataURL("image/jpeg", 0.92), "JPEG", 0, 0, pdfW, Math.min(drawH, pdfH));
        y += h;
      } while (y < canvas.height - 2);
    }

    const formattedFrom = from ? dayjs(from).format("YYYY-MM-DD") : "UnknownStart";
    const formattedTo = to ? dayjs(to).format("YYYY-MM-DD") : "UnknownEnd";
    pdf.save(`CS_Report_${key}_${formattedFrom}_to_${formattedTo}.pdf`);
  } finally {
    document.body.removeChild(frame);
  }
}
