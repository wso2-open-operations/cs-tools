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
        pdfContent.appendChild(widgetTitlePage);
      }
    }

    const widgetContainer = document.createElement("div");
    widgetContainer.classList.add("page-break");

    if (selector === ".cases-charts-widget" || selector === ".cases-volume-widget") {
      const heading = selector === ".cases-charts-widget" ? "Cases" : "Case Volume by Quarters";
      widgetContainer.innerHTML = `<h2 class="widget-title">${heading}</h2>`;
      // recharts renders SVG, not <canvas> (chart.js's rasterizable surface
      // the source relied on) — capture the chart containers themselves
      // instead of hunting for a <canvas>, so html2canvas rasterizes the SVG
      // in place when the whole pdfContent tree is captured below.
      const chartsContainer = document.createElement("div");
      chartsContainer.classList.add(selector === ".cases-charts-widget" ? "charts-container" : "trend-chart-container");
      const charts = element.querySelectorAll("svg");
      if (charts.length > 0) {
        charts.forEach((svg) => chartsContainer.appendChild(svg.cloneNode(true)));
      } else {
        chartsContainer.innerHTML = "<p>No data to show</p>";
      }
      widgetContainer.appendChild(chartsContainer);
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".subscription-widget") {
      const dataContent = element.innerHTML.trim();
      widgetContainer.classList.remove("page-break");
      widgetContainer.innerHTML = `<div class="subscription-content"><div>${dataContent || "<p>No data to show</p>"}</div></div>`;
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".current-products-widget") {
      const dataContent = element.innerHTML.trim();
      widgetContainer.innerHTML = `<div>${dataContent || "<p>No data to show</p>"}</div>`;
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".product-versions-widget") {
      const dataContent = element.innerHTML.trim();
      widgetContainer.innerHTML = `<div class="product-versions-content"><div>${dataContent || "<p>No data to show</p>"}</div></div>`;
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".incident-sla-compliance-widget") {
      const dataContent = element.innerHTML.trim();
      widgetContainer.innerHTML = `<div class="incident-sla-compliance-widget"><div>${dataContent || "<p>No data to show</p>"}</div></div>`;
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".product-updates-widget") {
      const dataContent = element.innerHTML.trim();
      widgetContainer.innerHTML = `<div>${dataContent || "<p>No data to show</p>"}</div>`;
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".engagements-widget") {
      const dataContent = element.innerHTML.trim();
      widgetContainer.classList.remove("page-break");
      widgetContainer.innerHTML = `<div class="engagements-content"><div>${dataContent || "<p>No data to show</p>"}</div></div>`;
      pdfContent.appendChild(widgetContainer);
    } else if (selector === ".business-overview-section" && businessOverviewText) {
      const title = element.querySelector(".widget-title")?.textContent || "Business Overview";
      const textarea = element.querySelector("textarea") as HTMLTextAreaElement | null;
      const formattedText = textarea ? escapeHtml(textarea.value).replace(/\n/g, "<br>") : "No data to show";
      widgetContainer.innerHTML = `<h2 class="widget-title">${title}</h2><p>${formattedText}</p>`;
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
        imageWrapperDiv.style.height = "80mm";
        imageWrapperDiv.style.paddingTop = "50mm";
        imageWrapperDiv.appendChild(imageElement.cloneNode(true));

        deploymentElement.appendChild(imageWrapperDiv);
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
      pdfContent.appendChild(textElement);

      if (image) {
        const imageElement = document.createElement("div");
        imageElement.classList.add("page-break");
        imageElement.innerHTML = `<h2 class="widget-title">${title}</h2>`;

        const imageWrapperDiv = document.createElement("div");
        imageWrapperDiv.style.display = "flex";
        imageWrapperDiv.style.justifyContent = "center";
        imageWrapperDiv.style.alignItems = "center";
        imageWrapperDiv.style.height = "60mm";
        imageWrapperDiv.style.paddingTop = "67mm";

        const clonedImage = new Image();
        clonedImage.src = image.src;
        imageWrapperDiv.appendChild(clonedImage);
        imageElement.appendChild(imageWrapperDiv);
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
  pdfContent.appendChild(slaTablePage);

  const pageBreak1 = document.createElement("div");
  pageBreak1.classList.add("page-break");
  pdfContent.appendChild(pageBreak1);

  const thankPage = document.createElement("div");
  thankPage.classList.add("thank-page");
  thankPage.innerHTML = `<h1>Thanks!</h1><hr class="section-divider"/>`;
  pdfContent.appendChild(thankPage);

  return pdfContent;
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

  // Off-screen but laid out (html2canvas needs real layout, not display:none).
  pdfContent.style.position = "fixed";
  pdfContent.style.top = "0";
  pdfContent.style.left = "-99999px";
  pdfContent.style.width = "1000px";
  document.body.appendChild(pdfContent);

  try {
    const canvas = await html2canvas(pdfContent, { scale: 3, backgroundColor: "#ffffff", useCORS: true });
    // A5 landscape, same as the source's html2pdf jsPDF option.
    const pdf = new jsPDF("l", "mm", "a5");
    const pdfWidth = pdf.internal.pageSize.getWidth();
    const pdfHeight = pdf.internal.pageSize.getHeight();
    const imgData = canvas.toDataURL("image/jpeg", 0.98);
    const imgProps = pdf.getImageProperties(imgData);
    const imgHeight = (imgProps.height * pdfWidth) / imgProps.width;

    let heightLeft = imgHeight;
    let position = 0;
    pdf.addImage(imgData, "JPEG", 0, position, pdfWidth, imgHeight);
    heightLeft -= pdfHeight;
    while (heightLeft > 0) {
      position -= pdfHeight;
      pdf.addPage();
      pdf.addImage(imgData, "JPEG", 0, position, pdfWidth, imgHeight);
      heightLeft -= pdfHeight;
    }

    const formattedFrom = from ? dayjs(from).format("YYYY-MM-DD") : "UnknownStart";
    const formattedTo = to ? dayjs(to).format("YYYY-MM-DD") : "UnknownEnd";
    pdf.save(`CS_Report_${key}_${formattedFrom}_to_${formattedTo}.pdf`);
  } finally {
    document.body.removeChild(pdfContent);
  }
}
