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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, Code, MessageSquare } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useState } from "react";
import { useNavigate } from "react-router";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import {
  savedCodeDashboardPath,
  usePutCodeDashboard,
  type SavedCodeDashboard,
} from "@features/csm-custom-dashboards/api/useCodeDashboards";
import { useGenerateCodeDashboard } from "@features/csm-custom-dashboards/api/useGenerateCodeDashboard";
import CodeDashboardFrame from "@features/csm-custom-dashboards/components/CodeDashboardFrame";
import DashboardNameDialog from "@features/csm-custom-dashboards/components/DashboardNameDialog";
import DiscardGeneratedDashboardDialog from "@features/csm-custom-dashboards/components/DiscardGeneratedDashboardDialog";
import GenerateChatPanel, {
  type GenerateChatMessage,
} from "@features/csm-custom-dashboards/components/GenerateChatPanel";
import { generateErrorMessage } from "@features/csm-custom-dashboards/utils/generateErrorMessage";
import { saveErrorMessage } from "@features/csm-custom-dashboards/utils/saveErrorMessage";

const MY_DASHBOARDS_PATH = "/dashboard/my";

/**
 * Dashboards > My Dashboards > new (advanced)
 * (`/dashboard/my/new-code`), opened by the Generate Dashboard page's
 * "Generate Advanced Dashboard" button (only for whoever
 * `canGenerateAdvancedDashboards`): the code-based counterpart of GenerateDashboardPage, where the
 * model writes the dashboard as code that will run in a sandbox and read live
 * data through a fixed set of portal APIs.
 *
 * Same shape as GenerateDashboardPage, reusing its chat panel and discard
 * dialog unchanged: each message goes to `useGenerateCodeDashboard` with the
 * current source; the reply is added to the chat and the returned source
 * replaces the draft. The left side runs the source in a sandboxed iframe
 * (CodeDashboardFrame, with the viewer's real data); the source itself is
 * never shown.
 *
 * Save asks for a name (DashboardNameDialog, as the JSON page), starting from
 * the one the model suggested with its latest reply (a reply without one
 * keeps the one before), and stores the source with it (`usePutCodeDashboard`) under the page's own draft id, then
 * opens the saved dashboard (`savedCodeDashboardPath`). A refused save (a name
 * the user already has, the limit, code too large) shows in that dialog,
 * which stays open.
 *
 * Editing a saved advanced dashboard (`initialDashboard`, from
 * EditCodeDashboardPage) starts with its source and name, under its id, so
 * Save replaces it in place, still offering the saved name (renaming is in
 * My Dashboards' switcher); Back and Discard return to it. Until a message
 * changes the source there is nothing to save or lose.
 *
 * Back names its destination because this page is only ever opened from My
 * Dashboards (the webapp CLAUDE.md's fixed-destination exception).
 */
