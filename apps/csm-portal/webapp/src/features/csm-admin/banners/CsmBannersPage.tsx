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
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Checkbox,
  FormControlLabel,
  IconButton,
  MenuItem,
  Paper,
  Stack,
  Switch,
  Tab,
  TablePagination,
  Tabs,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown, Pencil, Plus } from "@wso2/oxygen-ui-icons-react";
import { darken, lighten, useTheme, type Theme } from "@mui/material/styles";
import { useEffect, useMemo, useState, type ChangeEvent, type JSX, type KeyboardEvent } from "react";
import {
  BANNER_TYPE_LABEL,
  EMPTY_BANNER_CONTENT,
  buildBannerHtml,
  colorsForBannerType,
  cssColorToHex,
  messageKeepingTitle,
  parseBannerHtml,
  TEXT_BANNER_TYPES,
  type AlertTone,
  type BannerContent,
  type BannerType,
  type SeverityColors,
  type TextBannerType,
} from "@features/csm-admin/banners/bannerHtml";
import { nextStackOrder } from "@features/csm-admin/banners/bannerOrder";
import { fromApiTimestamp, scheduleError, scheduleLocksStatus, statusOnSchedule, toSavePortalBanner } from "@features/csm-admin/banners/bannerSave";
import { useCreatePortalBanner, usePortalBanners, useUpdatePortalBanner, type PortalBanner } from "@features/csm-admin/banners/usePortalBanners";
import Editor from "@components/rich-text-editor/Editor";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { isBlankHtml, sanitizeAnnouncementHtml } from "@utils/sanitizeHtml";

type BannerStatus = "draft" | "published" | "unpublished";

interface Banner {
  id: string;
  status: BannerStatus;
  sortOrder: number;
  visibleInCustomerPortal: boolean;
  visibleInCsmPortal: boolean;
  html: string;
  startsOn: string;
  expiresOn: string;
}

interface BannerDraft extends BannerContent {
  status: BannerStatus;
  sortOrder: string;
  visibleInCustomerPortal: boolean;
  visibleInCsmPortal: boolean;
  startsOn: string;
  expiresOn: string;
}

const STATUS_LABEL: Record<BannerStatus, string> = {
  draft: "Draft",
  published: "Published",
  unpublished: "Unpublished",
};

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, DEFAULT_ROWS_PER_PAGE, BE_MAX_PAGE_LIMIT];

/** Oxygen orange and the Acrylic Purple primary, used only as an outline. */
const CUSTOMER_TAG = "#ff7300";
const CSM_TAG = "#646cff";

function outlinedTag(color: string) {
  return { color, borderColor: color };
}

function toPageBanner(row: PortalBanner): Banner {
  return {
    id: row.id,
    status: row.status,
    sortOrder: row.sortOrder,
    visibleInCustomerPortal: row.visibleInCustomerPortal,
    visibleInCsmPortal: row.visibleInCsmPortal,
    html: row.html,
    startsOn: fromApiTimestamp(row.startsAt),
    expiresOn: fromApiTimestamp(row.expiresAt),
  };
}

function emptyDraft(order: number): BannerDraft {
  return {
    ...EMPTY_BANNER_CONTENT,
    status: "draft",
    sortOrder: String(order),
    visibleInCustomerPortal: false,
    visibleInCsmPortal: false,
    startsOn: "",
    expiresOn: "",
  };
}

function draftFromBanner(banner: Banner): BannerDraft {
  const parsed = parseBannerHtml(banner.html);
  return {
    ...parsed,
    title: "",
    message: parsed.type === "image" ? "" : messageKeepingTitle(parsed.message, parsed.title),
    status: banner.status,
    sortOrder: String(banner.sortOrder),
    visibleInCustomerPortal: banner.visibleInCustomerPortal,
    visibleInCsmPortal: banner.visibleInCsmPortal,
    startsOn: banner.startsOn,
    expiresOn: banner.expiresOn,
  };
}

function hexChannels(value: string): [number, number, number] {
  const hex = cssColorToHex(value, "#000000").slice(1);
  return [parseInt(hex.slice(0, 2), 16), parseInt(hex.slice(2, 4), 16), parseInt(hex.slice(4, 6), 16)];
}

function channelsToHex(channels: [number, number, number]): string {
  return `#${channels.map((channel) => channel.toString(16).padStart(2, "0")).join("")}`;
}

