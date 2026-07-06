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

import {
  Alert,
  alpha,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Dialog,
  Divider,
  FormControl,
  Grid,
  IconButton,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Skeleton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import { Download, FileText, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useCallback, useMemo, useState } from "react";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import DOMPurify from "dompurify";
import { useGetProductUpdateLevels } from "@features/updates/api/useGetProductUpdateLevels";
import { usePostUpdateLevelsSearch } from "@features/updates/api/usePostUpdateLevelsSearch";
import type {
  ProductUpdateLevel,
  SearchUpdatesPayload,
  UpdateDescription,
} from "@features/updates/types/updates";
import {
  buildReportData,
  parseJsonStringArray,
  type ReportData,
  type ReportRow,
} from "@features/updates/utils/updateReport";

interface FilterState {
  productName: string;
  productVersion: string;
  startLevel: string;
  endLevel: string;
}

const INITIAL_FILTER: FilterState = {
  productName: "",
  productVersion: "",
  startLevel: "",
  endLevel: "",
};

// Real HTML tags that warrant sanitized HTML rendering vs plain-text fallback.
const HTML_FORMAT_RE =
  /<\/?(p|span|div|ul|ol|li|strong|em|b|i|br|h[1-6]|a[\s>]|table|tr|td|th|code|pre|blockquote)\b/i;

const HTML_CONTENT_SX = {
  fontSize: "0.875rem",
  lineHeight: 1.7,
  color: "text.secondary",
  "& p": { margin: "0 0 0.4em 0" },
  "& p:last-child": { marginBottom: 0 },
  "& a": { color: "primary.main", textDecoration: "underline" },
  "& ul, & ol": { mt: 0, mb: 0.5, pl: 2.5 },
  "& li": { mb: 0.25 },
  "& strong, & b": { fontWeight: 600, color: "text.primary" },
};

function HtmlOrText({ content }: { content: string }): JSX.Element {
  if (HTML_FORMAT_RE.test(content)) {
    return (
      <Box
        dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(content) }}
        sx={HTML_CONTENT_SX}
      />
    );
  }
  // Plain-text branch: render as text so React handles escaping. Avoids the
  // decode/re-encode round-trip and the need for dangerouslySetInnerHTML.
  return (
    <Box
      component="pre"
      sx={{
        ...HTML_CONTENT_SX,
        whiteSpace: "pre-wrap",
        wordBreak: "break-word",
        m: 0,
        fontFamily: "inherit",
      }}
    >
      {content}
    </Box>
  );
}

function isMeaningfulContent(value: string | undefined | null): boolean {
  const t = (value ?? "").trim().toLowerCase();
  return t !== "" && t !== "n/a" && t !== "na";
}

function getProductNames(data: ProductUpdateLevel[] | undefined): string[] {
  if (!data) return [];
  return Array.from(new Set(data.map((p) => p.productName))).sort();
}

function getVersionsForProduct(
  data: ProductUpdateLevel[] | undefined,
  productName: string,
): string[] {
  if (!data || !productName) return [];
  const product = data.find((p) => p.productName === productName);
  if (!product) return [];
  return Array.from(
    new Set(product.productUpdateLevels.map((v) => v.productBaseVersion)),
  ).sort();
}

function getLevelsForVersion(
  data: ProductUpdateLevel[] | undefined,
  productName: string,
  productVersion: string,
): number[] {
  if (!data || !productName || !productVersion) return [];
  const product = data.find((p) => p.productName === productName);
  if (!product) return [];
  const version = product.productUpdateLevels.find(
    (v) => v.productBaseVersion === productVersion,
  );
  return version ? [...version.updateLevels].sort((a, b) => a - b) : [];
}

function getUpdateTypeChipColor(
  updateType: string,
): "error" | "warning" | "primary" {
  const t = updateType.toLowerCase();
  if (t === "security") return "error";
  if (t === "regular") return "warning";
  return "primary";
}

function UpdateSection({
  title,
  content,
}: {
  title: string;
  content: string;
}): JSX.Element {
  return (
    <Box>
      <Typography variant="subtitle2" fontWeight={700} sx={{ mb: 0.75 }}>
        {title}
      </Typography>
      <HtmlOrText content={content} />
    </Box>
  );
}

