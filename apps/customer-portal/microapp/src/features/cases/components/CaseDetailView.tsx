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

import { useLayoutEffect } from "react";
import { User, Users } from "@wso2/oxygen-ui-icons-react";
import { Card, Grid, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { CommentSkeleton, InfoField, MenuOptions, OverlineSlot, StickyCommentBar } from "@components/detail";
import { PriorityChip, StatusChip } from "@components/support";
import { AttachmentPreviewDialog, RichText, SectionCard } from "@components/common";
import { useLayout } from "@context/layout";
import { Comment } from "@components/detail";
import DOMPurify from "dompurify";
import { useDateTime } from "@shared/hooks/useDateTime";
import { useOverlineVariant } from "@shared/hooks/useOverlineVariant";
import { AttachmentCard } from "@features/cases/components/AttachmentCard";
import type { Case, Attachment } from "@features/cases/types/case.model";
import type { MenuOptionProps } from "@components/detail";
import type { useDetailComments } from "@shared/hooks/useDetailComments";
import type { useAttachmentPreview } from "@features/cases/hooks/useAttachmentPreview";

type CaseDetailViewProps = {
  data: Case | undefined;
  isLoading: boolean;
  attachments: Attachment[] | undefined;
  isAttachmentsLoading: boolean;
  isFiltersLoading: boolean;
  issueType: string | undefined;
  menuOptions: MenuOptionProps[];
  comments: ReturnType<typeof useDetailComments>;
  attachmentPreview: ReturnType<typeof useAttachmentPreview>;
};

export function CaseDetailView({
  data,
  isLoading,
  attachments,
  isAttachmentsLoading,
  isFiltersLoading,
  issueType,
  menuOptions,
  comments,
  attachmentPreview,
}: CaseDetailViewProps) {
  const layout = useLayout();
  const { format } = useDateTime();
  const { ref, variant: overlineSlotVariant } = useOverlineVariant();
  const { comments: commentList, comment, setComment, handleSend, isSendingComment, bottomRef } = comments;
  const { previewAttachment, previewSrc, open: handlePreviewOpen, close: handlePreviewClose } = attachmentPreview;

  useLayoutEffect(() => {
    layout.setLayoutOverrides({
      title: (
        <OverlineSlot
          variant={overlineSlotVariant}
          type="case"
          id={data?.number ? `${data.internalId} | ${data.number}` : undefined}
          title={data?.title}
        />
      ),
    });
    return () => {
      layout.setLayoutOverrides({ title: undefined });
    };
  }, [data, overlineSlotVariant]);

  useLayoutEffect(() => {
    layout.setLayoutOverrides({
      endSlot: <MenuOptions disabled={!data || menuOptions.every((option) => option.hidden)} options={menuOptions} />,
    });
    return () => {
      layout.setLayoutOverrides({ endSlot: undefined });
    };
  }, [data, menuOptions]);

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
              <InfoField label="Last Updated" value={data?.updatedOn && format(data.updatedOn)} />
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
            {commentList ? (
              <>
                {commentList.map(({ id, content, createdOn, createdBy }) => (
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
