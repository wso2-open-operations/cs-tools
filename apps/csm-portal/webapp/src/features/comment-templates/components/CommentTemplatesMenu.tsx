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
  Box,
  Button,
  Divider,
  ListItemText,
  ListSubheader,
  Menu,
  MenuItem,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { BookmarkPlus, FileText, Pencil } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX, type MouseEvent } from "react";
import type { BeCommentTemplate } from "@api/backend/types";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useGetCommentTemplates } from "@features/comment-templates/api/useCommentTemplates";
import CommentTemplateDialog, {
  type CommentTemplateDraft,
} from "@features/comment-templates/components/CommentTemplateDialog";
import ManageCommentTemplatesDialog from "@features/comment-templates/components/ManageCommentTemplatesDialog";
import {
  filterTemplates,
  isBlankBody,
  templatePlainText,
  templatesForMode,
} from "@features/comment-templates/utils/commentTemplates";

interface CommentTemplatesMenuProps {
  /** The comment box is in work-note mode: only work note templates are offered. */
  internal: boolean;
  /** The current draft, for "Save draft as template". */
  draftHtml: string;
  disabled?: boolean;
  /** Puts a picked template's text into the comment box. */
  onInsert: (html: string) => void;
}

const PREVIEW_CHARS = 90;

/**
 * The comment box's "Templates" button: pick a saved template to insert,
 * save the current draft as one, or manage them. The list is fetched the
 * first time the menu opens.
 */
export default function CommentTemplatesMenu({
  internal,
  draftHtml,
  disabled = false,
  onInsert,
}: CommentTemplatesMenuProps): JSX.Element {
  const { isAdmin } = usePortalAccess();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const [everOpened, setEverOpened] = useState(false);
  const [search, setSearch] = useState("");
  const [editing, setEditing] = useState<{ key: number; draft: CommentTemplateDraft } | null>(null);
  const [manageOpen, setManageOpen] = useState(false);
  const { data: templates = [], isLoading, error } = useGetCommentTemplates(everOpened);

  const offered = useMemo(
    () => filterTemplates(templatesForMode(templates, internal), search),
    [templates, internal, search],
  );
  const mine = offered.filter((t) => !t.shared);
  const shared = offered.filter((t) => t.shared);

  const open = (e: MouseEvent<HTMLElement>) => {
    setEverOpened(true);
    setSearch("");
    setAnchor(e.currentTarget);
  };
  const close = () => setAnchor(null);

  const pick = (t: BeCommentTemplate) => {
    close();
    onInsert(t.body);
  };

  const startEdit = (draft: CommentTemplateDraft) => setEditing({ key: Date.now(), draft });

  const item = (t: BeCommentTemplate) => {
    const preview = templatePlainText(t.body);
    return (
      <MenuItem key={t.id} onClick={() => pick(t)} sx={{ whiteSpace: "normal", alignItems: "flex-start" }}>
        <ListItemText
          primary={t.name}
          secondary={preview.length > PREVIEW_CHARS ? `${preview.slice(0, PREVIEW_CHARS)}…` : preview}
        />
      </MenuItem>
    );
  };

  return (
    <>
      <Button
        size="small"
        variant="text"
        startIcon={<FileText size={14} />}
        onClick={open}
        disabled={disabled}
        aria-haspopup="menu"
        sx={{ fontSize: "0.8rem" }}
      >
        Templates
      </Button>
      <Menu
        anchorEl={anchor}
        open={!!anchor}
        onClose={close}
        slotProps={{ paper: { sx: { width: 380, maxHeight: 480 } } }}
      >
        <Box sx={{ px: 1.5, pb: 1 }} onKeyDown={(e) => e.stopPropagation()}>
          <TextField
            size="small"
            fullWidth
            autoFocus
            placeholder="Search templates"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            inputProps={{ "aria-label": "Search templates" }}
          />
        </Box>
        {isLoading && (
          <Typography variant="body2" color="text.secondary" sx={{ px: 2, py: 1 }}>
            Loading templates…
          </Typography>
        )}
        {error && (
          <Typography variant="body2" color="error" sx={{ px: 2, py: 1 }}>
            {error.message}
          </Typography>
        )}
        {!isLoading && !error && offered.length === 0 && (
          <Typography variant="body2" color="text.secondary" sx={{ px: 2, py: 1 }}>
            {search
              ? "No template matches."
              : internal
                ? "No work note templates yet."
                : "No customer comment templates yet."}
          </Typography>
        )}
        {mine.length > 0 && <ListSubheader>My templates</ListSubheader>}
        {mine.map(item)}
        {shared.length > 0 && <ListSubheader>Shared</ListSubheader>}
        {shared.map(item)}
        <Divider />
        <MenuItem
          disabled={isBlankBody(draftHtml)}
          onClick={() => {
            close();
            startEdit({
              name: "",
              body: draftHtml,
              appliesTo: internal ? "WORK_NOTE" : "COMMENT",
              shared: false,
            });
          }}
        >
          <BookmarkPlus size={16} style={{ marginRight: 8 }} />
          Save draft as template
        </MenuItem>
        <MenuItem
          onClick={() => {
            close();
            setManageOpen(true);
          }}
        >
          <Pencil size={16} style={{ marginRight: 8 }} />
          Manage templates
        </MenuItem>
      </Menu>

      {manageOpen && (
        <ManageCommentTemplatesDialog
          open
          templates={templates}
          isAdmin={isAdmin}
          onEdit={(t) =>
            startEdit({ id: t.id, name: t.name, body: t.body, appliesTo: t.appliesTo, shared: t.shared })
          }
          onClose={() => setManageOpen(false)}
        />
      )}
      {editing && (
        <CommentTemplateDialog
          key={editing.key}
          open
          initial={editing.draft}
          canShare={isAdmin}
          onClose={() => setEditing(null)}
        />
      )}
    </>
  );
}