function FileList({
  title,
  items,
}: {
  title: string;
  items: string[];
}): JSX.Element | null {
  if (items.length === 0) return null;
  return (
    <Box>
      <Typography variant="subtitle2" fontWeight={700} sx={{ mb: 0.75 }}>
        {title}
      </Typography>
      <Box component="ul" sx={{ pl: 2.5, m: 0 }}>
        {items.map((f) => (
          <Typography key={f} component="li" variant="body2" color="text.secondary">
            {f}
          </Typography>
        ))}
      </Box>
    </Box>
  );
}

interface UpdateDetailsDialogProps {
  open: boolean;
  onClose: () => void;
  levelKey: string;
  updateType: string;
  descriptions: UpdateDescription[];
}

function UpdateDetailsDialog({
  open,
  onClose,
  levelKey,
  updateType,
  descriptions,
}: UpdateDetailsDialogProps): JSX.Element {
  const theme = useTheme();
  const chipColor = getUpdateTypeChipColor(updateType);

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="md">
      <Box sx={{ p: 3, display: "flex", flexDirection: "column", gap: 2 }}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 2,
          }}
        >
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
            <Typography variant="h6">Update Level {levelKey}</Typography>
            <Chip
              label={updateType.charAt(0).toUpperCase() + updateType.slice(1)}
              size="small"
              sx={{
                height: 22,
                fontSize: "0.72rem",
                fontWeight: 600,
                bgcolor: alpha(theme.palette[chipColor].main, 0.15),
                color: theme.palette[chipColor].dark,
                border: `1px solid ${alpha(theme.palette[chipColor].main, 0.35)}`,
              }}
            />
          </Box>
          <IconButton size="small" onClick={onClose} aria-label="Close">
            <X size={18} />
          </IconButton>
        </Box>

        <Divider />

        <Stack spacing={3}>
          {descriptions.map((desc) => {
            const bugFixes = parseJsonStringArray(desc.bugFixes);
            const filesAdded = parseJsonStringArray(desc.filesAdded);
            const filesModified = parseJsonStringArray(desc.filesModified);
            const filesRemoved = parseJsonStringArray(desc.filesRemoved);

            return (
              <Box key={desc.updateNumber}>
                <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 1.5 }}>
                  Update Number: {desc.updateNumber}
                </Typography>
                <Stack spacing={2}>
                  {desc.description && (
                    <UpdateSection title="Description" content={desc.description} />
                  )}
                  {isMeaningfulContent(desc.instructions) && (
                    <UpdateSection
                      title="Instructions"
                      content={desc.instructions as string}
                    />
                  )}
                  {bugFixes.length > 0 && (
                    <FileList title="Bug fixes" items={bugFixes} />
                  )}
                  <FileList title="Files added" items={filesAdded} />
                  <FileList title="Files modified" items={filesModified} />
                  <FileList title="Files removed" items={filesRemoved} />
                  {desc.securityAdvisories?.length > 0 && (
                    <Box>
                      <Typography variant="subtitle2" fontWeight={700} sx={{ mb: 0.75 }}>
                        Security advisories
                      </Typography>
                      <Stack spacing={1.5}>
                        {desc.securityAdvisories.map((a) => (
                          <Paper key={a.id} variant="outlined" sx={{ p: 1.5 }}>
                            <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 0.5 }}>
                              <Typography variant="body2" fontWeight={600}>
                                {a.id}
                              </Typography>
                              <Chip size="small" label={a.severity} color="error" variant="outlined" />
                            </Stack>
                            {a.overview && <HtmlOrText content={a.overview} />}
                          </Paper>
                        ))}
                      </Stack>
                    </Box>
                  )}
                </Stack>
              </Box>
            );
          })}
        </Stack>
      </Box>
    </Dialog>
  );
}

function ReportSection({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}): JSX.Element {
  return (
    <Box>
      <Typography variant="subtitle2" fontWeight={700} sx={{ mb: 0.5 }}>
        {title}
      </Typography>
      {children}
    </Box>
  );
}

