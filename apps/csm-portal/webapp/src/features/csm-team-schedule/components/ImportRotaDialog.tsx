/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Link,
  Typography,
} from "@wso2/oxygen-ui";
import { useRef, useState, type ChangeEvent, type JSX } from "react";
import { useImportRota, type ImportRotaResult } from "../api/useImportRota";
import { buildRotaImportTemplate, parseRotaImportFile, type ParsedRotaImport } from "../utils/rotaImport";
import type { ScheduleCatalogue } from "../types";

/** The largest file read; the server takes at most 1 MiB. */
const MAX_FILE_BYTES = 1 << 20;

/** How many assignments with skipped days are listed before the rest are counted. */
const SHOWN_SKIPS = 6;

function readText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ""));
    reader.onerror = () => reject(reader.error ?? new Error("The file could not be read."));
    reader.readAsText(file);
  });
}

function downloadTemplate(rotaCode: string, catalogue: ScheduleCatalogue | undefined): void {
  if (typeof URL.createObjectURL !== "function") return;
  const template = buildRotaImportTemplate(rotaCode, catalogue);
  const blob = new Blob([JSON.stringify(template, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${rotaCode.toLowerCase()}-import-template.json`;
  a.click();
  URL.revokeObjectURL(url);
}

function plural(n: number, one: string, many = `${one}s`): string {
  return `${n} ${n === 1 ? one : many}`;
}

function absenceNote(r: ImportRotaResult): string {
  const parts = [plural(r.absencesCreated, "absence")];
  if (r.absencesRemoved > 0) parts.push(`${plural(r.absencesRemoved, "earlier absence")} covered`);
  if (r.absencesTrimmed > 0) parts.push(`${plural(r.absencesTrimmed, "earlier absence")} shortened`);
  return parts.join(", ");
}

function summary(r: ImportRotaResult): string {
  const replaced = r.cleared > 0 ? `, ${plural(r.cleared, "earlier assignment")} replaced` : "";
  const leave = (r.leaveWrittenOver ?? 0) > 0 ? `, ${plural(r.leaveWrittenOver ?? 0, "earlier leave span")} cut` : "";
  return `${plural(r.applied, "day")} on the rota from ${r.from} to ${r.to}${replaced}${leave}, ${absenceNote(r)}.`;
}

export interface ImportRotaDialogProps {
  open: boolean;
  onClose: () => void;
  /** The rota the roster shows; the file must be for it. */
  rotaCode: string;
  /** What the template is built from; without it a fixed example is offered. */
  catalogue?: ScheduleCatalogue;
}

/** Imports a whole rota for a period from one JSON file: choose it, preview what it would do, then write it, all or nothing. */
export default function ImportRotaDialog({ open, onClose, rotaCode, catalogue }: ImportRotaDialogProps): JSX.Element {
  const inputRef = useRef<HTMLInputElement>(null);
  const [fileName, setFileName] = useState<string | null>(null);
  const [parsed, setParsed] = useState<ParsedRotaImport | null>(null);
  const [preview, setPreview] = useState<ImportRotaResult | null>(null);
  const [written, setWritten] = useState<ImportRotaResult | null>(null);
  const importRota = useImportRota();
  // A real write cannot be called back, so the dialog stays open until it lands; a preview writes nothing.
  const writing = importRota.isPending && importRota.variables?.dryRun !== true;

  const reset = (): void => {
    setFileName(null);
    setParsed(null);
    setPreview(null);
    setWritten(null);
    importRota.reset();
    if (inputRef.current) inputRef.current.value = "";
  };
  const close = (): void => {
    if (writing) return;
    reset();
    onClose();
  };

  const onFile = async (e: ChangeEvent<HTMLInputElement>): Promise<void> => {
    const chosen = e.target.files?.[0];
    setPreview(null);
    setWritten(null);
    importRota.reset();
    if (!chosen) return;
    setFileName(chosen.name);
    if (chosen.size > MAX_FILE_BYTES) {
      setParsed({ error: "The file is larger than 1 MB." });
      return;
    }
    try {
      setParsed(parseRotaImportFile(await readText(chosen), rotaCode));
    } catch {
      setParsed({ error: "The file could not be read." });
    }
  };

  const file = parsed?.file;
  const runPreview = (): void => {
    if (!file) return;
    importRota.mutate({ rotaCode, file, dryRun: true }, { onSuccess: (res) => setPreview(res) });
  };
  const runImport = (): void => {
    if (!file) return;
    importRota.mutate({ rotaCode, file, dryRun: false }, { onSuccess: (res) => setWritten(res) });
  };

  const shown = written ?? preview;

  return (
    <Dialog
      open={open}
      onClose={close}
      disableEscapeKeyDown={writing}
      maxWidth="sm"
      fullWidth
      aria-labelledby="import-rota-title"
    >
      <DialogTitle id="import-rota-title">Import rota</DialogTitle>
      <DialogContent>
        <Typography variant="body2" sx={{ mb: 1.5 }}>
          Writes a whole {rotaCode} rota for a period from one JSON file: turns and time away, people by email. The
          file lands whole or not at all, through the same rules as editing the roster. Preview first to see what it
          would do.
        </Typography>
        <Typography variant="caption" color="text.secondary" component="p" sx={{ mb: 2 }}>
          Not sure of the format?{" "}
          <Link component="button" type="button" variant="caption" onClick={() => downloadTemplate(rotaCode, catalogue)}>
            Download a template
          </Link>
          .
        </Typography>

        <input
          ref={inputRef}
          type="file"
          accept="application/json,.json"
          hidden
          aria-label="Rota file"
          onChange={(e) => void onFile(e)}
        />
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
          <Button
            variant="outlined"
            size="small"
            onClick={() => inputRef.current?.click()}
            disabled={importRota.isPending || written !== null}
          >
            Choose file
          </Button>
          <Typography variant="body2" color={fileName ? "text.primary" : "text.secondary"} noWrap>
            {fileName ?? "No file chosen"}
          </Typography>
        </Box>

        {parsed?.error ? (
          <Alert severity="error" sx={{ mt: 2 }}>
            {parsed.error}
          </Alert>
        ) : null}

        {file && !shown ? (
          <Box sx={{ mt: 2 }}>
            <Typography variant="body2">
              {file.rotaCode}, {file.period.from} to {file.period.to}:{" "}
              {plural(file.assignments?.length ?? 0, "assignment")} and {plural(file.absences?.length ?? 0, "absence")}.
            </Typography>
            {file.mode === "replace" ? (
              <Alert severity="warning" sx={{ mt: 1 }}>
                Replace removes every assignment of the rota&rsquo;s teams from {file.period.from} to {file.period.to}{" "}
                before writing the file, and the time away of each person in the file on the days they are put on the
                rota. A move such as the Night cover, and the shifts it gives, are kept. Preview shows how many.
              </Alert>
            ) : (
              <Alert severity="info" sx={{ mt: 1 }}>
                Merge keeps what is on the roster and writes the file&rsquo;s entries over it; a clash with a row it
                does not replace is refused.
              </Alert>
            )}
          </Box>
        ) : null}

        {importRota.isError ? (
          <Alert severity="error" sx={{ mt: 2 }}>
            {importRota.error.message || "The rota could not be imported."}
          </Alert>
        ) : null}

        {shown ? (
          <Box sx={{ mt: 2 }} role="status">
            {written ? (
              <Alert severity="success">
                {written.rotaCode} imported: {summary(written)}
              </Alert>
            ) : (
              <>
                <Typography variant="body2">Preview: {summary(shown)}</Typography>
                <Typography variant="caption" color="text.secondary" component="p">
                  Nothing has been written yet.
                </Typography>
              </>
            )}
            {shown.skippedDays.length > 0 ? (
              <Alert severity="info" sx={{ mt: 1 }}>
                <Typography variant="body2" sx={{ fontWeight: 600 }}>
                  {plural(shown.skipped, "day")} skipped: the window is not worked then
                </Typography>
                <Box component="ul" sx={{ m: 0, pl: 2 }}>
                  {shown.skippedDays.slice(0, SHOWN_SKIPS).map((s) => (
                    <li key={s.index}>
                      assignments[{s.index}]: {s.dates.join(", ")}
                    </li>
                  ))}
                </Box>
                {shown.skippedDays.length > SHOWN_SKIPS ? (
                  <Typography variant="caption">and {shown.skippedDays.length - SHOWN_SKIPS} more</Typography>
                ) : null}
              </Alert>
            ) : null}
          </Box>
        ) : null}
      </DialogContent>
      <DialogActions>
        {writing ? (
          <Typography variant="body2" color="text.secondary" role="status" sx={{ mr: "auto", pl: 1 }}>
            Importing the rota&hellip;
          </Typography>
        ) : null}
        <Button onClick={close} disabled={writing}>
          {written ? "Done" : "Cancel"}
        </Button>
        {written ? null : preview ? (
          <Button variant="contained" onClick={runImport} disabled={importRota.isPending}>
            Import
          </Button>
        ) : (
          <Button variant="contained" onClick={runPreview} disabled={!file || importRota.isPending}>
            Preview
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
