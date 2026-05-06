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

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { CheckIcon, CircleX, Download, Image, Paperclip, PlusIcon, User, Users } from "@wso2/oxygen-ui-icons-react";
import { Box, Card, Grid, IconButton, Skeleton, Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import {
  CommentSkeleton,
  InfoField,
  MenuOptions,
  OverlineSlot,
  StickyCommentBar,
  type MenuOptionProps,
} from "@components/features/detail";
import { PriorityChip, StatusChip } from "@components/features/support";
import { AttachmentPreviewDialog, RichText, SectionCard } from "@components/shared";
import { useLayout } from "@context/layout";
import { cases } from "@src/services/cases";
import { CASE_STATE_IDS } from "@src/config/constants";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { Comment } from "@components/features/detail";
import { useFilters } from "../context/filters";
import DOMPurify from "dompurify";
import { useNotify } from "../context/snackbar";
import type { Attachment } from "@src/types";
import { useDateTime } from "../utils/useDateTime";

export default function CaseDetailPage() {
  const notify = useNotify();
  const layout = useLayout();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { fromNow, format } = useDateTime();
  const [comment, setComment] = useState("");

  const { id } = useParams();
  const { data, isLoading } = useQuery(cases.get(id!));
  const { data: attachments, isLoading: isAttachmentsLoading } = useQuery({
    ...cases.attachments(id!),
    enabled: Boolean(id),
  });
  const { data: filters, isLoading: isFiltersLoading } = useFilters();
  const { data: comments, isFetching: isCommentsRefetching } = useQuery({
    ...cases.comments(id!),
    select: (data) => [...data].sort((a, b) => a.createdOn.getTime() - b.createdOn.getTime()),
  });

  const issueType = filters?.issueTypes.find((issueType) => issueType.id === data?.issueTypeId)?.label;

  const mutation = useMutation({
    ...cases.createComment(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["comments", id] });
      setComment("");
    },
  });

  const isSendingComment = mutation.status !== "idle" && mutation.isPending && isCommentsRefetching;

  const handleSend = () => {
    if (!comment.trim()) return;

    mutation.mutate({
      content: comment,
      type: "comments",
    });
  };

  const ref = useRef<HTMLSpanElement>(null);
  const [overlineSlotVariant, setOverlineSlotVariant] = useState<"normal" | "shrunk">("normal");

  const editCaseMutation = useMutation({
    ...cases.edit(id!),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: cases.get(id!).queryKey }),
    onError: () => notify.error("Failed to update case. Please try again."),
  });

  const closedStateKey = CASE_STATE_IDS.CLOSED;
  const waitingOnWso2StateKey = CASE_STATE_IDS.WAITING_ON_WSO2;
  const solutionProposedStateKey = CASE_STATE_IDS.SOLUTION_PROPOSED;
  const awaitingInfoStateKey = CASE_STATE_IDS.AWAITING_INFO;

  const menuOptions = data
    ? getCaseMenuOptions(data.statusId, {
        stateKeys: {
          closed: closedStateKey,
          waitingOnWso2: waitingOnWso2StateKey,
          solutionProposed: solutionProposedStateKey,
          awaitingInfo: awaitingInfoStateKey,
        },
        onResolve: () => {
          if (closedStateKey === undefined) return;
          editCaseMutation.mutate({ stateKey: closedStateKey });
        },
        onAcceptSolution: () => {
          if (closedStateKey === undefined) return;
          editCaseMutation.mutate({ stateKey: closedStateKey });
        },
        onRejectSolution: () => {
          if (waitingOnWso2StateKey === undefined) return;
          editCaseMutation.mutate({ stateKey: waitingOnWso2StateKey });
        },
        onCreateRelated: () => navigate("/create", { state: { case: data } }),
      })
    : [];

  useEffect(() => {
    const element = ref.current;
    if (!element) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        const next = entry.isIntersecting ? "normal" : "shrunk";
        setOverlineSlotVariant(next);
      },
      {
        root: null,
        rootMargin: "-80px 0px 0px 0px",
        threshold: 1.0,
      },
    );

    observer.observe(element);

    return () => observer.unobserve(element);
  }, []);

  useLayoutEffect(() => {
    layout.setTitleOverride(
      <OverlineSlot
        variant={overlineSlotVariant}
        type="case"
        id={data?.number ? `${data.internalId} | ${data.number}` : undefined}
        title={data?.title}
      />,
    );

    return () => {
      layout.setTitleOverride(undefined);
    };
  }, [data, overlineSlotVariant]);

  useLayoutEffect(() => {
    layout.setEndSlotOverride(
      <MenuOptions disabled={!data || menuOptions.every((option) => option.hidden)} options={menuOptions} />,
    );

    return () => {
      layout.setEndSlotOverride(undefined);
    };
  }, [data]);

  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [comments]);

  const [previewAttachment, setPreviewAttachment] = useState<Attachment | null>(null);
  const [previewSrc, setPreviewSrc] = useState<string | null>(null);
  const [, setIsPreviewLoading] = useState(false);

  const handlePreviewOpen = async (attachment: Attachment, blob: Blob) => {
    setPreviewAttachment(attachment);
    setIsPreviewLoading(true);
    try {
      const url = URL.createObjectURL(blob);
      setPreviewSrc(url);
    } finally {
      setIsPreviewLoading(false);
    }
  };

  const handlePreviewClose = () => {
    setPreviewAttachment(null);
    setPreviewSrc(null);
  };

  return (
    <>
      <Stack gap={2} mb={10}>
        <Typography ref={ref} variant="h5" fontWeight="medium">
          {data?.title}
        </Typography>
        <SectionCard title="Case Information">
          <Grid spacing={1.5} container>
            <Grid size={6}>
              <InfoField
                label="Status"
                value={
                  data?.statusId ? (
                    <StatusChip type="case" id={data.statusId} size="small" />
                  ) : (
                    <Skeleton variant="text" width={50} height={30} />
                  )
                }
              />
            </Grid>
            <Grid size={6}>
              <InfoField
                label="Priority"
                value={
                  data?.statusId ? (
                    <PriorityChip id={data.severityId} size="small" />
                  ) : (
                    <Skeleton variant="text" width={50} height={30} />
                  )
                }
              />
            </Grid>
            <Grid size={6}>
              <InfoField label="Assignee" value={isLoading ? undefined : (data?.assigned ?? "N/A")} icon={Users} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Reporter" value={isLoading ? undefined : (data?.reporter ?? "N/A")} icon={User} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Category" value={isLoading || isFiltersLoading ? undefined : (issueType ?? "N/A")} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Last Updated" value={data?.updatedOn && fromNow(data.updatedOn)} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Created" value={data?.createdOn ? format(data.createdOn) : undefined} />
            </Grid>
          </Grid>
        </SectionCard>
        <SectionCard title="Product & Environment">
          <Grid spacing={1.5} container>
            <Grid size={12}>
              <InfoField label="Product Name" value={isLoading ? undefined : data?.product || "N/A"} />
            </Grid>
            <Grid size={12}>
              <InfoField label="Deployment" value={isLoading ? undefined : data?.deployment || "N/A"} />
            </Grid>
          </Grid>
        </SectionCard>
        <SectionCard title="Attachments">
          {isAttachmentsLoading ? (
            <Grid spacing={1.5} container>
              {Array.from({ length: 3 }).map((_, index) => (
                <Grid key={index} size={{ xs: 12, sm: 6, md: 4 }}>
                  <Card variant="outlined" sx={{ p: 1.5 }}>
                    <Stack direction="row" gap={1} alignItems="center">
                      <Skeleton variant="rounded" width={40} height={40} />
                      <Stack gap={0.5} flex={1} minWidth={0}>
                        <Skeleton variant="text" width="80%" />
                        <Skeleton variant="text" width="50%" />
                      </Stack>
                      <Skeleton variant="circular" width={32} height={32} />
                    </Stack>
                  </Card>
                </Grid>
              ))}
            </Grid>
          ) : attachments?.length ? (
            <Grid spacing={1.5} container>
              {attachments.map((attachment) => (
                <Grid key={attachment.id} size={{ xs: 12 }}>
                  <AttachmentCard attachment={attachment} onPreview={handlePreviewOpen} />
                </Grid>
              ))}
            </Grid>
          ) : (
            <Typography variant="body2" color="text.secondary">
              No attachments for this case.
            </Typography>
          )}
        </SectionCard>
        <SectionCard title="Activity Timeline">
          <Stack gap={2} pt={1}>
            {comments ? (
              <>
                {comments.map(({ id, content, createdOn, createdBy }) => (
                  <Comment key={id} author={createdBy} timestamp={format(createdOn)}>
                    <RichText dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(content) }} />
                  </Comment>
                ))}
              </>
            ) : (
              <>
                {Array.from({ length: 5 }).map((_, index) => (
                  <CommentSkeleton key={index} />
                ))}
              </>
            )}
          </Stack>
        </SectionCard>
      </Stack>
      <StickyCommentBar
        placeholder="Add Comment"
        value={comment}
        onChange={setComment}
        onSend={handleSend}
        loading={isSendingComment}
      />

      <AttachmentPreviewDialog
        open={Boolean(previewAttachment)}
        attachment={previewAttachment}
        onClose={handlePreviewClose}
        src={previewSrc}
      />

      <div ref={bottomRef} />
    </>
  );
}