export default function GenerateCodeDashboardPage({
  initialDashboard,
}: {
  /** The saved advanced dashboard being edited; absent for a new one. */
  initialDashboard?: SavedCodeDashboard;
} = {}): JSX.Element {
  const navigate = useNavigate();
  const editing = initialDashboard !== undefined;
  const [messages, setMessages] = useState<GenerateChatMessage[]>([]);
  const [chatOpen, setChatOpen] = useState(true);
  const [source, setSource] = useState<string | null>(initialDashboard?.source ?? null);
  const [busy, setBusy] = useState(false);
  const [confirmDiscardOpen, setConfirmDiscardOpen] = useState(false);
  const generate = useGenerateCodeDashboard();
  // The id the dashboard is saved under: the edited dashboard's, or a new one for this page.
  const [draftId] = useState(() => initialDashboard?.id ?? crypto.randomUUID());
  // The saved source and name: the edited dashboard's; null for a new one.
  const saved = initialDashboard ? { source: initialDashboard.source, name: initialDashboard.name } : null;
  // The name the model suggested last; Save's dialog starts from it for a new dashboard.
  const [suggestedName, setSuggestedName] = useState<string | null>(null);
  const [nameOpen, setNameOpen] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);
  const save = usePutCodeDashboard();
  const { showSuccess } = useSuccessBanner();

  const savedUnchanged = saved !== null && saved.source === source;
  const hasWork = saved === null ? source !== null || messages.length > 0 : !savedUnchanged;
  const leave = (): void => {
    void navigate(editing ? savedCodeDashboardPath(draftId) : MY_DASHBOARDS_PATH);
  };
  const requestLeave = (): void => {
    if (hasWork) setConfirmDiscardOpen(true);
    else leave();
  };

  /**
   * Saves the source under the name from the name dialog. A failure keeps the
   * dialog open with the error (a name already taken says so).
   */
  const handleSaveAs = async (name: string): Promise<void> => {
    if (source === null) return;
    setNameError(null);
    try {
      await save.mutateAsync({ id: draftId, name, source });
      setNameOpen(false);
      showSuccess(editing ? `"${name}" was updated.` : `"${name}" was saved to My Dashboards.`);
      void navigate(savedCodeDashboardPath(draftId));
    } catch (err) {
      setNameError(saveErrorMessage(err, name));
    }
  };

  const addMessage = (role: GenerateChatMessage["role"], text: string): void => {
    setMessages((prev) => [...prev, { id: crypto.randomUUID(), role, text }]);
  };

  const handleSend = async (text: string): Promise<void> => {
    addMessage("user", text);
    setBusy(true);
    try {
      const result = await generate(text, source);
      setSource(result.source);
      if (result.name) setSuggestedName(result.name);
      addMessage("assistant", result.reply);
    } catch (err) {
      // The source is left as it was, so a failed turn never loses the draft.
      // The reply says what went wrong, as on the JSON page (a decline's
      // reason, the usage limit, a timeout...).
      addMessage("assistant", generateErrorMessage(err, source !== null));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={requestLeave}
        sx={{ alignSelf: "flex-start" }}
      >
        Back to My Dashboards
      </Button>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Typography variant="h5">{editing ? "Edit Dashboard (Advanced)" : "New Dashboard (Advanced)"}</Typography>
        </Box>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          {!chatOpen && (
            <Button
              variant="outlined"
              size="small"
              startIcon={<MessageSquare size={16} />}
              onClick={() => setChatOpen(true)}
            >
              Open chat
            </Button>
          )}
          <Button variant="outlined" size="small" onClick={requestLeave}>
            Discard
          </Button>
          <Button
            variant="contained"
            color="primary"
            size="small"
            disabled={source === null || busy || save.isPending || savedUnchanged}
            onClick={() => {
              setNameError(null);
              setNameOpen(true);
            }}
          >
            {save.isPending ? "Saving…" : "Save"}
          </Button>
        </Box>
      </Box>

      {/* A bounded height, so the chat's message list scrolls inside the panel. */}
      <Box sx={{ display: "flex", gap: 2, alignItems: "stretch", height: "calc(100dvh - 240px)", minHeight: 520 }}>
        {/* Left: the generated dashboard, or a placeholder until the first reply. */}
        {source !== null ? (
          <Box
            sx={{
              flex: 1,
              minWidth: 0,
              border: 1,
              borderColor: "divider",
              borderRadius: 1,
              p: 2,
              display: "flex",
              flexDirection: "column",
              gap: 1,
              minHeight: 0,
            }}
          >
            <CodeDashboardFrame key={source} source={source} />
          </Box>
        ) : (
          <Box
            sx={{
              flex: 1,
              minWidth: 0,
              border: 1,
              borderColor: "divider",
              borderRadius: 1,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              gap: 1,
              p: 3,
              textAlign: "center",
            }}
          >
            <Code size={32} />
            <Typography variant="h6">The generated dashboard will appear here</Typography>
            <Typography variant="body2" color="text.secondary">
              Describe the dashboard you want in the chat. It runs here with your real data.
            </Typography>
          </Box>
        )}

        {/* Right: the chat with the model. */}
        {chatOpen && (
          <Box
            sx={{
              width: 400,
              flexShrink: 0,
              border: 1,
              borderColor: "divider",
              borderRadius: 1,
              overflow: "hidden",
            }}
          >
            <GenerateChatPanel
              messages={messages}
              onSend={(text) => void handleSend(text)}
              onClose={() => setChatOpen(false)}
              busy={busy}
            />
          </Box>
        )}
      </Box>

      <DashboardNameDialog
        open={nameOpen}
        title="Save dashboard"
        initialName={saved?.name ?? suggestedName ?? ""}
        confirmLabel="Save"
        busyLabel="Saving…"
        busy={save.isPending}
        error={nameError}
        onCancel={() => setNameOpen(false)}
        onConfirm={(name) => void handleSaveAs(name)}
      />
      <DiscardGeneratedDashboardDialog
        open={confirmDiscardOpen}
        onCancel={() => setConfirmDiscardOpen(false)}
        onConfirm={leave}
      />
    </Box>
  );
}