function ColorField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}): JSX.Element {
  const hex = cssColorToHex(value, "#000000");
  const channels = hexChannels(hex);
  const [hexText, setHexText] = useState(hex);
  useEffect(() => setHexText(hex), [hex]);

  const commitHex = (next: string): void => {
    setHexText(next);
    const candidate = next.trim().startsWith("#") ? next.trim() : `#${next.trim()}`;
    if (/^#[0-9a-f]{6}$/i.test(candidate)) onChange(cssColorToHex(candidate, ""));
  };

  const commitChannel = (index: 0 | 1 | 2, raw: string): void => {
    if (!/^\d{0,3}$/.test(raw)) return;
    if (raw === "") return;
    const next = [...channels] as [number, number, number];
    next[index] = Math.max(0, Math.min(255, Number(raw)));
    onChange(channelsToHex(next));
  };

  return (
    <Box>
      <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 0.5 }}>
        {label}
      </Typography>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: "40px minmax(96px, 1.3fr) repeat(3, minmax(58px, 0.7fr))",
          gap: 1,
          alignItems: "center",
        }}
      >
      <Box
        component="input"
        type="color"
        aria-label={label}
        value={hex}
        onChange={(event: { target: { value: string } }) => onChange(event.target.value)}
        sx={(theme) => ({
          width: 40,
          height: 40,
          p: 0.25,
          border: 1,
          borderColor: "divider",
          borderRadius: 1,
          bgcolor: "background.paper",
          cursor: "pointer",
          ...theme.applyStyles("dark", { borderColor: "rgba(255,255,255,0.45)" }),
        })}
      />
      <TextField
        label="Hex"
        size="small"
        value={hexText}
        onChange={(event) => commitHex(event.target.value)}
        onBlur={() => {
          const candidate = hexText.trim().startsWith("#") ? hexText.trim() : `#${hexText.trim()}`;
          const parsed = cssColorToHex(candidate, "");
          if (parsed) {
            onChange(parsed);
            setHexText(parsed);
          } else {
            setHexText(hex);
          }
        }}
        slotProps={{ htmlInput: { "aria-label": `${label} hex`, spellCheck: false } }}
        sx={{ "& input": { fontFamily: "monospace" } }}
      />
      {(["R", "G", "B"] as const).map((name, index) => (
        <TextField
          key={name}
          label={name}
          size="small"
          type="number"
          value={channels[index]}
          onChange={(event) => commitChannel(index as 0 | 1 | 2, event.target.value)}
          slotProps={{ htmlInput: { "aria-label": `${label} ${name}`, min: 0, max: 255 } }}
          sx={{ "& input::-webkit-outer-spin-button, & input::-webkit-inner-spin-button": { appearance: "none", m: 0 } }}
        />
      ))}
      </Box>
    </Box>
  );
}

function FormatChoice({
  title,
  detail,
  selected,
  onClick,
}: {
  title: string;
  detail: string;
  selected: boolean;
  onClick: () => void;
}): JSX.Element {
  return (
    <Box
      component="button"
      type="button"
      aria-pressed={selected}
      aria-label={`Banner ${title}`}
      onClick={onClick}
      sx={(theme) => ({
        p: 1.25,
        borderRadius: 1,
        border: 2,
        borderColor: selected ? "primary.main" : "divider",
        bgcolor: selected ? "action.selected" : "background.paper",
        // A native button does not inherit the theme text colour, so the
        // title stays black in dark mode unless it is set here.
        color: "text.primary",
        cursor: "pointer",
        textAlign: "left",
        "&:hover": { borderColor: "primary.main", bgcolor: "action.hover" },
        ...theme.applyStyles("dark", {
          borderColor: selected ? theme.palette.primary.main : "rgba(255,255,255,0.45)",
          bgcolor: selected ? theme.palette.action.selected : "rgba(255,255,255,0.06)",
        }),
      })}
    >
      <Typography variant="body2" color="text.primary" sx={{ fontWeight: 700 }}>
        {title}
      </Typography>
      <Typography variant="caption" color="text.secondary">
        {detail}
      </Typography>
    </Box>
  );
}

