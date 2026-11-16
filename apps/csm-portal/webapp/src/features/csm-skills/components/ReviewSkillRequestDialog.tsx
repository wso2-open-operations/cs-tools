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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  FormLabel,
  Radio,
  RadioGroup,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import RelativeTime from "@components/RelativeTime";
import {
  useSkill,
  useSkillCategories,
  useSkillLevelTypes,
  useSkillPickerSearch,
} from "@features/csm-skills/api/useSkillCatalogue";
import {
  useApproveSkillRequest,
  useCheckSkillRequest,
  useRejectSkillRequest,
} from "@features/csm-skills/api/useSkillManagement";
import { CategorySelect } from "@features/csm-skills/components/CategorySelect";
import LevelChip from "@features/csm-skills/components/LevelChip";
import { fileableCategoryIds } from "@features/csm-skills/utils/categoryTree";
import { defaultLevelType, findLevelType, sortedLevels } from "@features/csm-skills/utils/levels";
import { personName } from "@features/csm-skills/utils/skillRequests";
import type { ApproveMode, ApproveSkillRequestRequest, Skill, SkillRequest } from "@features/csm-skills/types";

type Decision = ApproveMode | "reject";

interface ReviewSkillRequestDialogProps {
  request: SkillRequest;
  onClose: (notice?: string) => void;
}

const getSkillId = (s: Skill): string => s.id;
const getSkillLabel = (s: Skill): string => s.name;

/**
 * Review one skill request. The reviewer sees what the catalogue already has
 * (an exact match, similar names) and decides: create the skill, make an
 * inactive one available again, point at one that already covers it, or
 * reject with a reason the requester sees. Approving also adds the skill to
 * the requester's profile, at the level they asked for unless changed here.
 */