function ReportRowBlock({ row }: { row: ReportRow }): JSX.Element {
  const theme = useTheme();
  const chipColor = getUpdateTypeChipColor(row.updateType);
  return (
    <Box>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
        <Typography variant="subtitle1" fontWeight={600}>
          Update {row.desc.updateNumber}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          · Level {row.levelKey}
        </Typography>
        <Chip
          label={row.updateType.charAt(0).toUpperCase() + row.updateType.slice(1)}
          size="small"
          sx={{
            height: 22,
            fontSize: "0.72rem",
            fontWeight: 600,
            bgcolor: alpha(theme.palette[chipColor].main, 0.15),
            color: theme.palette[chipColor].dark,
            border: `1px solid ${alpha(theme.palette[chipColor].main, 0.35)}`,
          }}
        />
        <Box sx={{ flex: 1 }} />
        <Typography variant="caption" color="text.secondary">
          {row.released}
        </Typography>
      </Stack>
      <Stack spacing={1.5}>
        {row.description && (
          <ReportSection title="Description">
            <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: "pre-wrap" }}>
              {row.description}
            </Typography>
          </ReportSection>
        )}
        {row.instructions && (
          <ReportSection title="Instructions">
            <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: "pre-wrap" }}>
              {row.instructions}
            </Typography>
          </ReportSection>
        )}
        {row.bugFixes.length > 0 && (
          <ReportSection title="Bug fixes">
            <Box component="ul" sx={{ pl: 2.5, m: 0 }}>
              {row.bugFixes.map((b) => (
                <Typography key={b} component="li" variant="body2" color="text.secondary">
                  {b}
                </Typography>
              ))}
            </Box>
          </ReportSection>
        )}
        {(row.filesAdded.length > 0 || row.filesModified.length > 0 || row.filesRemoved.length > 0) && (
          <ReportSection title="Files">
            {row.filesAdded.length > 0 && (
              <Typography variant="caption" color="text.secondary">
                Added: {row.filesAdded.length}
              </Typography>
            )}
            {row.filesModified.length > 0 && (
              <Typography variant="caption" color="text.secondary" sx={{ ml: row.filesAdded.length > 0 ? 2 : 0 }}>
                Modified: {row.filesModified.length}
              </Typography>
            )}
            {row.filesRemoved.length > 0 && (
              <Typography variant="caption" color="text.secondary" sx={{ ml: 2 }}>
                Removed: {row.filesRemoved.length}
              </Typography>
            )}
          </ReportSection>
        )}
        {row.securityAdvisories.length > 0 && (
          <ReportSection title="Security advisories">
            <Stack spacing={1}>
              {row.securityAdvisories.map((a) => (
                <Paper key={a.id} variant="outlined" sx={{ p: 1 }}>
                  <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 0.5 }}>
                    <Typography variant="body2" fontWeight={600}>{a.id}</Typography>
                    <Chip size="small" label={a.severity} color="error" variant="outlined" />
                  </Stack>
                  {a.overview && <HtmlOrText content={a.overview} />}
                </Paper>
              ))}
            </Stack>
          </ReportSection>
        )}
      </Stack>
    </Box>
  );
}

interface ReportPreviewDialogProps {
  open: boolean;
  onClose: () => void;
  report: ReportData;
  onDownload: () => void;
}

function ReportPreviewDialog({
  open,
  onClose,
  report,
  onDownload,
}: ReportPreviewDialogProps): JSX.Element {
  const { params, rows, counts } = report;
  const levelCount = useMemo(
    () => new Set(rows.map((r) => r.levelKey)).size,
    [rows],
  );
  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="lg">
      <Box sx={{ p: 3, display: "flex", flexDirection: "column", gap: 2 }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" spacing={2}>
          <Box>
            <Typography variant="h6">Update Levels Report</Typography>
            <Typography variant="body2" color="text.secondary">
              {params.productName} {params.productVersion} · Levels {params.startLevel} → {params.endLevel}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {levelCount} level(s) · {counts.security} security · {counts.regular} regular
              {counts.mixed > 0 ? ` · ${counts.mixed} mixed` : ""}
            </Typography>
          </Box>
          <Stack direction="row" spacing={1}>
            <Button
              variant="outlined"
              size="small"
              startIcon={<Download size={14} />}
              onClick={onDownload}
            >
              Download PDF
            </Button>
            <IconButton size="small" onClick={onClose} aria-label="Close">
              <X size={18} />
            </IconButton>
          </Stack>
        </Stack>
        <Divider />
        <Stack spacing={3}>
          {rows.length === 0 ? (
            <Typography variant="body2" color="text.secondary" align="center" sx={{ py: 4 }}>
              No updates to report.
            </Typography>
          ) : (
            rows.map((row, i) => (
              <Box key={`${row.levelKey}-${row.desc.updateNumber}`}>
                <ReportRowBlock row={row} />
                {i < rows.length - 1 && <Divider sx={{ mt: 2 }} />}
              </Box>
            ))
          )}
        </Stack>
      </Box>
    </Dialog>
  );
}