function AttachmentCard({
  attachment,
  onPreview,
}: {
  attachment: Attachment;
  onPreview: (attachment: Attachment, blob: Blob) => void;
}) {
  const queryClient = useQueryClient();
  const { fromNow } = useDateTime();

  const handlePreview = async () => {
    const data = await queryClient.fetchQuery(cases.attachment(attachment.id));

    const [prefix, base64] = data.content.split(",");
    const mimeType = prefix.split(":")[1].split(";")[0];

    const byteCharacters = atob(base64);
    const byteArray = new Uint8Array(byteCharacters.length);
    for (let i = 0; i < byteCharacters.length; i++) {
      byteArray[i] = byteCharacters.charCodeAt(i);
    }

    const blob = new Blob([byteArray], { type: mimeType });

    onPreview(attachment, blob);
  };

  return (
    <>
      <Card sx={{ p: 1.5 }}>
        <Stack direction="row" alignItems="flex-start" gap={1}>
          <Box
            sx={{
              flexShrink: 0,
              width: 40,
              height: 40,
              borderRadius: 0.5,
              overflow: "hidden",
              bgcolor: "action.hover",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              color: "text.secondary",
            }}
          >
            {attachment.type === "image" ? <Image size={pxToRem(18)} /> : <Paperclip size={pxToRem(18)} />}
          </Box>
          <Stack gap={0.25} minWidth={0} flex={1}>
            <Typography variant="subtitle2" fontWeight="medium" noWrap>
              {attachment.fileName}
            </Typography>
            <Typography variant="caption" color="text.secondary" noWrap>
              {attachment.createdBy} · {fromNow(attachment.createdOn)}
            </Typography>
          </Stack>
          <IconButton onClick={handlePreview}>
            <Download size={pxToRem(18)} />
          </IconButton>
        </Stack>
      </Card>
    </>
  );
}

