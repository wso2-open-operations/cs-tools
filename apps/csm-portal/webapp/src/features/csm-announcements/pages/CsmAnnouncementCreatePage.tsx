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

import { Box, Button, Card, Grid, TextField, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import Editor from "@components/rich-text-editor/Editor";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";
import { usePostCsmCase } from "@features/csm-cases/api/usePostCsmCase";
import { useNavTransition } from "@hooks/useNavTransition";

/** The rich-text editor emits `<p></p>` when empty; check the stripped text. */
function isEmptyHtml(html: string): boolean {
  return html.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length === 0;
}

const BACK_TARGET = "/announcements";

/**
 * Create an announcement (`type: "announcement"`) against one or more
 * projects. Unlike a standard case, this is a broadcast — no
 * severity/issueType/deployment/deployedProduct/attachments, just a subject
 * and description.
 *
 * A single announcement "record" per selected project: the backend's
 * `POST /cases` create call takes exactly one `projectId`, so a
 * multi-project pick here fans out into one independent create call per
 * project (same subject/description on each), not one record with a target
 * list. Submitting is therefore a batch: if some calls fail while others
 * succeed, the succeeded ones stand (no auto-retry) and the failures are
 * reported by project so the engineer can retry just those.
 */
export default function CsmAnnouncementCreatePage(): JSX.Element {
  const navigate = useNavTransition();
  const { showError } = useErrorBanner();

  const [projectIds, setProjectIds] = useState<string[]>([]);
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const postCase = usePostCsmCase();

  const canSubmit = useMemo(
    () =>
      projectIds.length > 0 &&
      subject.trim().length > 0 &&
      !isEmptyHtml(description) &&
      !submitting,
    [projectIds, subject, description, submitting],
  );

  const handleSubmit = async (): Promise<void> => {
    if (!canSubmit) return;
    setSubmitting(true);

    const trimmedSubject = subject.trim();
    const results = await Promise.allSettled(
      projectIds.map((projectId) =>
        postCase.mutateAsync({
          type: "announcement",
          projectId,
          subject: trimmedSubject,
          description,
        }),
      ),
    );
    setSubmitting(false);

    // The multi-select doesn't expose picked project names to this page (it
    // only reports ids via onChange), so a failure is reported by id — still
    // enough for the engineer to identify which project(s) to retry.
    const failedProjectIds = projectIds.filter(
      (_, i) => results[i].status === "rejected",
    );

    if (failedProjectIds.length === 0) {
      navigate(BACK_TARGET);
      return;
    }

    const succeededCount = projectIds.length - failedProjectIds.length;
    if (succeededCount > 0) {
      // Partial failure: the succeeded creates already landed and aren't
      // retried automatically, so navigate away and surface exactly which
      // project(s) still need a retry.
      showError(
        `The announcement was created for ${succeededCount} of ${projectIds.length} project${
          projectIds.length === 1 ? "" : "s"
        }, but failed for project${failedProjectIds.length === 1 ? "" : "s"} ${failedProjectIds.join(
          ", ",
        )}. Create it again for the failed project${failedProjectIds.length === 1 ? "" : "s"} only.`,
      );
      navigate(BACK_TARGET);
    } else {
      showError("Could not create the announcement. Please try again.");
    }
  };

  return (
    <Box sx={{ width: "100%", px: 3, py: 3 }}>
      <Button
        variant="text"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(BACK_TARGET)}
        sx={{ mb: 1 }}
      >
        Back
      </Button>
      <Typography variant="h5" sx={{ mb: 2 }}>
        New announcement
      </Typography>

      <Card variant="outlined" sx={{ p: 3 }}>
        <Grid container spacing={2.5}>
          <Grid size={{ xs: 12 }}>
            <AsyncProjectMultiSelect
              id="announcement-create-projects"
              label="Projects"
              values={projectIds}
              onChange={setProjectIds}
            />
          </Grid>

          <Grid size={{ xs: 12 }}>
            <TextField
              label="Subject"
              size="small"
              fullWidth
              required
              value={subject}
              onChange={(e) => setSubject(e.target.value.slice(0, 200))}
              helperText={
                subject.length >= 160 ? `${subject.length}/200` : undefined
              }
            />
          </Grid>
          <Grid size={{ xs: 12 }}>
            <Typography
              id="announcement-description-label"
              component="label"
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", mb: 0.5 }}
            >
              Description
            </Typography>
            {/* Editor doesn't accept an `id`, so associate the label by wrapping
                the editor in a labelled group for assistive tech. */}
            <Box role="group" aria-labelledby="announcement-description-label">
              <Editor
                value={description}
                onChange={setDescription}
                placeholder="Describe the announcement…"
                minHeight={180}
                maxHeight={420}
                toolbarVariant="full"
                disabled={submitting}
              />
            </Box>
          </Grid>
        </Grid>

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 2.5 }}>
          <Button variant="outlined" onClick={() => navigate(BACK_TARGET)}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={() => void handleSubmit()}
            disabled={!canSubmit}
          >
            {submitting ? "Creating…" : "Create announcement"}
          </Button>
        </Box>
      </Card>
    </Box>
  );
}
