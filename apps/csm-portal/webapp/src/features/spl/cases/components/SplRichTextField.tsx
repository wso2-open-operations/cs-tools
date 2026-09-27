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

// FLAG (fidelity trade-off, needs a follow-up decision): the source app's
// worknote composer (apps/support-portal-lite/webapp's own
// features/spl/cases/components/SplRichTextField.tsx) uses the
// `react-quill-new` rich-text editor (bold/italic/underline/strike,
// headers, lists). That package is NOT a dependency of this app
// (csm-portal/webapp) and this port's directive explicitly disallows
// touching package.json (a shared file other concurrent SPL-domain ports
// also touch — adding it here risks a merge conflict). This is a plain
// multi-line TextField instead: functionally equivalent (submits a
// worknote), but with NO rich-text formatting toolbar, a real UI
// regression from the source app.
//
// Two ways to close this gap later, neither done here:
//   1. Add `react-quill-new` (`^2.x`, matching apps/support-portal-lite/
//      webapp's package.json) to this app's package.json centrally.
//   2. Wire up this app's own existing Lexical-based rich text editor
//      (src/components/rich-text-editor/) instead — no new dependency,
//      but a larger integration (its own toolbar/plugin config) than fits
//      this pass.
import { TextField } from "@wso2/oxygen-ui";

export default function SplRichTextField({
  value,
  onChange,
}: {
  value: string;
  onChange: (html: string) => void;
}) {
  // value/onChange still carry HTML (EMPTY_NOTE = "<p><br></p>", see
  // SplCaseDetailPage.tsx) to keep this a drop-in swap for the real editor
  // later — this plain field just treats it as opaque text for now, wrapped
  // in a <p> on submit (see addWorkNote in SplCaseDetailPage.tsx).
  return (
    <TextField
      fullWidth
      multiline
      minRows={4}
      placeholder="Add a work note…"
      value={value === "<p><br></p>" ? "" : value}
      onChange={(e) => onChange(e.target.value)}
    />
  );
}