function getCaseMenuOptions(
  statusId: string | undefined,
  context: {
    stateKeys: Partial<{
      closed: number;
      waitingOnWso2: number;
      solutionProposed: number;
      awaitingInfo: number;
    }>;
  } & Partial<{
    onResolve: () => void;
    onAcceptSolution: () => void;
    onRejectSolution: () => void;
    onCreateRelated: () => void;
  }>,
): MenuOptionProps[] {
  const { stateKeys, ...actions } = context;
  const currentStatusId = statusId ? Number(statusId) : undefined;
  const isSolutionProposed = currentStatusId === stateKeys.solutionProposed;
  const isClosed = currentStatusId === stateKeys.closed;
  const isAwaitingInfo = currentStatusId === stateKeys.awaitingInfo;
  const hasKnownStatus = currentStatusId !== undefined && !Number.isNaN(currentStatusId);

  return [
    {
      label: isSolutionProposed ? "Accept Solution" : "Mark as Resolved",
      color: "success",
      icon: <CheckIcon />,
      hidden: !hasKnownStatus || isClosed,
      onClick: isSolutionProposed ? actions?.onAcceptSolution : actions?.onResolve,
    },
    {
      label: "Reject Solution",
      color: "error",
      icon: <CircleX />,
      hidden: !isSolutionProposed && !isAwaitingInfo,
      onClick: actions?.onRejectSolution,
    },
    {
      label: "Create Related Case",
      icon: <PlusIcon />,
      hidden: !isClosed,
      onClick: actions?.onCreateRelated,
    },
  ];
}
