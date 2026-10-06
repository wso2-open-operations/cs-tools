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

import { createTheme } from "@mui/material/styles";

// The SLA and Timelogs report previews (#sla-report / #timelogs-report) are
// printable documents captured by html2canvas for PDF export (see
// handleDownloadPDF in each page) — the same reason the CS report's own
// image-resize canvas fills with a literal white background. A print
// preview stays light "paper" regardless of the app's own light/dark mode,
// the same way a PDF viewer or print dialog never goes dark just because
// the host app did. Without this, SplShell's live dark-mode ThemeProvider
// would make the report's own Paper/Table defaults follow the app's current
// mode while the report's ported inline styles (e.g. its "#f7f7f6" page
// background) stay fixed-light — an inconsistent, half-dark, half-light mix
// worse than either extreme. Wrapping the report's own subtree in this
// fixed-light theme keeps it a single, intentional light document instead.
export const reportPaperTheme = createTheme({ palette: { mode: "light" } });