/**
 * Scaffolding for the four filter dropdowns + action buttons, shown while the
 * product/version/level catalogue loads — so the initial screen presents the
 * shape of the controls instead of empty disabled selects. Mirrors the customer
 * portal's "Search Update Levels" loading state.
 */
function UpdatesFilterSkeleton(): JSX.Element {
  return (
    <>
      <Grid container spacing={2}>
        {[1, 2, 3, 4].map((i) => (
          <Grid key={i} size={{ xs: 12, sm: 6, md: 3 }}>
            <Skeleton variant="rounded" height={40} />
          </Grid>
        ))}
      </Grid>
      <Stack direction="row" spacing={1} sx={{ mt: 2 }} flexWrap="wrap">
        <Skeleton variant="rounded" width={96} height={36} />
        <Skeleton variant="rounded" width={72} height={36} />
        <Skeleton variant="rounded" width={150} height={36} />
        <Skeleton variant="rounded" width={150} height={36} />
      </Stack>
    </>
  );
}

export default function CsmUpdatesPage(): JSX.Element {
  const theme = useTheme();

  const [filter, setFilter] = useState<FilterState>(INITIAL_FILTER);
  const [search, setSearch] = useState<SearchUpdatesPayload | null>(null);
  const [openLevel, setOpenLevel] = useState<string | null>(null);
  const [previewOpen, setPreviewOpen] = useState(false);

  const productLevels = useGetProductUpdateLevels();
  const searchResult = usePostUpdateLevelsSearch(search);

  const productNames = useMemo(
    () => getProductNames(productLevels.data),
    [productLevels.data],
  );
  const versionOptions = useMemo(
    () => getVersionsForProduct(productLevels.data, filter.productName),
    [productLevels.data, filter.productName],
  );
  const rawLevels = useMemo(
    () => getLevelsForVersion(productLevels.data, filter.productName, filter.productVersion),
    [productLevels.data, filter.productName, filter.productVersion],
  );

  // Start level dropdown always includes 0 (meaning "from no updates installed").
  const startLevelOptions = useMemo(() => {
    if (rawLevels.length === 0) return [];
    return rawLevels[0] === 0 ? rawLevels : [0, ...rawLevels];
  }, [rawLevels]);

  const endLevelOptions = useMemo(() => {
    if (!filter.startLevel || rawLevels.length === 0) return [];
    const start = Number(filter.startLevel);
    return rawLevels.filter((l) => l > start);
  }, [rawLevels, filter.startLevel]);

  const handleFilterChange =
    (field: keyof FilterState) => (e: SelectChangeEvent<string>) => {
      const value = String(e.target.value);
      setFilter((prev) => {
        const next = { ...prev, [field]: value };
        if (field === "productName") {
          next.productVersion = "";
          next.startLevel = "";
          next.endLevel = "";
        } else if (field === "productVersion") {
          next.startLevel = "";
          next.endLevel = "";
        } else if (field === "startLevel") {
          next.endLevel = "";
        }
        return next;
      });
    };

  const canSearch =
    filter.productName !== "" &&
    filter.productVersion !== "" &&
    filter.startLevel !== "" &&
    filter.endLevel !== "" &&
    Number(filter.endLevel) > Number(filter.startLevel);

  const handleSearch = useCallback(() => {
    if (!canSearch) return;
    setSearch({
      productName: filter.productName,
      productVersion: filter.productVersion,
      startingUpdateLevel: Number(filter.startLevel),
      endingUpdateLevel: Number(filter.endLevel),
    });
  }, [canSearch, filter]);

  const handleClear = useCallback(() => {
    setFilter(INITIAL_FILTER);
    setSearch(null);
  }, []);

  const sortedEntries = useMemo(() => {
    if (!searchResult.data) return [];
    return Object.entries(searchResult.data).sort(
      ([a], [b]) => Number(a) - Number(b),
    );
  }, [searchResult.data]);

  const counts = useMemo(() => {
    const c = { security: 0, regular: 0, mixed: 0 };
    for (const [, entry] of sortedEntries) {
      const t = entry.updateType.toLowerCase();
      if (t === "security") c.security += 1;
      else if (t === "regular") c.regular += 1;
      else if (t === "mixed") c.mixed += 1;
    }
    return c;
  }, [sortedEntries]);

  const openEntry = useMemo(() => {
    if (!openLevel || !searchResult.data) return null;
    const entry = searchResult.data[openLevel];
    return entry ? { key: openLevel, entry } : null;
  }, [openLevel, searchResult.data]);

  const reportData = useMemo<ReportData | null>(() => {
    if (!search || !searchResult.data) return null;
    return buildReportData(
      {
        productName: search.productName,
        productVersion: search.productVersion,
        startLevel: search.startingUpdateLevel,
        endLevel: search.endingUpdateLevel,
      },
      searchResult.data,
    );
  }, [search, searchResult.data]);

  // Preview/Download buttons should only act on results matching the *current*
  // filter selections. If the user changes any dropdown after Search, the
  // result data is stale relative to what's shown in the inputs — block the
  // report actions until they hit Search again.
  const filterMatchesSearch = useMemo(() => {
    if (!search) return false;
    return (
      filter.productName === search.productName &&
      filter.productVersion === search.productVersion &&
      Number(filter.startLevel) === search.startingUpdateLevel &&
      Number(filter.endLevel) === search.endingUpdateLevel
    );
  }, [filter, search]);

  const handleDownloadPdf = useCallback(async () => {
    if (!reportData || !filterMatchesSearch) return;
    const { generateUpdateReportPdf } = await import(
      "@features/updates/utils/updateReportPdf"
    );
    generateUpdateReportPdf(reportData);
  }, [reportData, filterMatchesSearch]);

  const hasResults = sortedEntries.length > 0;
  const reportActionsEnabled = hasResults && !!reportData && filterMatchesSearch;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box>
        <Typography variant="h5">Updates</Typography>
        <Typography variant="body2" color="text.secondary">
          Check release notes between two update levels of a WSO2 product version.
        </Typography>
      </Box>

      {/* Filter */}
      <Card variant="outlined">
        <CardContent>
          <Typography variant="subtitle1" sx={{ mb: 2 }}>
            Search updates between levels
          </Typography>
          {productLevels.isError && (
            <Alert severity="error" sx={{ mb: 2 }}>
              Could not load product catalog.
            </Alert>
          )}
          {productLevels.isLoading ? (
            <UpdatesFilterSkeleton />
          ) : (
          <>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControl fullWidth size="small" disabled={productLevels.isLoading}>
                <InputLabel id="upd-product">Product</InputLabel>
                <Select
                  labelId="upd-product"
                  value={filter.productName}
                  label="Product"
                  onChange={handleFilterChange("productName")}
                >
                  {productNames.map((n) => (
                    <MenuItem key={n} value={n}>{n}</MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControl fullWidth size="small" disabled={!filter.productName}>
                <InputLabel id="upd-version">Version</InputLabel>
                <Select
                  labelId="upd-version"
                  value={filter.productVersion}
                  label="Version"
                  onChange={handleFilterChange("productVersion")}
                >
                  {versionOptions.map((v) => (
                    <MenuItem key={v} value={v}>{v}</MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControl fullWidth size="small" disabled={!filter.productVersion}>
                <InputLabel id="upd-start">Start level</InputLabel>
                <Select
                  labelId="upd-start"
                  value={filter.startLevel}
                  label="Start level"
                  onChange={handleFilterChange("startLevel")}
                >
                  {startLevelOptions.map((l) => (
                    <MenuItem key={l} value={String(l)}>{l}</MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControl fullWidth size="small" disabled={!filter.startLevel}>
                <InputLabel id="upd-end">End level</InputLabel>
                <Select
                  labelId="upd-end"
                  value={filter.endLevel}
                  label="End level"
                  onChange={handleFilterChange("endLevel")}
                >
                  {endLevelOptions.map((l) => (
                    <MenuItem key={l} value={String(l)}>{l}</MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
          </Grid>
          <Stack direction="row" spacing={1} sx={{ mt: 2 }} flexWrap="wrap">
            <Button variant="contained" onClick={handleSearch} disabled={!canSearch}>
              Search
            </Button>
            <Button variant="text" onClick={handleClear}>Clear</Button>
            <Button
              variant="outlined"
              startIcon={<FileText size={16} />}
              onClick={() => setPreviewOpen(true)}
              disabled={!reportActionsEnabled}
            >
              Preview report
            </Button>
            <Button
              variant="outlined"
              color="warning"
              startIcon={<Download size={16} />}
              onClick={handleDownloadPdf}
              disabled={!reportActionsEnabled}
            >
              Download PDF
            </Button>
          </Stack>
          </>
          )}
        </CardContent>
      </Card>

      {/* Results */}
      {search && (
        <Box>
          {searchResult.isLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
              <CircularProgress />
            </Box>
          ) : searchResult.isError ? (
            <Alert severity="error">
              Could not load updates: {searchResult.error.message}
            </Alert>
          ) : sortedEntries.length === 0 ? (
            <Typography variant="body2" color="text.secondary" align="center" sx={{ py: 4 }}>
              No updates found between level {search.startingUpdateLevel} and {search.endingUpdateLevel}.
            </Typography>
          ) : (
            <Stack spacing={2}>
              <Typography variant="body2" color="text.secondary">
                There are <strong>{sortedEntries.length}</strong> updates with{" "}
                <strong>{counts.security}</strong> security,{" "}
                <strong>{counts.regular}</strong> regular
                {counts.mixed > 0 && (
                  <>
                    , and <strong>{counts.mixed}</strong> mixed
                  </>
                )}{" "}
                updates.
              </Typography>

              <TableContainer component={Paper}>
                <Table sx={{ minWidth: 650, width: "100%", tableLayout: "fixed" }}>
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ fontWeight: 600, textTransform: "uppercase", fontSize: "0.75rem", width: "33.33%" }}>
                        Update Level
                      </TableCell>
                      <TableCell align="center" sx={{ fontWeight: 600, textTransform: "uppercase", fontSize: "0.75rem", width: "33.33%" }}>
                        Update Type
                      </TableCell>
                      <TableCell align="right" sx={{ fontWeight: 600, textTransform: "uppercase", fontSize: "0.75rem", width: "33.33%" }}>
                        Details
                      </TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {sortedEntries.map(([levelKey, entry]) => {
                      const chipColor = getUpdateTypeChipColor(entry.updateType);
                      return (
                        <TableRow key={levelKey} hover sx={{ "&:last-child td, &:last-child th": { border: 0 } }}>
                          <TableCell>
                            <Typography variant="body2">{levelKey}</Typography>
                          </TableCell>
                          <TableCell align="center">
                            <Box sx={{ display: "flex", justifyContent: "center" }}>
                              <Chip
                                label={entry.updateType.charAt(0).toUpperCase() + entry.updateType.slice(1)}
                                size="small"
                                sx={{
                                  height: 22,
                                  fontSize: "0.72rem",
                                  fontWeight: 600,
                                  bgcolor: alpha(theme.palette[chipColor].main, 0.15),
                                  color: theme.palette[chipColor].dark,
                                  border: `1px solid ${alpha(theme.palette[chipColor].main, 0.35)}`,
                                }}
                              />
                            </Box>
                          </TableCell>
                          <TableCell align="right">
                            <Button
                              variant="text"
                              size="small"
                              onClick={() => setOpenLevel(levelKey)}
                              sx={{
                                color: "warning.main",
                                fontWeight: 500,
                                p: 0,
                                minWidth: 0,
                                textTransform: "none",
                                "&:hover": {
                                  bgcolor: "transparent",
                                  textDecoration: "underline",
                                  color: "warning.dark",
                                },
                              }}
                            >
                              View
                            </Button>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </TableContainer>
            </Stack>
          )}
        </Box>
      )}

      {openEntry && (
        <UpdateDetailsDialog
          open
          onClose={() => setOpenLevel(null)}
          levelKey={openEntry.key}
          updateType={openEntry.entry.updateType}
          descriptions={openEntry.entry.updateDescriptionLevels}
        />
      )}

      {reportData && (
        <ReportPreviewDialog
          open={previewOpen}
          onClose={() => setPreviewOpen(false)}
          report={reportData}
          onDownload={handleDownloadPdf}
        />
      )}
    </Box>
  );
}
