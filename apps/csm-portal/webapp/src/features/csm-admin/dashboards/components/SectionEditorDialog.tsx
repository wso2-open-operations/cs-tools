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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  List,
  ListItem,
  ListItemText,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Pencil, Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type { BeDashboardFilterPreset, BeDashboardWidget } from "@api/backend/types";
import WidgetEditorDialog from "@features/csm-admin/dashboards/components/WidgetEditorDialog";
import type { SectionDraft } from "@features/csm-admin/dashboards/utils/sharedConfigDraftsStorage";

interface SectionEditorDialogProps {
  /** `undefined` when designing a brand-new section. */
  section?: SectionDraft;
  /** Names already taken, excluding the one being edited. */
  takenNames: readonly string[];
  /** Shared presets a widget in this section may reference — passed straight
   * through to the widget editor. */
  presets?: readonly BeDashboardFilterPreset[];
  onClose: () => void;
  onSave: (section: SectionDraft) => void;
}

/**
 * Designs one reusable section: a name, the heading its widgets are grouped
 * under, and the widgets themselves.
 *
 * The widgets are built with the SAME dialog a dashboard's own widgets use.
 * A section's widget and a dashboard's widget are the same shape — the whole
 * point of the feature is that one is substitutable for the other — so
 * forking a second widget editor here would guarantee the two drift.
 *
 * A widget's own `section` field is not editable here and is not written out:
 * within a section, the heading comes from this section's `displayName`, and
 * the loader overwrites each included widget's `section` with it (or with the
 * including dashboard's override). Offering it would be a field that silently
 * does nothing.
 */
export default function SectionEditorDialog({
  section,
  takenNames,
  presets,
  onClose,
  onSave,
}: SectionEditorDialogProps): JSX.Element {
  const [name, setName] = useState(section?.name ?? "");
  const [displayName, setDisplayName] = useState(section?.displayName ?? "");
  const [widgets, setWidgets] = useState<BeDashboardWidget[]>(section?.widgets ?? []);
  const [editingWidget, setEditingWidget] = useState<
    { widget?: BeDashboardWidget; index?: number } | undefined
  >(undefined);

  const trimmedName = name.trim();
  const trimmedDisplayName = displayName.trim();
  const duplicate = takenNames.some((n) => n === trimmedName);
  const canSave =
    trimmedName.length > 0 &&
    trimmedDisplayName.length > 0 &&
    !duplicate &&
    widgets.length > 0;

  const upsertWidget = (widget: BeDashboardWidget): void => {
    setWidgets((current) =>
      editingWidget?.index === undefined
        ? [...current, widget]
        : current.map((w, i) => (i === editingWidget.index ? widget : w)),
    );
    setEditingWidget(undefined);
  };

  return (
    <>
      <Dialog open onClose={onClose} fullWidth maxWidth="md">
        <DialogTitle>{section ? "Edit section" : "New section"}</DialogTitle>
        <DialogContent>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
            <TextField
              size="small"
              label="Section name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              error={duplicate}
              helperText={
                duplicate
                  ? "Another section already uses this name."
                  : "The name a dashboard definition includes this section by, e.g. my-work."
              }
              slotProps={{ htmlInput: { "aria-label": "Section name" } }}
            />
            <TextField
              size="small"
              label="Section heading"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              helperText="Shown above the section's widgets, e.g. My Work. A dashboard including this section can override it."
              slotProps={{ htmlInput: { "aria-label": "Section heading" } }}
            />

            <Box>
              <Typography variant="subtitle2" gutterBottom>
                Widgets
              </Typography>
              {widgets.length === 0 ? (
                <Alert severity="info">
                  A section needs at least one widget — the loader rejects an
                  empty one, because a section that expands to nothing is a
                  dashboard missing a whole block with nothing in the logs to
                  say why.
                </Alert>
              ) : (
                <List dense disablePadding>
                  {widgets.map((widget, index) => (
                    <ListItem
                      key={widget.widgetId}
                      disableGutters
                      secondaryAction={
                        <Box>
                          <Tooltip title="Edit this widget">
                            <IconButton
                              size="small"
                              aria-label={`Edit widget ${widget.displayName}`}
                              onClick={() => setEditingWidget({ widget, index })}
                            >
                              <Pencil size={16} />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="Remove this widget">
                            <IconButton
                              size="small"
                              aria-label={`Remove widget ${widget.displayName}`}
                              onClick={() =>
                                setWidgets((c) => c.filter((_, i) => i !== index))
                              }
                            >
                              <Trash2 size={16} />
                            </IconButton>
                          </Tooltip>
                        </Box>
                      }
                    >
                      <ListItemText
                        primary={widget.displayName}
                        secondary={`${widget.resourceType} · ${widget.shape} · ${widget.gridWidth}/12`}
                      />
                    </ListItem>
                  ))}
                </List>
              )}
              <Button
                size="small"
                variant="text"
                startIcon={<Plus size={16} />}
                onClick={() => setEditingWidget({})}
                sx={{ mt: 1 }}
              >
                Add widget
              </Button>
            </Box>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="contained"
            disabled={!canSave}
            onClick={() =>
              onSave({
                name: trimmedName,
                displayName: trimmedDisplayName,
                widgets,
              })
            }
          >
            Save section
          </Button>
        </DialogActions>
      </Dialog>

      {editingWidget && (
        <WidgetEditorDialog
          presets={presets}
          widget={editingWidget.widget}
          // Pre-filled with, and only suggesting, this section's own heading:
          // the loader overwrites every included widget's `section` with the
          // section's displayName anyway, so anything else here would preview
          // one grouping and deploy another. It is also dropped on export
          // (see `widgetToDefinition`) rather than written as dead config.
          defaultSection={trimmedDisplayName}
          sectionSuggestions={trimmedDisplayName ? [trimmedDisplayName] : []}
          onClose={() => setEditingWidget(undefined)}
          onSave={upsertWidget}
        />
      )}
    </>
  );
}
