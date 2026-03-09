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

import { useState, useMemo, type JSX, type ReactElement } from "react";
import {
  Box,
  Button,
  Chip,
  Divider,
  Paper,
  Stack,
  Typography,
  Avatar,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import {
  ArrowLeft,
  Calendar,
  Folder,
  Globe,
  Layers,
  Loader2,
  MessageSquare,
  Package,
  Send,
  User,
} from "@wso2/oxygen-ui-icons-react";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { usePatchCase } from "@api/usePatchCase";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import useGetCaseComments from "@api/useGetCaseComments";
import useGetUserDetails from "@api/useGetUserDetails";
import { usePostComment } from "@api/usePostComment";
import type { CaseDetails } from "@models/responses";
import type { CaseComment } from "@models/responses";
import {
  formatDateTime,
  formatRelativeTime,
  getAssignedEngineerLabel,
  getInitials,
  getStatusColor,
  getStatusIconElement,
  mapSeverityToDisplay,
  resolveColorFromTheme,
  stripHtml,
  hasSingleCodeWrapper,
  stripCodeWrapper,
  stripAllCodeBlocks,
  convertCodeTagsToHtml,
  trimLeadingBr,
  stripCustomerCommentAddedLabel,
  replaceInlineImageSources,
  hasDisplayableContent,
  ACTION_TO_CASE_STATE_LABEL,
  getAvailableCaseActions,
  toPresentContinuousActionLabel,
  toPresentTenseActionLabel,
} from "@utils/support";
import { CASE_STATUS_ACTIONS, CommentType } from "@constants/supportConstants";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import CaseDetailsAttachmentsPanel from "@case-details-attachments/CaseDetailsAttachmentsPanel";
import Editor from "@components/common/rich-text-editor/Editor";
import DOMPurify from "dompurify";

export interface ServiceRequestDetailContentProps {
  data: CaseDetails | undefined;
  isLoading: boolean;
  isError: boolean;
  caseId: string;
  projectId: string | undefined;
  onBack: () => void;
}

interface RequestDetailSection {
  label: string;
  value: string;
}

const WSO2_PRODUCT_LABEL_REGEX = /^\s*wso2\s*product\s*:?\s*/i;

/** Names of context fields to hide from Request Details (same as create flow). */
const REQUEST_DETAILS_HIDDEN_VARIABLE_NAMES =
  /^(project|deployments?|product|wso2\s*product|environment)$/i;

function isPlainTextComment(content: string): boolean {
  const trimmed = (content ?? "").trim();
  if (!trimmed) return true;
  const hasHtmlTags = trimmed.includes("<") || trimmed.includes(">");
  const isFullCodeBlock =
    trimmed.startsWith("[code]") && trimmed.endsWith("[/code]");
  const hasInlineImageRef = /\[img:\d+\]/.test(trimmed);
  return !hasHtmlTags && !isFullCodeBlock && !hasInlineImageRef;
}

function stripWso2ProductFromText(text: string): string {
  if (!text?.trim()) return text ?? "";
  return text
    .split(/\r?\n/)
    .filter((line) => !WSO2_PRODUCT_LABEL_REGEX.test(line.trim()))
    .join("\n")
    .trim();
}

function parseRequestDetails(
  descriptionHtml: string | null | undefined,
): RequestDetailSection[] {
  if (!descriptionHtml) {
    return [];
  }

  if (typeof document === "undefined") {
    return [{ label: "Details", value: descriptionHtml }];
  }

  const sections: RequestDetailSection[] = [];
  const strongRegex = /<strong[^>]*>([\s\S]*?)<\/strong>/gi;
  let match: RegExpExecArray | null;

  while ((match = strongRegex.exec(descriptionHtml)) !== null) {
    const label = (match[1] ?? "").replace(/<[^>]+>/g, "").trim().replace(/:$/, "");
    const valueStart = match.index + match[0].length;
    const nextStrong = descriptionHtml.slice(valueStart).search(/<strong[^>]*>/i);
    const valueEnd =
      nextStrong >= 0 ? valueStart + nextStrong : descriptionHtml.length;
    const valueHtml = descriptionHtml.slice(valueStart, valueEnd).trim();
    if (label && valueHtml) {
      sections.push({ label, value: valueHtml });
    }
  }

  if (sections.length === 0 && descriptionHtml.trim()) {
    sections.push({ label: "Details", value: descriptionHtml.trim() });
  }

  return sections;
}

export default function ServiceRequestDetailContent({
  data,
  isLoading,
  isError,
  caseId,
  projectId,
  onBack,
}: ServiceRequestDetailContentProps): JSX.Element {
  const theme = useTheme();
  const { data: userDetails } = useGetUserDetails();
  const currentUserEmail = userDetails?.email?.toLowerCase() ?? "";
  const [commentText, setCommentText] = useState("");
  const [commentResetTrigger, setCommentResetTrigger] = useState(0);

  const {
    data: commentsData,
    isLoading: isCommentsLoading,
    isError: isCommentsError,
  } = useGetCaseComments(projectId ?? "", caseId, {
    offset: 0,
    limit: 50,
  });
  const postComment = usePostComment();

  const { data: filterMetadata } = useGetProjectFilters(projectId ?? "");
  const caseStates = filterMetadata?.caseStates;
  const patchCase = usePatchCase(projectId ?? "", caseId);
  const { showSuccess } = useSuccessBanner();
  const { showError } = useErrorBanner();
  const [pendingActionLabel, setPendingActionLabel] = useState<string | null>(
    null,
  );

  const statusLabel = data?.status?.label;
  const severityLabel = data?.severity?.label;
  const statusColorPath = getStatusColor(statusLabel ?? undefined);
  const resolvedStatusColor = resolveColorFromTheme(statusColorPath, theme);
  const statusChipIcon = getStatusIconElement(
    statusLabel,
    12,
  ) as ReactElement | undefined;

  const assignedLabel = getAssignedEngineerLabel(data?.assignedEngineer);
  const environmentLabel = data?.deployment?.label ?? null;
  const productLabel = data?.deployedProduct?.label
    ? data.deployedProduct.version
      ? `${data.deployedProduct.label} ${data.deployedProduct.version}`
      : data.deployedProduct.label
    : null;

  const raw: Record<string, unknown> =
    ((data as unknown as Record<string, unknown>) ?? {}) || {};
  const srMeta =
    (raw.serviceRequestDetails as Record<string, unknown> | undefined) ??
    (raw.serviceRequest as Record<string, unknown> | undefined) ??
    raw;

  const requestedBy: string | null =
    (srMeta.requestedBy as string | undefined) ??
    (srMeta.requester as string | undefined) ??
    (srMeta.requestedFor as string | undefined) ??
    (srMeta.openedBy as string | undefined) ??
    (srMeta.createdBy as string | undefined) ??
    null;

  const requestDetailSections = useMemo(
    () => parseRequestDetails(data?.description),
    [data?.description],
  );

  const commentsSorted = useMemo(() => {
    const list = commentsData?.comments ?? [];
    return [...list].sort(
      (a, b) =>
        new Date(a.createdOn).getTime() - new Date(b.createdOn).getTime(),
    );
  }, [commentsData?.comments]);

  const commentsToShow = useMemo(
    () => commentsSorted.filter(hasDisplayableContent),
    [commentsSorted],
  );

  const timelineEntries = useMemo(() => {
    const entries: {
      type: string;
      label: string;
      date: string;
      actor: string;
      desc: string;
    }[] = [];

    if (data?.createdOn) {
      entries.push({
        type: "created",
        label: "Service Request Created",
        date: data.createdOn,
        actor: requestedBy ?? data?.engineerEmail ?? "System",
        desc: "Initial service request submitted",
      });
    }

    commentsToShow.forEach((c) => {
      entries.push({
        type: "comment",
        label: "Comment Added",
        date: c.createdOn,
        actor: c.createdBy ?? "Unknown",
        desc: "",
      });
    });

    if (data?.closedOn) {
      const closedByLabel =
        data?.closedBy?.label ?? data?.closedBy?.name ?? "System";
      entries.push({
        type: "status_changed",
        label: "Status Changed",
        date: data.closedOn,
        actor: closedByLabel,
        desc: `Status changed to ${statusLabel ?? "Closed"}`,
      });
    }

    return entries.sort(
      (a, b) => new Date(a.date).getTime() - new Date(b.date).getTime(),
    );
  }, [data, statusLabel, requestedBy, commentsToShow]);

  const handleAddComment = () => {
    const content = commentText.trim();
    if (!content || !stripHtml(content).trim()) return;
    postComment.mutate(
      {
        caseId,
        body: { content, type: CommentType.COMMENT },
      },
      {
        onSuccess: () => {
          setCommentText("");
          setCommentResetTrigger((prev) => prev + 1);
        },
        onError: (err) => {
          showError(
            err?.message ?? "Failed to add comment. Please try again.",
          );
        },
      },
    );
  };

  if (isLoading) {
    return (
      <Box>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={onBack}
          sx={{ mb: 2 }}
          variant="text"
        >
          Back to Service Requests
        </Button>
        <Paper variant="outlined" sx={{ p: 3 }}>
          <Stack spacing={2}>
            <Box sx={{ height: 24, width: 120 }} />
            <Box sx={{ height: 32, width: "60%" }} />
            <Box sx={{ height: 56, width: "100%" }} />
            <Stack direction="row" spacing={3} sx={{ pt: 2 }}>
              {[1, 2, 3, 4].map((i) => (
                <Box key={i} sx={{ height: 40, width: 140 }} />
              ))}
            </Stack>
          </Stack>
        </Paper>
      </Box>
    );
  }

  if (isError) {
    return (
      <Box>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={onBack}
          sx={{ mb: 2 }}
          variant="text"
        >
          Back to Service Requests
        </Button>
        <ErrorIndicator entityName="service request details" size="medium" />
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Button
        startIcon={<ArrowLeft size={16} />}
        onClick={onBack}
        sx={{ alignSelf: "flex-start", mb: 0.5 }}
        variant="text"
      >
        Back to Service Requests
      </Button>

      <Paper variant="outlined" sx={{ p: 2, borderRadius: 0 }}>
        <Stack spacing={1.5}>
          <Stack
            direction="row"
            spacing={1.5}
            alignItems="center"
            sx={{ flexWrap: "wrap" }}
          >
            <Typography variant="body2" fontWeight={600} color="text.primary">
              {data?.number ?? "--"}
            </Typography>
            <Chip
              size="small"
              variant="outlined"
              label={statusLabel ?? "--"}
              icon={statusChipIcon}
              sx={{
                bgcolor: alpha(resolvedStatusColor, 0.1),
                color: resolvedStatusColor,
                height: 22,
                fontSize: "0.75rem",
                "& .MuiChip-icon": { color: "inherit", ml: "6px", mr: "6px" },
                "& .MuiChip-label": { pl: 0, pr: "6px" },
              }}
            />
            <Typography variant="body2" color="text.secondary">
              Priority: {mapSeverityToDisplay(severityLabel ?? undefined)}
            </Typography>
          </Stack>
          <Typography variant="h6" color="text.primary" fontWeight={500}>
            {data?.title ?? "--"}
          </Typography>

          <Box
            sx={{
              mt: 1.5,
              display: "grid",
              gridTemplateColumns: { xs: "1fr", md: "repeat(4, minmax(0, 1fr))" },
              gap: 3,
            }}
          >
            <Box>
              <Stack spacing={0.5}>
                <Stack direction="row" alignItems="center" spacing={1}>
                  <Box color="text.secondary">
                    <Layers size={16} />
                  </Box>
                  <Typography variant="caption" color="text.secondary">
                    Environment
                  </Typography>
                </Stack>
                <Typography variant="body2" color="text.primary">
                  {environmentLabel ?? "--"}
                </Typography>
              </Stack>
            </Box>
            <Box>
              <Stack spacing={0.5}>
                <Stack direction="row" alignItems="center" spacing={1}>
                  <Box color="text.secondary">
                    <Package size={16} />
                  </Box>
                  <Typography variant="caption" color="text.secondary">
                    Product
                  </Typography>
                </Stack>
                <Typography variant="body2" color="text.primary">
                  {productLabel ?? "--"}
                </Typography>
              </Stack>
            </Box>
            <Box>
              <Stack spacing={0.5}>
                <Stack direction="row" alignItems="center" spacing={1}>
                  <Box color="text.secondary">
                    <User size={16} />
                  </Box>
                  <Typography variant="caption" color="text.secondary">
                    Requested By
                  </Typography>
                </Stack>
                <Typography variant="body2" color="text.primary">
                  {requestedBy ?? "--"}
                </Typography>
              </Stack>
            </Box>
            <Box>
              <Stack spacing={0.5}>
                <Stack direction="row" alignItems="center" spacing={1}>
                  <Box color="text.secondary">
                    <Calendar size={16} />
                  </Box>
                  <Typography variant="caption" color="text.secondary">
                    Requested On
                  </Typography>
                </Stack>
                <Typography variant="body2" color="text.primary">
                  {data?.createdOn
                    ? formatDateTime(data.createdOn, "long") ?? "--"
                    : "--"}
                </Typography>
              </Stack>
            </Box>
          </Box>
        </Stack>
      </Paper>

      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "1fr 360px" },
          gap: 3,
          alignItems: "start",
        }}
      >
        <Stack spacing={3}>
          <Paper variant="outlined" sx={{ p: 2, borderRadius: 0 }}>
            <Typography variant="subtitle2" color="text.primary" sx={{ mb: 1.5 }}>
              Request Details
            </Typography>
            {(() => {
              const apiVariables = data?.variables ?? [];
              const filteredVariables = apiVariables.filter(
                (v) => !REQUEST_DETAILS_HIDDEN_VARIABLE_NAMES.test((v.name ?? "").trim()),
              );
              if (filteredVariables.length > 0) {
                return filteredVariables.map((v, index) => {
                  const rawValue = (v.value ?? "").trim();
                  const hasHtml = rawValue.includes("<") || rawValue.includes(">");
                  const processedValue = hasHtml
                    ? DOMPurify.sanitize(
                        convertCodeTagsToHtml(
                          stripCustomerCommentAddedLabel(rawValue),
                        ),
                      )
                    : "";
                  return (
                    <Box key={`${v.name}-${index}`} sx={{ mb: 1.5 }}>
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{ display: "block", mb: 0.5 }}
                      >
                        {v.name}
                      </Typography>
                      {hasHtml ? (
                        // biome-ignore security/noDangerouslySetInnerHtml: sanitized with DOMPurify
                        <Box
                          component="div"
                          sx={{
                            "& p": { mb: 0.5 },
                            "& p:last-child": { mb: 0 },
                            "& code": {
                              display: "block",
                              p: 1,
                              bgcolor: "action.hover",
                              fontSize: "0.875rem",
                              whiteSpace: "pre-wrap",
                              overflowWrap: "break-word",
                            },
                          }}
                          dangerouslySetInnerHTML={{
                            __html: processedValue || "--",
                          }}
                        />
                      ) : (
                        <Typography
                          variant="body2"
                          color="text.primary"
                          sx={{ whiteSpace: "pre-wrap" }}
                        >
                          {rawValue || "--"}
                        </Typography>
                      )}
                      {index < filteredVariables.length - 1 && (
                        <Divider sx={{ mt: 1.5 }} />
                      )}
                    </Box>
                  );
                });
              }
              const filtered = requestDetailSections.filter(
                (s) => !REQUEST_DETAILS_HIDDEN_VARIABLE_NAMES.test(s.label.trim()),
              );
              if (filtered.length > 0) {
                return filtered.map((section, index) => {
                  const processedHtml = DOMPurify.sanitize(
                    convertCodeTagsToHtml(
                      stripCustomerCommentAddedLabel(section.value),
                    ),
                  );
                  return (
                    <Box key={`${section.label}-${index}`} sx={{ mb: 1.5 }}>
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{ display: "block", mb: 0.5 }}
                      >
                        {section.label}
                      </Typography>
                      {/* biome-ignore security/noDangerouslySetInnerHtml: sanitized with DOMPurify */}
                      <Box
                        component="div"
                        sx={{
                          "& p": { mb: 0.5 },
                          "& p:last-child": { mb: 0 },
                          "& code": {
                            display: "block",
                            p: 1,
                            bgcolor: "action.hover",
                            fontSize: "0.875rem",
                            whiteSpace: "pre-wrap",
                            overflowWrap: "break-word",
                          },
                        }}
                        dangerouslySetInnerHTML={{ __html: processedHtml }}
                      />
                      {index < filtered.length - 1 && (
                        <Divider sx={{ mt: 1.5 }} />
                      )}
                    </Box>
                  );
                });
              }
              const fallbackHtml = stripWso2ProductFromText(
                data?.description ?? "",
              );
              const processedFallback = fallbackHtml
                ? DOMPurify.sanitize(
                    convertCodeTagsToHtml(
                      stripCustomerCommentAddedLabel(fallbackHtml),
                    ),
                  )
                : "";
              return (
                // biome-ignore security/noDangerouslySetInnerHtml: sanitized with DOMPurify
                <Box
                  component="div"
                  sx={{
                    "& p": { mb: 0.5 },
                    "& p:last-child": { mb: 0 },
                    "& code": {
                      display: "block",
                      p: 1,
                      bgcolor: "action.hover",
                      fontSize: "0.875rem",
                      whiteSpace: "pre-wrap",
                      overflowWrap: "break-word",
                    },
                  }}
                  dangerouslySetInnerHTML={{
                    __html: processedFallback || "--",
                  }}
                />
              );
            })()}
          </Paper>

          <Paper variant="outlined" sx={{ p: 2, borderRadius: 0 }}>
            <Typography
              variant="subtitle2"
              color="text.primary"
              sx={{ mb: 1.5 }}
            >
              Attachments
            </Typography>
            <CaseDetailsAttachmentsPanel caseId={caseId} />
          </Paper>

          <Paper variant="outlined" sx={{ p: 2, borderRadius: 0 }}>
            <Typography
              variant="subtitle2"
              color="text.primary"
              sx={{ mb: 1.5, display: "flex", alignItems: "center", gap: 0.5 }}
            >
              <MessageSquare size={18} />
              Communication
            </Typography>
            <Stack spacing={2} sx={{ mb: 2 }}>
              {isCommentsLoading ? (
                <Typography variant="body2" color="text.secondary">
                  Loading comments...
                </Typography>
              ) : isCommentsError ? (
                <Typography variant="body2" color="error.main">
                  Could not load comments. Please try again.
                </Typography>
              ) : commentsToShow.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  No comments yet.
                </Typography>
              ) : (
                commentsToShow.map((comment: CaseComment) => {
                  const isCurrentUser =
                    (comment.createdBy?.toLowerCase() ?? "") === currentUserEmail;
                  const avatarBg = isCurrentUser
                    ? alpha(theme.palette.info?.light ?? theme.palette.info?.main, 0.2)
                    : alpha(theme.palette.primary?.light ?? theme.palette.primary?.main, 0.2);
                  const avatarColor = isCurrentUser
                    ? (theme.palette.info?.main ?? theme.palette.info?.light)
                    : (theme.palette.primary?.main ?? theme.palette.primary?.light);
                  return (
                    <Stack
                      key={comment.id}
                      direction="row"
                      spacing={1.5}
                      alignItems="flex-start"
                    >
                      <Avatar
                        sx={{
                          width: 32,
                          height: 32,
                          fontSize: "0.75rem",
                          bgcolor: avatarBg,
                          color: avatarColor,
                          flexShrink: 0,
                        }}
                      >
                        {getInitials(comment.createdBy)}
                      </Avatar>
                      <Box sx={{ flex: 1, minWidth: 0 }}>
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          sx={{ display: "block" }}
                        >
                          {comment.createdBy} • {formatRelativeTime(comment.createdOn)}
                        </Typography>
                        {isPlainTextComment(comment.content ?? "") ? (
                          <Box
                            component="div"
                            sx={{
                              mt: 0.5,
                              whiteSpace: "pre-wrap",
                              wordBreak: "break-word",
                            }}
                          >
                            {(comment.content ?? "").trim() || ""}
                          </Box>
                        ) : (
                          // biome-ignore security/noDangerouslySetInnerHtml: sanitized with DOMPurify
                          <Box
                            component="div"
                            sx={{
                              mt: 0.5,
                              "& p": { mb: 0.5 },
                              "& p:last-child": { mb: 0 },
                              "& code": {
                                display: "block",
                                p: 1,
                                bgcolor: "action.hover",
                                fontSize: "0.875rem",
                                whiteSpace: "pre-wrap",
                                overflowWrap: "break-word",
                              },
                            }}
                            dangerouslySetInnerHTML={{
                              __html: (() => {
                                const raw = comment.content ?? "";
                                const isFullCodeWrap =
                                  hasSingleCodeWrapper(raw);
                                const codeBlockCount =
                                  raw.match(/\[code\]/gi)?.length ?? 0;
                                const afterCode = isFullCodeWrap
                                  ? stripCodeWrapper(raw)
                                  : codeBlockCount > 1
                                    ? stripAllCodeBlocks(raw)
                                    : convertCodeTagsToHtml(raw);
                                const trimmedBr = trimLeadingBr(afterCode);
                                const withoutLabel =
                                  stripCustomerCommentAddedLabel(trimmedBr);
                                const withImages = replaceInlineImageSources(
                                  withoutLabel,
                                  comment.inlineAttachments,
                                );
                                return DOMPurify.sanitize(withImages);
                              })(),
                            }}
                          />
                        )}
                      </Box>
                    </Stack>
                  );
                })
              )}
            </Stack>
            <Stack spacing={1.5}>
              <Editor
                value={commentText}
                onChange={setCommentText}
                disabled={postComment.isPending}
                resetTrigger={commentResetTrigger}
                minHeight={100}
                showToolbar
                placeholder="Add a comment..."
                onSubmitKeyDown={handleAddComment}
              />
              <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
                <Button
                  size="small"
                  variant="contained"
                  color="primary"
                  startIcon={<Send size={16} />}
                  onClick={handleAddComment}
                  disabled={
                    !stripHtml(commentText).trim() || postComment.isPending
                  }
                  sx={{ textTransform: "none" }}
                >
                  Add Comment
                </Button>
              </Box>
            </Stack>
          </Paper>
        </Stack>

        <Stack spacing={3}>
          <Paper variant="outlined" sx={{ p: 2, borderRadius: 0 }}>
            <Typography variant="subtitle2" color="text.primary" sx={{ mb: 1.5 }}>
              Assignment
            </Typography>
            <Stack spacing={2}>
              <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5 }}>
                <Box
                  sx={{
                    width: 40,
                    height: 40,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    flexShrink: 0,
                    bgcolor: alpha(
                      theme.palette.primary?.main,
                      0.12,
                    ),
                  }}
                >
                  <User
                    size={20}
                    color={theme.palette.primary?.main}
                  />
                </Box>
                <Box sx={{ minWidth: 0 }}>
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    display="block"
                  >
                    Assigned To
                  </Typography>
                  <Typography variant="body2" fontWeight={600} color="text.primary">
                    {assignedLabel ?? "--"}
                  </Typography>
                </Box>
              </Box>
              <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5 }}>
                <Box
                  sx={{
                    width: 40,
                    height: 40,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    flexShrink: 0,
                    bgcolor: alpha(
                      theme.palette.info?.main,
                      0.12,
                    ),
                  }}
                >
                  <Folder
                    size={20}
                    color={theme.palette.info?.main}
                  />
                </Box>
                <Box sx={{ minWidth: 0 }}>
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    display="block"
                  >
                    Category
                  </Typography>
                  <Typography variant="body2" fontWeight={600} color="text.primary">
                    {data?.catalog?.label ?? "--"}
                  </Typography>
                </Box>
              </Box>
            </Stack>
          </Paper>

          {(() => {
            const displayableActions = CASE_STATUS_ACTIONS.filter((action) =>
              getAvailableCaseActions(statusLabel).includes(action.label),
            ).filter((action) => action.label !== "Open Related Case");
            const isClosed =
              statusLabel?.toLowerCase() === "closed";
            if (isClosed || displayableActions.length === 0) return null;
            return (
          <Paper variant="outlined" sx={{ p: 2, borderRadius: 0 }}>
            <Typography variant="subtitle2" color="text.primary" sx={{ mb: 1.5 }}>
              Manage service request status
            </Typography>
            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
              {displayableActions
                .map(({ label, Icon }) => {
                  const stateLabel = ACTION_TO_CASE_STATE_LABEL[label];
                  const stateKeyEntry = caseStates?.find(
                    (s) =>
                      s.label.toLowerCase() ===
                      (stateLabel ?? "").toLowerCase(),
                  );
                  const stateKey =
                    stateKeyEntry && !Number.isNaN(Number(stateKeyEntry.id))
                      ? Number(stateKeyEntry.id)
                      : undefined;
                  const canPatch = stateKey != null && !!caseId;
                  const isThisPending =
                    patchCase.isPending && pendingActionLabel === label;

                  return (
                    <Button
                      key={label}
                      variant="outlined"
                      size="small"
                      startIcon={
                        isThisPending ? (
                          <Loader2 size={12} />
                        ) : (
                          <Icon size={12} />
                        )
                      }
                      disabled={patchCase.isPending || !canPatch}
                      onClick={
                        canPatch
                          ? () => {
                              setPendingActionLabel(label);
                              patchCase.mutate(
                                { stateKey },
                                {
                                  onSuccess: () => {
                                    showSuccess(
                                      "Service request status updated successfully.",
                                    );
                                  },
                                  onError: (err) => {
                                    showError(
                                      err?.message ??
                                        "Failed to update service request status. Please try again.",
                                    );
                                  },
                                  onSettled: () => {
                                    setPendingActionLabel(null);
                                  },
                                },
                              );
                            }
                          : undefined
                      }
                      sx={{
                        fontSize: "0.7rem",
                        minHeight: 0,
                        py: 0.5,
                        px: 1,
                        textTransform: "none",
                      }}
                    >
                      {isThisPending
                        ? toPresentContinuousActionLabel(label)
                        : toPresentTenseActionLabel(label)}
                    </Button>
                  );
                })}
            </Stack>
          </Paper>
            );
          })()}

          <Paper variant="outlined" sx={{ p: 2, borderRadius: 0 }}>
            <Typography variant="subtitle2" color="text.primary" sx={{ mb: 1.5 }}>
              Activity Timeline
            </Typography>
            <Box
              sx={{
                position: "relative",
                pl: 3,
                "&::before": {
                  content: '""',
                  position: "absolute",
                  left: 35,
                  top: 14,
                  bottom: 14,
                  width: 2,
                  bgcolor: "grey.400",
                },
              }}
            >
              {timelineEntries.map((entry, idx) => (
                <Stack
                  key={`${entry.type}-${entry.date}-${idx}`}
                  direction="row"
                  spacing={1.5}
                  alignItems="flex-start"
                  sx={{
                    position: "relative",
                    pb: idx < timelineEntries.length - 1 ? 2 : 0,
                  }}
                >
                  <Box
                    sx={{
                      width: 24,
                      height: 24,
                      borderRadius: "50%",
                      boxShadow: `0 0 0 2px ${theme.palette.primary?.main}`,
                      bgcolor: "background.paper",
                      flexShrink: 0,
                      zIndex: 1,
                      mt: 0.25,
                    }}
                  />
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Typography
                      variant="body2"
                      fontWeight={600}
                      color="text.primary"
                    >
                      {entry.label}
                    </Typography>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      display="block"
                      sx={{ mt: 0.25 }}
                    >
                      {entry.actor} - {formatRelativeTime(entry.date)}
                    </Typography>
                    {entry.desc && (
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        display="block"
                        sx={{ mt: 0.25 }}
                      >
                        {entry.desc}
                      </Typography>
                    )}
                  </Box>
                </Stack>
              ))}
            </Box>
          </Paper>

          <Paper variant="outlined" sx={{ p: 2, borderRadius: 0 }}>
            <Typography variant="subtitle2" color="text.primary" sx={{ mb: 1.5 }}>
              Related Information
            </Typography>
            <Stack spacing={1.5}>
              {data?.project?.label && (
                <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Box color="text.secondary">
                      <Globe size={16} />
                    </Box>
                    <Typography variant="caption" color="text.secondary">
                      Project
                    </Typography>
                  </Stack>
                  <Typography variant="body2" color="text.primary">
                    {data.project.label}
                  </Typography>
                </Box>
              )}
              <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
                <Stack direction="row" alignItems="center" spacing={1}>
                  <Box color="text.secondary">
                    <Calendar size={16} />
                  </Box>
                  <Typography variant="caption" color="text.secondary">
                    Last Updated
                  </Typography>
                </Stack>
                <Typography variant="body2" color="text.primary">
                  {data?.updatedOn
                    ? formatDateTime(data.updatedOn, "long") ?? "--"
                    : "--"}
                </Typography>
              </Box>
            </Stack>
          </Paper>
        </Stack>
      </Box>
    </Box>
  );
}