export default function ReviewSkillRequestDialog({ request, onClose }: ReviewSkillRequestDialogProps): JSX.Element {
  const check = useCheckSkillRequest(request.requestedName);
  const categories = useSkillCategories();
  const levelTypes = useSkillLevelTypes();
  const approve = useApproveSkillRequest();
  const reject = useRejectSkillRequest();

  const exact = check.data?.exactMatch ?? null;
  const similar = useMemo(() => check.data?.similar ?? [], [check.data]);
  const inactiveCandidates = useMemo(
    () => [...(exact && !exact.active ? [exact] : []), ...similar.filter((s) => !s.active)],
    [exact, similar],
  );
  const suggested: Decision = exact ? (exact.active ? "existing" : "reactivate") : "create";

  const [chosen, setDecision] = useState<Decision | null>(null);
  const decision = chosen ?? (check.isLoading ? "create" : suggested);
  const [name, setName] = useState(request.requestedName);
  const [categoryId, setCategoryId] = useState(request.category?.id ?? "");
  const [description, setDescription] = useState(request.description ?? "");
  const [keywords, setKeywords] = useState("");
  const [existingId, setExistingId] = useState<string | null>(null);
  const [reactivateId, setReactivateId] = useState<string | null>(null);
  const [levelId, setLevelId] = useState("");
  const [comment, setComment] = useState("");
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);

  const existingSkillId = existingId ?? (exact?.active ? exact.id : "");
  const reactivateSkillId = reactivateId ?? inactiveCandidates[0]?.id ?? "";
  const targetSkillId = decision === "existing" ? existingSkillId : decision === "reactivate" ? reactivateSkillId : "";
  const target = useSkill(targetSkillId || undefined);

  // The scale the requester's level is on once approved.
  const scale =
    decision === "create"
      ? defaultLevelType(levelTypes.data)
      : findLevelType(levelTypes.data, target.data?.levelTypeId);
  const levels = sortedLevels(scale);
  const fileable = fileableCategoryIds(categories.data);
  const busy = approve.isPending || reject.isPending;

  const ready =
    decision === "reject"
      ? reason.trim().length > 0
      : decision === "create"
        ? name.trim().length > 0 && !!categoryId
        : !!targetSkillId;

  const fail = (e: Error): void => setError(e.message || "The request could not be decided.");

  const submit = (): void => {
    setError(null);
    if (decision === "reject") {
      reject.mutate(
        { id: request.id, reason: reason.trim() },
        { onSuccess: () => onClose(`The request for ${request.requestedName} was rejected.`), onError: fail },
      );
      return;
    }
    const body: ApproveSkillRequestRequest = {
      mode: decision,
      ...(levelId ? { levelId } : {}),
      ...(comment.trim() ? { comment: comment.trim() } : {}),
    };
    if (decision === "create") {
      body.name = name.trim();
      body.categoryId = categoryId;
      if (description.trim()) body.description = description.trim();
      if (keywords.trim()) body.keywords = keywords.trim();
    } else {
      body.skillId = targetSkillId;
      if (decision === "reactivate" && categoryId && categoryId !== request.category?.id) body.categoryId = categoryId;
    }
    approve.mutate(
      { id: request.id, body },
      {
        onSuccess: (r) =>
          onClose(
            `Approved: ${r.resolvedSkill?.name ?? request.requestedName}` +
              (r.autoAddedLevel ? `, added to ${personName(request.requestedBy)}'s skills at ${r.autoAddedLevel.name}.` : "."),
          ),
        onError: fail,
      },
    );
  };

  return (
    <Dialog open onClose={busy ? undefined : () => onClose()} maxWidth="md" fullWidth aria-labelledby="review-request-title">
      <DialogTitle id="review-request-title">Review request: {request.requestedName}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          {error && <Alert severity="error">{error}</Alert>}

          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))" }, gap: 1.5, bgcolor: "action.hover", borderRadius: 1, p: 1.5 }}>
            <Box>
              <Typography variant="caption" color="text.secondary">Requested by</Typography>
              <Typography variant="body2">
                {personName(request.requestedBy)} · <RelativeTime iso={request.createdOn} />
              </Typography>
            </Box>
            <Box>
              <Typography variant="caption" color="text.secondary">Their level</Typography>
              <Box>
                <LevelChip level={request.requestedLevel} />
              </Box>
            </Box>
            <Box>
              <Typography variant="caption" color="text.secondary">Category</Typography>
              <Typography variant="body2">
                {request.category?.categoryPath ?? request.category?.name ?? "Not chosen"}
                {request.suggestedCategory ? ` (suggested: “${request.suggestedCategory}”)` : ""}
              </Typography>
            </Box>
            <Box>
              <Typography variant="caption" color="text.secondary">Description</Typography>
              <Typography variant="body2">{request.description || "None given"}</Typography>
            </Box>
          </Box>

          {(exact || similar.length > 0) && (
            <Box>
              <Typography variant="subtitle2">Already in the catalogue</Typography>
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.75, mt: 0.5 }}>
                {exact && (
                  <Chip
                    size="small"
                    color={exact.active ? "warning" : "default"}
                    label={`Same name: ${exact.name}${exact.active ? "" : " (inactive)"}`}
                  />
                )}
                {similar.map((s) => (
                  <Chip key={s.id} size="small" variant="outlined" label={`${s.name}${s.active ? "" : " (inactive)"}`} />
                ))}
              </Box>
            </Box>
          )}

          <FormControl>
            <FormLabel id="review-decision-label">Decision</FormLabel>
            <RadioGroup
              aria-labelledby="review-decision-label"
              value={decision}
              onChange={(e) => {
                setDecision(e.target.value as Decision);
                setLevelId("");
              }}
            >
              <FormControlLabel value="create" control={<Radio />} label="Create a new skill" disabled={!!exact?.active} />
              <FormControlLabel
                value="reactivate"
                control={<Radio />}
                label="Make an inactive skill available again"
                disabled={inactiveCandidates.length === 0}
              />
              <FormControlLabel value="existing" control={<Radio />} label="It already exists: use an existing skill" />
              <FormControlLabel value="reject" control={<Radio />} label="Reject" />
            </RadioGroup>
          </FormControl>

          {decision === "create" && (
            <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))" } }}>
              <TextField size="small" label="Skill name" required value={name} onChange={(e) => setName(e.target.value)} inputProps={{ maxLength: 255 }} />
              <CategorySelect
                id="review-category"
                label="Category"
                categories={categories.data}
                loading={categories.isLoading}
                value={categoryId}
                onChange={setCategoryId}
                selectableIds={fileable}
                helperText="Required. An active category with no sub-categories."
              />
              <TextField
                size="small"
                label="Description"
                multiline
                minRows={2}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
              <TextField
                size="small"
                label="Keywords"
                value={keywords}
                onChange={(e) => setKeywords(e.target.value)}
                helperText="Extra words the skill can be found by."
              />
            </Box>
          )}

          {decision === "reactivate" && (
            <TextField
              select
              size="small"
              label="Inactive skill"
              value={reactivateSkillId}
              onChange={(e) => setReactivateId(e.target.value)}
              slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
            >
              {inactiveCandidates.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </TextField>
          )}

          {decision === "existing" && (
            <AsyncEntitySelect<Skill>
              id="review-existing-skill"
              label="Existing skill"
              placeholder="Search skills…"
              value={existingSkillId}
              knownLabel={existingId ? undefined : exact?.active ? exact.name : undefined}
              onChange={(id) => setExistingId(id)}
              useSearch={useSkillPickerSearch}
              getId={getSkillId}
              getLabel={getSkillLabel}
              required
            />
          )}

          {decision !== "reject" && (
            <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))" } }}>
              <TextField
                select
                size="small"
                label={`Level for ${personName(request.requestedBy)}`}
                value={levelId}
                onChange={(e) => setLevelId(e.target.value)}
                disabled={levels.length === 0}
                slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
                helperText="The skill is added to their profile at this level."
              >
                <option value="">As requested{request.requestedLevel ? ` (${request.requestedLevel.name})` : ""}</option>
                {levels.map((l) => (
                  <option key={l.id} value={l.id}>
                    {l.name}
                  </option>
                ))}
              </TextField>
              <TextField
                size="small"
                label="Note to the requester"
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                helperText="Optional."
              />
            </Box>
          )}

          {decision === "reject" && (
            <TextField
              size="small"
              label="Reason"
              required
              multiline
              minRows={2}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              helperText="The requester sees this."
              inputProps={{ maxLength: 2000 }}
            />
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => onClose()} disabled={busy}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color={decision === "reject" ? "error" : "primary"}
          onClick={submit}
          disabled={!ready || busy}
        >
          {decision === "reject" ? "Reject request" : "Approve"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
