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
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  List,
  ListItem,
  ListItemText,
  Tab,
  Tabs,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, Copy, Pencil, Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { Link as RouterLink } from "react-router";
import PresetEditorDialog from "@features/csm-admin/dashboards/components/PresetEditorDialog";
import SectionEditorDialog from "@features/csm-admin/dashboards/components/SectionEditorDialog";
import {
  useDashboardFilterPresets,
  useDashboardSharedSections,
} from "@features/csm-admin/dashboards/api/useDashboardSharedConfig";
import {
  presetsFileFromDraft,
  saveSharedConfigDraft,
  sectionsFileFromDraft,
  seedSharedConfigDraft,
  useSharedConfigDraft,
  type PresetDraft,
  type SectionDraft,
} from "@features/csm-admin/dashboards/utils/sharedConfigDraftsStorage";

/**
 * Designs the two SHARED dashboard config files: the named filter presets and
 * the reusable sections that dashboard definitions reference by name.
 *
 * Same deploy story as the dashboard builder itself, for the same reason —
 * these are deployment configuration with no write API, so the output of this
 * page is JSON to hand to a maintainer, and everything until then lives in
 * `localStorage`. Nothing here changes what is currently deployed.
 *
 * Seeded from what IS deployed on first open, so an admin edits reality
 * rather than retyping it. After that first seed the local draft is
 * authoritative, so a deletion sticks instead of reappearing (see
 * `seedSharedConfigDraft`).
 */