function BannerTypePicker({
  value,
  tones,
  onFormat,
  onTextType,
}: {
  value: BannerType;
  tones: SeverityColors;
  onFormat: (format: "image" | "text") => void;
  onTextType: (type: TextBannerType) => void;
}): JSX.Element {
  const isImage = value === "image";
  return (
    <Stack spacing={1.5}>
      <Typography variant="subtitle2" color="text.primary">Banner</Typography>
      <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 1 }}>
        <FormatChoice title="Image" detail="A picture. Clicking it opens the link." selected={isImage} onClick={() => onFormat("image")} />
        <FormatChoice title="Text" detail="A message. Pick a type below." selected={!isImage} onClick={() => onFormat("text")} />
      </Box>
      {!isImage && (
        <Stack spacing={0.75}>
          <Typography variant="subtitle2" color="text.primary">Types</Typography>
          {TEXT_BANNER_TYPES.map((type) => {
            const selected = value === type;
            const choose = (): void => onTextType(type);
            const onKeyDown = (event: KeyboardEvent): void => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                choose();
              }
            };
            if (type === "custom") {
              return (
                <Box
                  key={type}
                  component="button"
                  type="button"
                  aria-pressed={selected}
                  aria-label="Banner type Custom"
                  onClick={choose}
                  sx={(theme) => ({
                    px: 1.5,
                    py: 1,
                    borderRadius: 1,
                    border: 2,
                    borderColor: selected ? "primary.main" : "divider",
                    bgcolor: "background.paper",
                    color: "text.primary",
                    cursor: "pointer",
                    textAlign: "left",
                    "&:hover": { borderColor: "primary.main" },
                    ...theme.applyStyles("dark", {
                      borderColor: selected ? theme.palette.primary.main : "rgba(255,255,255,0.45)",
                      bgcolor: "rgba(255,255,255,0.06)",
                    }),
                  })}
                >
                  <Typography variant="body2" color="text.primary" sx={{ fontWeight: 700 }}>
                    Custom
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    Choose the text and background colours.
                  </Typography>
                </Box>
              );
            }
            return (
              <Alert
                key={type}
                severity={type}
                role="button"
                tabIndex={0}
                aria-pressed={selected}
                aria-label={`Banner type ${BANNER_TYPE_LABEL[type]}`}
                onClick={choose}
                onKeyDown={onKeyDown}
                sx={{
                  cursor: "pointer",
                  py: 0,
                  border: 2,
                  borderColor: selected ? "primary.main" : "divider",
                  backgroundColor: `${tones[type].backgroundColor} !important`,
                  backgroundImage: "none",
                  backdropFilter: "none",
                  WebkitBackdropFilter: "none",
                  // The strip stays the light banner colour in both schemes, so
                  // the name has to stay the dark tone colour. Dark mode would
                  // otherwise paint it light on that pale background.
                  color: `${tones[type].textColor} !important`,
                  "& .MuiAlert-message": { color: `${tones[type].textColor} !important` },
                  "& .MuiAlert-icon": { color: `${tones[type].iconColor} !important` },
                  boxShadow: selected ? 1 : 0,
                  "&:hover": { borderColor: "primary.main", boxShadow: 1 },
                }}
              >
                {BANNER_TYPE_LABEL[type]}
              </Alert>
            );
          })}
        </Stack>
      )}
    </Stack>
  );
}

function contentOf(draft: BannerDraft): BannerContent {
  if (draft.type === "image") {
    return {
      type: "image",
      title: "",
      message: "",
      textColor: draft.textColor,
      backgroundColor: draft.backgroundColor,
      iconColor: "",
      imageUrl: draft.imageUrl,
      linkUrl: draft.linkUrl,
      closeable: draft.closeable,
    };
  }
  return {
    type: draft.type,
    title: "",
    message: draft.message,
    textColor: draft.textColor,
    backgroundColor: draft.backgroundColor,
    iconColor: draft.iconColor,
    imageUrl: "",
    linkUrl: "",
    closeable: draft.closeable,
  };
}

function oxygenAlertColors(theme: Theme): SeverityColors {
  const tone = (name: "info" | "warning" | "error" | "success"): AlertTone => {
    const swatch =
      (theme as Theme & { colorSchemes?: { light?: { palette: Theme["palette"] } } }).colorSchemes?.light?.palette?.[name] ??
      theme.palette[name];
    const light = cssColorToHex(String(swatch.light), cssColorToHex(String(swatch.main), "#ed6c02"));
    const main = cssColorToHex(String(swatch.main), light);
    return {
      backgroundColor: cssColorToHex(lighten(light, 0.84), light),
      textColor: cssColorToHex(darken(light, 0.6), "#000000"),
      iconColor: main,
    };
  };
  return { info: tone("info"), warning: tone("warning"), error: tone("error"), success: tone("success") };
}

