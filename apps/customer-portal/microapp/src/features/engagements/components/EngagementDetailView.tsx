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
import { Grid, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { User, Users } from "@wso2/oxygen-ui-icons-react";
import { Comment, CommentSkeleton, InfoField, OverlineSlot, StickyCommentBar } from "@components/detail";
import { CallRequestCard, PriorityChip, StatusChip } from "@components/support";
import { useLayout } from "@context/layout";
import { RichText, SectionCard } from "@components/common";
import EmptyState from "@components/common/EmptyState";
import DOMPurify from "dompurify";
import { useDateTime } from "@shared/hooks/useDateTime";
import { useOverlineVariant } from "@shared/hooks/useOverlineVariant";
import { stripHtmlTags } from "@shared/utils/string.utils";
import type { Case } from "@features/cases/types/case.model";
import type { CallRequestsDto } from "@features/engagements/types/engagement.dto";
import type { useDetailComments } from "@shared/hooks/useDetailComments";

type EngagementDetailViewProps = {
  data: Case | undefined;
  isLoading: boolean;
  calls: CallRequestsDto | undefined;
  comments: ReturnType<typeof useDetailComments>;
};

export function EngagementDetailView({ data, isLoading, calls, comments }: EngagementDetailViewProps) {
  const layout = useLayout();
  const { fromNow, format } = useDateTime();
  const { ref, variant: overlineSlotVariant } = useOverlineVariant();
  const { comments: commentList, comment, setComment, handleSend, isSendingComment, bottomRef } = comments;

  useLayoutEffect(() => {
    layout.setLayoutOverrides({
      title: (
        <OverlineSlot
          variant={overlineSlotVariant}
          type="engagement"
          id={data?.number ? `${data.internalId} | ${data.number}` : undefined}
          title={data?.title}
        />
      ),
    });
    return () => {
      layout.setLayoutOverrides({ title: undefined });
    };
  }, [data, overlineSlotVariant]);

  return (
    <>
      <Stack gap={2} mb={10}>
        <Typography ref={ref} variant="h5" fontWeight="medium">
          {data?.title}
        </Typography>
        <SectionCard title="Request Information">
          <Grid spacing={1.5} container>
            <Grid size={12}>
              <InfoField label="Description" value={data?.description ? stripHtmlTags(data.description) : undefined} />
            </Grid>
            <Grid size={6}>
              <InfoField
                label="Status"
                value={
                  data?.statusId ? (
                    <StatusChip type="service" id={data.statusId} size="small" />
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
              <InfoField label="Requested By" value={isLoading ? undefined : (data?.createdBy ?? "N/A")} icon={User} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Assigned To" value={isLoading ? undefined : (data?.assigned ?? "N/A")} icon={Users} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Created" value={data?.createdOn ? format(data.createdOn) : undefined} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Last Updated" value={data?.updatedOn ? fromNow(data.updatedOn) : undefined} />
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
        <SectionCard title="Call Requests">
          <Stack gap={2}>
            {calls?.callRequests.map((call) => (
              <CallRequestCard {...call} />
            ))}
          </Stack>
        </SectionCard>
        <SectionCard title="Updates">
          <Stack gap={2} pt={1}>
            {!commentList ? (
              Array.from({ length: 5 }).map((_, index) => <CommentSkeleton key={index} />)
            ) : commentList.length === 0 ? (
              <EmptyState />
            ) : (
              <>
                {commentList.map(({ id, content, createdOn, createdBy }) => (
                  <Comment key={id} author={createdBy} timestamp={format(createdOn)}>
                    <RichText dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(content) }} />
                  </Comment>
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
      <div ref={bottomRef} />
    </>
  );
}