export default function CsmDashboardSharedConfigPage(): JSX.Element {
  const {
    data: deployedPresets,
    isSuccess: presetsLoaded,
    isError: presetsFailed,
    refetch: refetchPresets,
  } = useDashboardFilterPresets();
  const {
    data: deployedSections,
    isSuccess: sectionsLoaded,
    isError: sectionsFailed,
    refetch: refetchSections,
  } = useDashboardSharedSections();
  const draft = useSharedConfigDraft();

  const [tab, setTab] = useState<"presets" | "sections">("presets");
  const [editingPreset, setEditingPreset] = useState<
    { preset?: PresetDraft; index?: number } | undefined
  >(undefined);
  const [editingSection, setEditingSection] = useState<
    { section?: SectionDraft; index?: number } | undefined
  >(undefined);
  const [copied, setCopied] = useState<"presets" | "sections" | undefined>(undefined);

  // Both catalogues must have SUCCEEDED, not merely settled. Seeding is a
  // one-shot, persisted decision (see the `seeded` flag), so seeding from a
  // failed request would bake a half-empty draft in permanently: the flag
  // says "already seeded", the retry never runs, and the deployed entries
  // that failed to load can never be recovered without clearing storage.
  const catalogueReady = presetsLoaded && sectionsLoaded;
  const catalogueFailed = presetsFailed || sectionsFailed;

  // Fold the deployed catalogues in once, as a render-time side effect
  // guarded by the draft's own `seeded` flag rather than an effect: this
  // writes to localStorage and then re-reads through the storage event, so
  // there is no React state to cascade (and no set-state-in-effect).
  if (catalogueReady && !draft.seeded) {
    seedSharedConfigDraft(
      (deployedPresets ?? []).map((p) => ({ name: p.name, filter: p.filter })),
      (deployedSections ?? []).map((s) => ({
        name: s.name,
        displayName: s.displayName,
        widgets: s.widgets,
      })),
    );
  }

  const deployedPresetNames = new Set((deployedPresets ?? []).map((p) => p.name));
  const deployedSectionNames = new Set((deployedSections ?? []).map((s) => s.name));

  const save = (next: Partial<Pick<typeof draft, "presets" | "sections">>): void => {
    saveSharedConfigDraft({
      presets: next.presets ?? draft.presets,
      sections: next.sections ?? draft.sections,
      seeded: draft.seeded,
    });
  };

  const copyJson = async (which: "presets" | "sections"): Promise<void> => {
    const payload =
      which === "presets"
        ? presetsFileFromDraft(draft.presets)
        : sectionsFileFromDraft(draft.sections, draft.presets);
    try {
      await navigator.clipboard.writeText(JSON.stringify(payload, null, 2));
      setCopied(which);
      setTimeout(() => setCopied(undefined), 2000);
    } catch {
      // Clipboard can be denied by permissions policy; failing silently is
      // consistent with the dashboard builder's own copy action.
    }
  };

  if (catalogueFailed) {
    // Deliberately blocks the whole page rather than showing an empty
    // designer: an empty designer here looks like "nothing is configured",
    // and copying its JSON would hand a maintainer a file that deletes every
    // deployed preset and section.
    return (
      <Alert
        severity="error"
        action={
          <Button
            size="small"
            color="inherit"
            onClick={() => {
              void refetchPresets();
              void refetchSections();
            }}
          >
            Retry
          </Button>
        }
      >
        Could not load what is currently deployed, so this page cannot show you
        an accurate starting point. Nothing has been changed.
      </Alert>
    );
  }

  if (!catalogueReady) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", py: 6 }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <Button
          component={RouterLink}
          to="/admin/dashboards"
          size="small"
          startIcon={<ArrowLeft size={16} />}
        >
          Dashboards
        </Button>
        <Typography variant="h6">Shared dashboard config</Typography>
      </Box>

      <Alert severity="info">
        Presets and sections are shared across dashboards and deployed as their
        own files. Design them here, then copy the JSON and hand it to a
        maintainer — nothing on this page changes what is deployed.
      </Alert>

      <Tabs value={tab} onChange={(_e, next: "presets" | "sections") => setTab(next)}>
        <Tab value="presets" label={`Presets (${draft.presets.length})`} />
        <Tab value="sections" label={`Sections (${draft.sections.length})`} />
      </Tabs>

      {tab === "presets" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          <Box sx={{ display: "flex", gap: 1 }}>
            <Button
              size="small"
              variant="text"
              startIcon={<Plus size={16} />}
              onClick={() => setEditingPreset({})}
            >
              New preset
            </Button>
            <Button
              size="small"
              variant="outlined"
              startIcon={<Copy size={14} />}
              onClick={() => void copyJson("presets")}
            >
              {copied === "presets" ? "Copied!" : "Copy _presets.json"}
            </Button>
          </Box>
          {draft.presets.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No presets yet.
            </Typography>
          ) : (
            <List dense disablePadding>
              {draft.presets.map((preset, index) => (
                <ListItem
                  key={preset.name}
                  disableGutters
                  secondaryAction={
                    <Box>
                      <Tooltip title="Edit this preset">
                        <IconButton
                          size="small"
                          aria-label={`Edit preset ${preset.name}`}
                          onClick={() => setEditingPreset({ preset, index })}
                        >
                          <Pencil size={16} />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Remove this preset">
                        <IconButton
                          size="small"
                          aria-label={`Remove preset ${preset.name}`}
                          onClick={() =>
                            save({ presets: draft.presets.filter((_, i) => i !== index) })
                          }
                        >
                          <Trash2 size={16} />
                        </IconButton>
                      </Tooltip>
                    </Box>
                  }
                >
                  <ListItemText
                    primary={
                      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                        {preset.name}
                        {/* Which entries already exist in the deployed file,
                            so an admin can tell what a maintainer still has
                            to land. */}
                        {!deployedPresetNames.has(preset.name) && (
                          <Chip size="small" label="not deployed" color="warning" />
                        )}
                      </Box>
                    }
                    secondary={JSON.stringify(preset.filter)}
                  />
                </ListItem>
              ))}
            </List>
          )}
        </Box>
      )}

      {tab === "sections" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          <Box sx={{ display: "flex", gap: 1 }}>
            <Button
              size="small"
              variant="text"
              startIcon={<Plus size={16} />}
              onClick={() => setEditingSection({})}
            >
              New section
            </Button>
            <Button
              size="small"
              variant="outlined"
              startIcon={<Copy size={14} />}
              onClick={() => void copyJson("sections")}
            >
              {copied === "sections" ? "Copied!" : "Copy _sections.json"}
            </Button>
          </Box>
          {draft.sections.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No sections yet.
            </Typography>
          ) : (
            <List dense disablePadding>
              {draft.sections.map((section, index) => (
                <ListItem
                  key={section.name}
                  disableGutters
                  secondaryAction={
                    <Box>
                      <Tooltip title="Edit this section">
                        <IconButton
                          size="small"
                          aria-label={`Edit section ${section.name}`}
                          onClick={() => setEditingSection({ section, index })}
                        >
                          <Pencil size={16} />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Remove this section">
                        <IconButton
                          size="small"
                          aria-label={`Remove section ${section.name}`}
                          onClick={() =>
                            save({ sections: draft.sections.filter((_, i) => i !== index) })
                          }
                        >
                          <Trash2 size={16} />
                        </IconButton>
                      </Tooltip>
                    </Box>
                  }
                >
                  <ListItemText
                    primary={
                      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                        {section.name}
                        {!deployedSectionNames.has(section.name) && (
                          <Chip size="small" label="not deployed" color="warning" />
                        )}
                      </Box>
                    }
                    secondary={`${section.displayName} · ${section.widgets.length} widget${
                      section.widgets.length === 1 ? "" : "s"
                    }`}
                  />
                </ListItem>
              ))}
            </List>
          )}
        </Box>
      )}

      {editingPreset && (
        <PresetEditorDialog
          preset={editingPreset.preset}
          takenNames={draft.presets
            .filter((_, i) => i !== editingPreset.index)
            .map((p) => p.name)}
          onClose={() => setEditingPreset(undefined)}
          onSave={(preset) => {
            save({
              presets:
                editingPreset.index === undefined
                  ? [...draft.presets, preset]
                  : draft.presets.map((p, i) => (i === editingPreset.index ? preset : p)),
            });
            setEditingPreset(undefined);
          }}
        />
      )}

      {editingSection && (
        <SectionEditorDialog
          section={editingSection.section}
          // The local draft's presets, not just the deployed ones: a preset
          // authored on the Presets tab a moment ago is exactly what an author
          // wants to reference from a section they are building now, and
          // offering only deployed ones would make a newly designed preset
          // unusable until someone deploys it. The draft already contains
          // every deployed preset (it was seeded from them), so this is a
          // superset, with local edits winning.
          presets={draft.presets}
          takenNames={draft.sections
            .filter((_, i) => i !== editingSection.index)
            .map((s) => s.name)}
          onClose={() => setEditingSection(undefined)}
          onSave={(section) => {
            save({
              sections:
                editingSection.index === undefined
                  ? [...draft.sections, section]
                  : draft.sections.map((s, i) => (i === editingSection.index ? section : s)),
            });
            setEditingSection(undefined);
          }}
        />
      )}
    </Box>
  );
}