export default function CsmBannersPage(): JSX.Element {
  const theme = useTheme();
  const palette = useMemo(() => oxygenAlertColors(theme), [theme]);
  const bannersQuery = usePortalBanners();
  const createBanner = useCreatePortalBanner();
  const updateBanner = useUpdatePortalBanner();
  const banners = useMemo(() => (bannersQuery.data ?? []).map(toPageBanner), [bannersQuery.data]);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [draft, setDraft] = useState<BannerDraft>(() => emptyDraft(0));
  const [showErrors, setShowErrors] = useState(false);
  const [saveError, setSaveError] = useState("");
  const saving = createBanner.isPending || updateBanner.isPending;
  const [editorKey, setEditorKey] = useState(0);
  const [statusTab, setStatusTab] = useState<BannerStatus>("published");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const now = useMemo(() => new Date(), []);

  const sorted = useMemo(() => {
    const scheduled = banners.map((banner) => {
      const status = statusOnSchedule(banner.status, banner.startsOn, banner.expiresOn, now);
      if (status === banner.status) return banner;
      return {
        ...banner,
        status,
        ...(status === "unpublished" ? { visibleInCustomerPortal: false, visibleInCsmPortal: false } : {}),
      };
    });
    return [...scheduled].sort((a, b) => a.sortOrder - b.sortOrder || a.id.localeCompare(b.id));
  }, [banners, now]);
  const visible = useMemo(
    () => sorted.filter((banner) => banner.status === statusTab),
    [sorted, statusTab],
  );
  const pageBanners = visible.slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage);
  useEffect(() => {
    const lastPage = Math.max(0, Math.ceil(visible.length / rowsPerPage) - 1);
    if (page > lastPage) setPage(lastPage);
  }, [page, rowsPerPage, visible.length]);
  const previewHtml = buildBannerHtml(contentOf(draft), editingId ?? "");
  const orderLocked = draft.status === "unpublished";
  const lockedStatus = scheduleLocksStatus(draft.startsOn, draft.expiresOn, now);
  const imageMissing = showErrors && draft.type === "image" && !draft.imageUrl.trim();
  const messageMissing = showErrors && draft.type !== "image" && isBlankHtml(draft.message);
  const orderValue = Number(draft.sortOrder);
  const orderMissing =
    showErrors && !orderLocked && (draft.sortOrder.trim() === "" || !Number.isInteger(orderValue) || orderValue < 0);
  const scheduleMessage = showErrors ? scheduleError(draft.startsOn, draft.expiresOn) : "";

  const applyFormat = (format: "image" | "text"): void => {
    setDraft((current) => {
      if (format === "image") return { ...current, type: "image", ...colorsForBannerType("image", palette) };
      if (current.type !== "image") return current;
      return { ...current, type: "info", ...colorsForBannerType("info", palette) };
    });
  };

  const applyTextType = (type: TextBannerType): void => {
    setDraft((current) => {
      if (type === "custom" && current.type === "custom") return current;
      return { ...current, type, ...colorsForBannerType(type, palette) };
    });
  };

  const openNew = (): void => {
    setDraft({
      ...emptyDraft(nextStackOrder(sorted.map((banner) => ({ id: banner.id, sortOrder: banner.sortOrder, unpublished: banner.status === "unpublished" })))),
      type: "info",
      ...colorsForBannerType("info", palette),
    });
    setShowErrors(false);
    setSaveError("");
    setEditorKey((key) => key + 1);
    setCreating(true);
    setEditingId(crypto.randomUUID());
  };

  const openEdit = (banner: Banner): void => {
    const parsed = draftFromBanner(banner);
    const next = parsed.type === "custom" ? parsed : { ...parsed, ...colorsForBannerType(parsed.type, palette) };
    const locked = scheduleLocksStatus(next.startsOn, next.expiresOn, now);
    setDraft({
      ...next,
      ...(locked ? { status: locked } : {}),
      ...(locked === "unpublished" ? { visibleInCustomerPortal: false, visibleInCsmPortal: false } : {}),
    });
    setShowErrors(false);
    setSaveError("");
    setEditorKey((key) => key + 1);
    setCreating(false);
    setEditingId(banner.id);
  };

  const setStatus = (status: BannerStatus): void => {
    setDraft((current) => {
      const next = { ...current, status };
      if (status === "unpublished") {
        next.visibleInCustomerPortal = false;
        next.visibleInCsmPortal = false;
      }
      if (current.status === "unpublished" && status !== "unpublished" && editingId && !creating) {
        const others = sorted.filter((banner) => banner.id !== editingId && banner.status !== "unpublished");
        next.sortOrder = String(others.length);
      }
      return next;
    });
  };

  const save = (): void => {
    setShowErrors(true);
    const order = Number(draft.sortOrder);
    const orderBad = !orderLocked && (draft.sortOrder.trim() === "" || !Number.isInteger(order) || order < 0);
    if (orderBad) return;
    if (draft.type === "image" && !draft.imageUrl.trim()) return;
    if (draft.type !== "image" && isBlankHtml(draft.message)) return;
    if (scheduleError(draft.startsOn, draft.expiresOn)) return;
    setShowErrors(false);
    setSaveError("");
    const id = editingId;
    if (!id || saving) return;
    const html = buildBannerHtml(contentOf(draft), creating ? "" : id);
    const status = statusOnSchedule(draft.status, draft.startsOn, draft.expiresOn, now);
    const input = toSavePortalBanner({
      status,
      sortOrder: order,
      visibleInCustomerPortal: draft.visibleInCustomerPortal,
      visibleInCsmPortal: draft.visibleInCsmPortal,
      html,
      startsOn: draft.startsOn,
      expiresOn: draft.expiresOn,
    });
    const done = (): void => {
      setStatusTab(status);
      setCreating(false);
      setEditingId(null);
    };
    const fail = (err: unknown): void => {
      setSaveError(err instanceof Error && err.message ? err.message : "Could not save the banner.");
    };
    if (creating) {
      createBanner.mutate(input, { onSuccess: done, onError: fail });
      return;
    }
    updateBanner.mutate({ id, input }, { onSuccess: done, onError: fail });
  };

  const setWhen = (key: "startsOn" | "expiresOn", value: string): void => {
    setDraft((current) => {
      const next = { ...current, [key]: value };
      const locked = scheduleLocksStatus(next.startsOn, next.expiresOn, now);
      if (locked === "unpublished") {
        return { ...next, status: "unpublished", visibleInCustomerPortal: false, visibleInCsmPortal: false };
      }
      if (locked) return { ...next, status: locked };
      return next;
    });
  };

  return (
    <Stack spacing={3}>
      <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <Typography variant="h6">Banners</Typography>
        <Button variant="contained" startIcon={<Plus size={16} />} onClick={openNew} disabled={bannersQuery.isPending}>
          New banner
        </Button>
      </Box>

      <Tabs
        value={statusTab}
        onChange={(_, value: BannerStatus) => {
          setStatusTab(value);
          setPage(0);
        }}
        sx={{ borderBottom: 1, borderColor: "divider" }}
      >
        {(["published", "draft", "unpublished"] as const).map((status) => (
          <Tab
            key={status}
            value={status}
            label={`${STATUS_LABEL[status]} (${sorted.filter((banner) => banner.status === status).length})`}
          />
        ))}
      </Tabs>

      <Stack spacing={2}>
        {bannersQuery.isPending && <Typography color="text.secondary">Loading banners…</Typography>}
        {bannersQuery.isError && (
          <Alert severity="error">{bannersQuery.error?.message || "Could not load banners."}</Alert>
        )}
        {!bannersQuery.isPending && !bannersQuery.isError && visible.length === 0 && (
          <Typography color="text.secondary">No {STATUS_LABEL[statusTab].toLowerCase()} banners.</Typography>
        )}
        {pageBanners.map((banner) => {
          const type = parseBannerHtml(banner.html).type;
          const kind = type === "image" ? "Image" : "Text";
          return (
          <Paper
            key={banner.id}
            variant="outlined"
            sx={(theme) => ({
              borderRadius: 2,
              overflow: "hidden",
              border: "1px solid",
              borderColor: "rgba(0,0,0,0.32)",
              // palette.mode stays on the default scheme, so a mode check never
              // sees dark. applyStyles is what actually switches with the toggle.
              ...theme.applyStyles("dark", {
                borderColor: "rgba(255,255,255,0.78)",
              }),
              "&:hover": { borderColor: "primary.light" },
            })}
          >
            <Box
              data-testid={`banner-body-${banner.id}`}
              sx={{
                overflow: "hidden",
                "& [data-banner='1']": {
                  height: "3rem !important",
                  minHeight: "3rem !important",
                  maxHeight: "3rem !important",
                  paddingTop: "0 !important",
                  paddingBottom: "0 !important",
                },
                "& [data-banner-type='info'], & [data-banner-type='warning'], & [data-banner-type='error'], & [data-banner-type='success']": {
                  backgroundImage: "none",
                  backdropFilter: "none",
                  WebkitBackdropFilter: "none",
                },
                "& [data-banner-type='info']": { backgroundColor: `${palette.info.backgroundColor} !important` },
                "& [data-banner-type='warning']": { backgroundColor: `${palette.warning.backgroundColor} !important` },
                "& [data-banner-type='error']": { backgroundColor: `${palette.error.backgroundColor} !important` },
                "& [data-banner-type='success']": { backgroundColor: `${palette.success.backgroundColor} !important` },
                "& [data-banner-text] p, & [data-banner-text] div": { margin: "0 !important" },
              }}
              dangerouslySetInnerHTML={{ __html: sanitizeAnnouncementHtml(banner.html) }}
            />
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, px: 1.5, py: 1 }}>
              <IconButton size="small" aria-label="Edit" onClick={() => openEdit(banner)} sx={{ flex: "none" }}>
                <Pencil size={16} />
              </IconButton>
              <Stack direction="row" spacing={0.75} useFlexGap sx={{ flex: "1 1 auto", flexWrap: "wrap" }}>
                  <Chip size="small" variant="outlined" label={kind} sx={outlinedTag("text.primary")} />
                  {banner.status !== "unpublished" && banner.visibleInCustomerPortal && (
                    <Chip size="small" variant="outlined" label="Customer portal" sx={outlinedTag(CUSTOMER_TAG)} />
                  )}
                  {banner.status !== "unpublished" && banner.visibleInCsmPortal && (
                    <Chip size="small" variant="outlined" label="CSM portal" sx={outlinedTag(CSM_TAG)} />
                  )}
              </Stack>
            </Box>
          </Paper>
          );
        })}
      </Stack>
      {!bannersQuery.isPending && !bannersQuery.isError && (
        <TablePagination
          component="div"
          count={visible.length}
          page={page}
          onPageChange={(_, nextPage) => setPage(nextPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(event: ChangeEvent<HTMLInputElement>) => {
            setRowsPerPage(parseInt(event.target.value, 10));
            setPage(0);
          }}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          showFirstButton
          showLastButton
        />
      )}

      <Dialog
        open={editingId !== null}
        onClose={() => {
          setCreating(false);
          setEditingId(null);
        }}
        maxWidth="lg"
        fullWidth
      >
        <DialogTitle>{creating ? "New banner" : "Edit banner"}</DialogTitle>
        <DialogContent
          dividers
          sx={(theme) => ({
            color: "text.primary",
            "& .MuiFormControlLabel-label, & .MuiInputBase-input, & .MuiSelect-select": {
              color: "text.primary",
            },
            // Acrylic dark fields are a faint black fill with a near-invisible
            // border, so the boxes disappear on the dialog.
            ...theme.applyStyles("dark", {
              "& .MuiOutlinedInput-root": {
                backgroundColor: "rgba(255,255,255,0.08)",
              },
              "& .MuiOutlinedInput-notchedOutline": {
                borderColor: "rgba(255,255,255,0.45)",
              },
              "& .MuiOutlinedInput-root:hover .MuiOutlinedInput-notchedOutline": {
                borderColor: "rgba(255,255,255,0.72)",
              },
              "& .MuiOutlinedInput-root.Mui-focused .MuiOutlinedInput-notchedOutline": {
                borderColor: theme.palette.primary.main,
              },
              "& .MuiOutlinedInput-root.Mui-disabled .MuiInputBase-input": {
                color: "rgba(255,255,255,0.6)",
                WebkitTextFillColor: "rgba(255,255,255,0.6)",
              },
              "& .MuiOutlinedInput-root.Mui-disabled .MuiOutlinedInput-notchedOutline": {
                borderColor: "rgba(255,255,255,0.28)",
              },
              "& .MuiInputLabel-root": {
                color: "rgba(255,255,255,0.86)",
              },
              "& .MuiInputLabel-root.Mui-focused": {
                color: theme.palette.primary.main,
              },
              "& .MuiInputLabel-root.Mui-disabled": {
                color: "rgba(255,255,255,0.55)",
              },
              "& .MuiFormHelperText-root": {
                color: "rgba(255,255,255,0.72)",
              },
              "& .MuiInputLabel-root.Mui-error, & .MuiFormHelperText-root.Mui-error": {
                color: theme.palette.error.main,
              },
              "& .MuiOutlinedInput-root.Mui-error .MuiOutlinedInput-notchedOutline": {
                borderColor: theme.palette.error.main,
              },
              "& input[type='datetime-local'], & input[type='color']": {
                colorScheme: "dark",
              },
            }),
          })}
        >
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", md: "minmax(0, 1.15fr) minmax(280px, 0.85fr)" },
              gap: 3,
              pt: 1,
              alignItems: "start",
            }}
          >
            <Stack spacing={2}>
              <BannerTypePicker value={draft.type} tones={palette} onFormat={applyFormat} onTextType={applyTextType} />
              {draft.type === "image" ? (
                <>
                  <TextField
                    label="Image URL"
                    required
                    error={imageMissing}
                    value={draft.imageUrl}
                    helperText={imageMissing ? "Required" : "The picture fills the strip."}
                    onChange={(event) => setDraft({ ...draft, imageUrl: event.target.value })}
                  />
                  <TextField
                    label="Link URL"
                    value={draft.linkUrl}
                    helperText="Optional. Clicking the banner opens this address."
                    onChange={(event) => setDraft({ ...draft, linkUrl: event.target.value })}
                  />
                </>
              ) : (
                <>
                  {draft.type === "custom" && (
                    <Stack spacing={1.25}>
                      <ColorField
                        label="Text"
                        value={draft.textColor}
                        onChange={(value) => setDraft({ ...draft, textColor: value })}
                      />
                      <ColorField
                        label="Background"
                        value={draft.backgroundColor}
                        onChange={(value) => setDraft({ ...draft, backgroundColor: value })}
                      />
                    </Stack>
                  )}
                  <Box>
                    <Typography
                      variant="body2"
                      component="label"
                      htmlFor="banner-message"
                      color={messageMissing ? "error" : "text.primary"}
                      sx={{ mb: 0.5, display: "block" }}
                    >
                      Message
                      <Box component="span" aria-hidden="true" sx={{ ml: 0.25 }}>
                        *
                      </Box>
                    </Typography>
                    <Box
                      sx={(theme) =>
                        messageMissing
                          ? {
                              "& .MuiPaper-outlined": {
                                borderColor: "error.main",
                                "&:hover:not(:focus-within)": { borderColor: "error.main" },
                                "&:focus-within": { borderColor: "error.main" },
                              },
                            }
                          : {
                              ...theme.applyStyles("dark", {
                                "& .MuiPaper-outlined": { borderColor: "rgba(255,255,255,0.45)" },
                              }),
                            }
                      }
                    >
                      <Editor
                        key={editorKey}
                        id="banner-message"
                        value={draft.message}
                        onChange={(html) => setDraft((current) => ({ ...current, message: html }))}
                        toolbarVariant="describeIssue"
                        minHeight={88}
                        maxHeight={180}
                        placeholder="Write the banner text"
                        showKeyboardHint={false}
                      />
                    </Box>
                    {messageMissing && (
                      <Typography variant="caption" color="error" sx={{ mt: 0.5, display: "block" }}>
                        Required
                      </Typography>
                    )}
                  </Box>
                </>
              )}
              <FormControlLabel
                control={
                  <Checkbox
                    checked={draft.closeable}
                    onChange={(event) => setDraft({ ...draft, closeable: event.target.checked })}
                  />
                }
                label="Closeable"
              />
              <TextField
                label="Place in the stack"
                type="number"
                required={!orderLocked}
                disabled={orderLocked}
                error={orderMissing}
                value={draft.sortOrder}
                helperText={
                  orderMissing
                    ? draft.sortOrder.trim() === ""
                      ? "Required"
                      : "Enter a whole number, 0 or more."
                    : orderLocked
                      ? "Unpublished banners sit after the stack. The banner below this one takes its place."
                      : sorted.filter((banner) => banner.status !== "unpublished").length === 0
                        ? "This is the first banner, so it sits at the top."
                        : "0 is the top. A number already in use moves that banner and the ones below it down."
                }
                onChange={(event) => setDraft({ ...draft, sortOrder: event.target.value })}
              />
              <Box sx={{ display: "flex", flexWrap: "wrap", columnGap: 3, rowGap: 1, alignItems: "center" }}>
                <FormControlLabel
                  control={
                    <Switch
                      checked={draft.status !== "unpublished" && draft.visibleInCustomerPortal}
                      disabled={draft.status === "unpublished"}
                      onChange={(event) => setDraft({ ...draft, visibleInCustomerPortal: event.target.checked })}
                    />
                  }
                  label="Show on the customer portal"
                />
                <FormControlLabel
                  control={
                    <Switch
                      checked={draft.status !== "unpublished" && draft.visibleInCsmPortal}
                      disabled={draft.status === "unpublished"}
                      onChange={(event) => setDraft({ ...draft, visibleInCsmPortal: event.target.checked })}
                    />
                  }
                  label="Show on the CSM portal"
                />
              </Box>
              <TextField
                select
                label="Status"
                value={draft.status}
                disabled={lockedStatus !== null}
                helperText={
                  lockedStatus === "draft"
                    ? "This stays a draft until the start date."
                    : lockedStatus === "unpublished"
                      ? "The end date has passed, so this is unpublished."
                      : undefined
                }
                onChange={(event) => setStatus(event.target.value as BannerStatus)}
              >
                <MenuItem value="draft">Draft — saved, not shown</MenuItem>
                <MenuItem value="published">Published — shown where the switches are on</MenuItem>
                <MenuItem value="unpublished">Unpublished — hidden on both portals</MenuItem>
              </TextField>
              <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
                <TextField
                  label="Starts"
                  type="datetime-local"
                  error={scheduleMessage !== ""}
                  value={draft.startsOn}
                  helperText={scheduleMessage || "Empty means it can show as soon as it is published."}
                  onChange={(event) => setWhen("startsOn", event.target.value)}
                  slotProps={{ inputLabel: { shrink: true } }}
                />
                <TextField
                  label="Ends"
                  type="datetime-local"
                  error={scheduleMessage !== ""}
                  value={draft.expiresOn}
                  helperText={scheduleMessage || "After this it becomes unpublished."}
                  onChange={(event) => setWhen("expiresOn", event.target.value)}
                  slotProps={{ inputLabel: { shrink: true } }}
                />
              </Box>
              {saveError && <Alert severity="error">{saveError}</Alert>}
            </Stack>
            <Paper
              variant="outlined"
              sx={(theme) => ({
                p: 2,
                position: { md: "sticky" },
                top: 8,
                color: "text.primary",
                ...theme.applyStyles("dark", { borderColor: "rgba(255,255,255,0.45)" }),
              })}
            >
              <Typography variant="subtitle1" color="text.primary">Preview</Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
                This is the banner people will see.
              </Typography>
              <Box
                data-testid="banner-preview"
                sx={{
                  borderRadius: 1,
                  overflow: "hidden",
                  border: 1,
                  borderColor: "divider",
                  "& [data-banner='1']": {
                    height: "3rem !important",
                    minHeight: "3rem !important",
                    maxHeight: "3rem !important",
                    paddingTop: "0 !important",
                    paddingBottom: "0 !important",
                  },
                  "& [data-banner-text] p, & [data-banner-text] div": { margin: "0 !important" },
                }}
                dangerouslySetInnerHTML={{ __html: sanitizeAnnouncementHtml(previewHtml) }}
              />
              <Accordion disableGutters elevation={0} sx={{ mt: 1, "&:before": { display: "none" } }}>
                <AccordionSummary expandIcon={<ChevronDown size={16} />}>
                  <Typography variant="body2" color="text.primary">Show HTML source</Typography>
                </AccordionSummary>
                <AccordionDetails>
                  <Box
                    component="pre"
                    sx={{
                      m: 0,
                      p: 1.5,
                      overflow: "auto",
                      whiteSpace: "pre-wrap",
                      wordBreak: "break-word",
                      fontSize: 12,
                      color: "text.primary",
                      bgcolor: "action.hover",
                      borderRadius: 1,
                    }}
                  >
                    {previewHtml}
                  </Box>
                </AccordionDetails>
              </Accordion>
            </Paper>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditingId(null)}>Cancel</Button>
          <Button variant="contained" onClick={save} disabled={saving}>
            {saving ? "Saving…" : "Save"}
          </Button>
        </DialogActions>
      </Dialog>
    </Stack>
  );
}
