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

import { useMemo, useState, type ReactNode } from "react";
import {
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  Dialog,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Skeleton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { useQuery } from "@tanstack/react-query";
import DOMPurify from "dompurify";
import { DialogPaper } from "@components/common/DialogPaper";
import { updates } from "@src/services/updates";
import type { ProductUpdateLevel, SearchUpdatesInput, UpdateDescription, UpdateLevelGroup } from "@src/types";

// The Acrylic theme renders popup papers translucent, so a dropdown reads as
// see-through unless forced opaque — same fix as TimeCardFiltersSheet /
// AnnouncementFiltersSheet / LogTimeCardDialog.
const OPAQUE_POPUP = { sx: { backgroundColor: "background.default", backgroundImage: "none" } };

interface FilterState {
  productName: string;
  productVersion: string;
  startLevel: string;
  endLevel: string;
}

const INITIAL_FILTER: FilterState = { productName: "", productVersion: "", startLevel: "", endLevel: "" };

// Real HTML tags that warrant sanitized HTML rendering vs a plain-text fallback
// — update descriptions come back as HTML sometimes and plain text other
// times, from the same upstream field (see the webapp's identical gotcha).
const HTML_FORMAT_RE = /<\/?(p|span|div|ul|ol|li|strong|em|b|i|br|h[1-6]|a[\s>]|table|tr|td|th|code|pre|blockquote)\b/i;

function HtmlOrText({ content }: { content: string }) {
  if (HTML_FORMAT_RE.test(content)) {
    return (
      <Box
        dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(content) }}
        sx={{ fontSize: "0.875rem", lineHeight: 1.6, color: "text.secondary", "& p": { m: "0 0 0.4em 0" } }}
      />
    );
  }
  return (
    <Box
      component="pre"
      sx={{
        fontSize: "0.875rem",
        lineHeight: 1.6,
        color: "text.secondary",
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

function isMeaningful(value: string | undefined): boolean {
  const t = (value ?? "").trim().toLowerCase();
  return t !== "" && t !== "n/a" && t !== "na";
}

function updateTypeChipColor(updateType: string): "error" | "warning" | "primary" {
  const t = updateType.toLowerCase();
  if (t === "security") return "error";
  if (t === "regular") return "warning";
  return "primary";
}

function getProductNames(data: ProductUpdateLevel[] | undefined): string[] {
  if (!data) return [];
  return Array.from(new Set(data.map((p) => p.productName))).sort();
}

function getVersionsForProduct(data: ProductUpdateLevel[] | undefined, productName: string): string[] {
  const product = data?.find((p) => p.productName === productName);
  if (!product) return [];
  return Array.from(new Set(product.productUpdateLevels.map((v) => v.productBaseVersion))).sort();
}

function getLevelsForVersion(
  data: ProductUpdateLevel[] | undefined,
  productName: string,
  productVersion: string,
): number[] {
  const product = data?.find((p) => p.productName === productName);
  const version = product?.productUpdateLevels.find((v) => v.productBaseVersion === productVersion);
  return version ? [...version.updateLevels].sort((a, b) => a - b) : [];
}

export default function UpdatesPage() {
  const [filter, setFilter] = useState<FilterState>(INITIAL_FILTER);
  const [search, setSearch] = useState<SearchUpdatesInput | null>(null);
  const [openGroup, setOpenGroup] = useState<UpdateLevelGroup | null>(null);

  const productLevels = useQuery(updates.productLevels());
  const searchResult = useQuery(updates.search(search));

  const productNames = useMemo(() => getProductNames(productLevels.data), [productLevels.data]);
  const versionOptions = useMemo(
    () => getVersionsForProduct(productLevels.data, filter.productName),
    [productLevels.data, filter.productName],
  );
  const rawLevels = useMemo(
    () => getLevelsForVersion(productLevels.data, filter.productName, filter.productVersion),
    [productLevels.data, filter.productName, filter.productVersion],
  );
  const startLevelOptions = useMemo(
    () => (rawLevels.length === 0 ? [] : rawLevels[0] === 0 ? rawLevels : [0, ...rawLevels]),
    [rawLevels],
  );
  const endLevelOptions = useMemo(() => {
    if (!filter.startLevel || rawLevels.length === 0) return [];
    const start = Number(filter.startLevel);
    return rawLevels.filter((l) => l > start);
  }, [rawLevels, filter.startLevel]);

  const setField = (field: keyof FilterState, value: string) => {
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

  const handleSearch = () => {
    if (!canSearch) return;
    setSearch({
      productName: filter.productName,
      productVersion: filter.productVersion,
      startingUpdateLevel: Number(filter.startLevel),
      endingUpdateLevel: Number(filter.endLevel),
    });
  };

  const handleClear = () => {
    setFilter(INITIAL_FILTER);
    setSearch(null);
  };

  return (
    <Stack gap={2}>
      <Box>
        <Typography variant="h6">Updates</Typography>
        <Typography variant="body2" color="text.secondary">
          Check release notes between two update levels of a product version.
        </Typography>
      </Box>

      <Card sx={{ p: 2 }}>
        {productLevels.isError ? (
          <Typography variant="body2" color="error.main">
            Could not load the product catalog. Please try again.
          </Typography>
        ) : productLevels.isLoading ? (
          <Stack gap={1.5}>
            {[1, 2, 3, 4].map((i) => (
              <Skeleton key={i} variant="rounded" height={40} />
            ))}
          </Stack>
        ) : (
          <Stack gap={1.5}>
            <FormControl size="small" fullWidth>
              <InputLabel id="upd-product">Product</InputLabel>
              <Select
                labelId="upd-product"
                label="Product"
                value={filter.productName}
                onChange={(e) => setField("productName", String(e.target.value))}
                MenuProps={{ slotProps: { paper: OPAQUE_POPUP } }}
              >
                {productNames.map((n) => (
                  <MenuItem key={n} value={n}>
                    {n}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <FormControl size="small" fullWidth disabled={!filter.productName}>
              <InputLabel id="upd-version">Version</InputLabel>
              <Select
                labelId="upd-version"
                label="Version"
                value={filter.productVersion}
                onChange={(e) => setField("productVersion", String(e.target.value))}
                MenuProps={{ slotProps: { paper: OPAQUE_POPUP } }}
              >
                {versionOptions.map((v) => (
                  <MenuItem key={v} value={v}>
                    {v}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <Stack direction="row" gap={1.5}>
              <FormControl size="small" fullWidth disabled={!filter.productVersion}>
                <InputLabel id="upd-start">Start level</InputLabel>
                <Select
                  labelId="upd-start"
                  label="Start level"
                  value={filter.startLevel}
                  onChange={(e) => setField("startLevel", String(e.target.value))}
                  MenuProps={{ slotProps: { paper: OPAQUE_POPUP } }}
                >
                  {startLevelOptions.map((l) => (
                    <MenuItem key={l} value={String(l)}>
                      {l}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>

              <FormControl size="small" fullWidth disabled={!filter.startLevel}>
                <InputLabel id="upd-end">End level</InputLabel>
                <Select
                  labelId="upd-end"
                  label="End level"
                  value={filter.endLevel}
                  onChange={(e) => setField("endLevel", String(e.target.value))}
                  MenuProps={{ slotProps: { paper: OPAQUE_POPUP } }}
                >
                  {endLevelOptions.map((l) => (
                    <MenuItem key={l} value={String(l)}>
                      {l}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Stack>

            <Stack direction="row" gap={1}>
              <Button variant="contained" size="small" onClick={handleSearch} disabled={!canSearch}>
                Search
              </Button>
              <Button variant="text" size="small" onClick={handleClear}>
                Clear
              </Button>
            </Stack>
          </Stack>
        )}
      </Card>

      {search && (
        <Box>
          {searchResult.isLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
              <CircularProgress size={28} />
            </Box>
          ) : searchResult.isError ? (
            <Typography variant="body2" color="error.main" textAlign="center" sx={{ py: 2 }}>
              Could not load updates. Please try again.
            </Typography>
          ) : !searchResult.data || searchResult.data.length === 0 ? (
            <Typography variant="body2" color="text.secondary" textAlign="center" sx={{ py: 2 }}>
              No updates found between level {search.startingUpdateLevel} and {search.endingUpdateLevel}.
            </Typography>
          ) : (
            <Stack gap={1}>
              <Typography variant="body2" color="text.secondary">
                {searchResult.data.length} update level(s) found.
              </Typography>
              {searchResult.data.map((group) => (
                <Stack
                  key={group.levelKey}
                  direction="row"
                  alignItems="center"
                  justifyContent="space-between"
                  gap={1}
                  sx={{ p: 1.5, border: "1px solid", borderColor: "divider", borderRadius: 1 }}
                >
                  <Stack direction="row" alignItems="center" gap={1}>
                    <Typography variant="body2" fontWeight={500}>
                      Level {group.levelKey}
                    </Typography>
                    <Chip
                      size="small"
                      label={group.updateType.charAt(0).toUpperCase() + group.updateType.slice(1)}
                      color={updateTypeChipColor(group.updateType)}
                    />
                  </Stack>
                  <Button size="small" variant="text" onClick={() => setOpenGroup(group)}>
                    View
                  </Button>
                </Stack>
              ))}
            </Stack>
          )}
        </Box>
      )}

      {openGroup && <UpdateLevelDialog group={openGroup} onClose={() => setOpenGroup(null)} />}
    </Stack>
  );
}

function UpdateLevelDialog({ group, onClose }: { group: UpdateLevelGroup; onClose: () => void }) {
  return (
    <DialogPaperWrapper onClose={onClose}>
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography variant="h6" fontWeight={650}>
          Update level {group.levelKey}
        </Typography>
        <Chip
          size="small"
          label={group.updateType.charAt(0).toUpperCase() + group.updateType.slice(1)}
          color={updateTypeChipColor(group.updateType)}
        />
      </Stack>

      <Stack gap={2.5}>
        {group.updateDescriptionLevels.map((desc) => (
          <UpdateDescriptionBlock key={desc.updateNumber} desc={desc} />
        ))}
      </Stack>

      <Button variant="outlined" onClick={onClose} sx={{ alignSelf: "end" }}>
        Close
      </Button>
    </DialogPaperWrapper>
  );
}

function UpdateDescriptionBlock({ desc }: { desc: UpdateDescription }) {
  return (
    <Stack gap={1.5}>
      <Typography variant="subtitle2" fontWeight={600}>
        Update number: {desc.updateNumber}
      </Typography>

      {desc.description && <UpdateSection title="Description" content={desc.description} />}
      {isMeaningful(desc.instructions) && <UpdateSection title="Instructions" content={desc.instructions!} />}
      <FileList title="Bug fixes" items={desc.bugFixes} />
      <FileList title="Files added" items={desc.filesAdded} />
      <FileList title="Files modified" items={desc.filesModified} />
      <FileList title="Files removed" items={desc.filesRemoved} />

      {desc.securityAdvisories.length > 0 && (
        <Box>
          <Typography variant="subtitle2" fontWeight={600} sx={{ mb: 0.75 }}>
            Security advisories
          </Typography>
          <Stack gap={1}>
            {desc.securityAdvisories.map((a) => (
              <Box key={a.id} sx={{ p: 1.25, border: "1px solid", borderColor: "divider", borderRadius: 1 }}>
                <Stack direction="row" gap={1} alignItems="center" sx={{ mb: 0.5 }}>
                  <Typography variant="body2" fontWeight={600}>
                    {a.id}
                  </Typography>
                  <Chip size="small" label={a.severity} color="error" variant="outlined" />
                </Stack>
                {a.overview && <HtmlOrText content={a.overview} />}
              </Box>
            ))}
          </Stack>
        </Box>
      )}
    </Stack>
  );
}

function UpdateSection({ title, content }: { title: string; content: string }) {
  return (
    <Box>
      <Typography variant="subtitle2" fontWeight={600} sx={{ mb: 0.75 }}>
        {title}
      </Typography>
      <HtmlOrText content={content} />
    </Box>
  );
}

function FileList({ title, items }: { title: string; items: string[] }) {
  if (items.length === 0) return null;
  return (
    <Box>
      <Typography variant="subtitle2" fontWeight={600} sx={{ mb: 0.75 }}>
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

// Thin wrapper so UpdateLevelDialog's JSX doesn't need to repeat the
// Dialog/slots/slotProps boilerplate — mirrors the microapp's other
// full-content dialogs (Card component={Stack} via the shared DialogPaper).
function DialogPaperWrapper({ children, onClose }: { children: ReactNode; onClose: () => void }) {
  return (
    <Dialog
      open
      onClose={onClose}
      slots={{ paper: DialogPaper }}
      slotProps={{
        paper: { sx: { bgcolor: "background.default", p: 2, gap: 2, m: 2, maxHeight: "85vh", overflowY: "auto" } },
      }}
    >
      {children}
    </Dialog>
  );
}
