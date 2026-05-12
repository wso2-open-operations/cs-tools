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
import { Chip, Grid, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { InfoField, OverlineSlot, StakeholderItem, StakeholderItemSkeleton } from "@components/features/detail";
import { PriorityChip, StatusChip } from "@components/features/support";
import { User, Users } from "@wso2/oxygen-ui-icons-react";
import { SectionCard } from "@components/shared";
import { useLayout } from "@context/layout";
import { useQuery } from "@tanstack/react-query";
import { changeRequests } from "@src/services/changes";
import { useParams } from "react-router-dom";
import { stripHtmlTags } from "@utils/others";
import { ProgressTimeline } from "../components/features/detail/ProgressTimeline";

export default function ChangeDetailPage() {
  const layout = useLayout();
  const { id } = useParams();
  const { data, isLoading } = useQuery(changeRequests.get(id!));

  const ref = useRef<HTMLSpanElement>(null);
  const [overlineSlotVariant, setOverlineSlotVariant] = useState<"normal" | "shrunk">("normal");

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
        type="change"
        id={data?.number ? `${data.internalId} | ${data.number}` : undefined}
        title={data?.title}
      />,
    );

    return () => {
      layout.setTitleOverride(undefined);
    };
  }, [data, overlineSlotVariant]);

  return (
    <>
      <Stack gap={2} mb={10}>
        <Typography ref={ref} variant="h5" fontWeight="medium">
          {data?.title}
        </Typography>
        <SectionCard title="Change Information">
          <Grid spacing={1.5} container>
            <Grid size={12}>
              <InfoField label="Description" value={data?.description ? stripHtmlTags(data.description) : "N/A"} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Change Owner" value={data?.createdBy} icon={User} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Requested By" value={data?.assignedTeam} icon={Users} />
            </Grid>
            <Grid size={6}>
              <InfoField
                label="Status"
                value={
                  data?.statusId ? (
                    <StatusChip type="change" id={data.statusId} size="small" />
                  ) : (
                    <Skeleton variant="text" width={50} height={30} />
                  )
                }
              />
            </Grid>
            <Grid size={6}>
              <InfoField
                label="Impact"
                value={
                  data ? (
                    <PriorityChip type="change" size="small" id={data?.impactId} />
                  ) : (
                    <Skeleton variant="rounded" width={70} height={22} sx={{ borderRadius: "16px" }} />
                  )
                }
              />
            </Grid>
            <Grid size={6}>
              <InfoField
                label="Request Type"
                value={
                  data?.statusId ? (
                    <Chip label={data.requestType ?? "N/A"} size="small" />
                  ) : (
                    <Skeleton variant="text" width={50} height={30} />
                  )
                }
              />
            </Grid>
            <Grid size={6}>
              <InfoField label="Estimated Duration" value={!isLoading ? (data?.duration ?? "N/A") : undefined} />
            </Grid>
            <Grid size={6}>
              <InfoField
                label="Approval Status"
                value={
                  data ? (
                    <Chip
                      size="small"
                      color={data.hasCustomerApproved ? "success" : "default"}
                      label={data.hasCustomerApproved ? "Approved" : "Pending"}
                    />
                  ) : (
                    <Skeleton variant="rounded" width={70} height={22} sx={{ borderRadius: "16px" }} />
                  )
                }
              />
            </Grid>
            <Grid size={6}>
              <InfoField label="Approved By" value={!isLoading ? (data?.approvedBy ?? "N/A") : undefined} />
            </Grid>
          </Grid>
        </SectionCard>
        <SectionCard title="Impact Assessment">
          <Grid spacing={1.5} container>
            <Grid size={12}>
              <InfoField
                label="Communication Plan"
                value={
                  !isLoading
                    ? data?.communicationPlan
                      ? stripHtmlTags(data.communicationPlan)
                      : "No communication plan available"
                    : undefined
                }
              />
            </Grid>
            <Grid size={12}>
              <InfoField
                label="Rollback Plan"
                value={
                  !isLoading
                    ? data?.rollbackPlan
                      ? stripHtmlTags(data.rollbackPlan)
                      : "No rollback plan available"
                    : undefined
                }
              />
            </Grid>
            <Grid size={12}>
              <InfoField
                label="Test Plan"
                value={
                  !isLoading ? (data?.testPlan ? stripHtmlTags(data.testPlan) : "No test plan available") : undefined
                }
              />
            </Grid>
          </Grid>
        </SectionCard>
        <SectionCard title="Progress Timeline">
          <ProgressTimeline id={id!} />
        </SectionCard>
        <SectionCard title="Stakeholders">
          <Stack gap={1.5}>
            {data ? (
              <>
                {data.createdBy && <StakeholderItem name={data.createdBy} role="owner" />}
                {data.approvedBy && <StakeholderItem name={data.approvedBy} role="approver" />}
                {data.assignedTeam && <StakeholderItem name={data.assignedTeam} role="requestor" />}
              </>
            ) : (
              <>
                {Array.from({ length: 3 }).map((_, index) => (
                  <StakeholderItemSkeleton key={index} />
                ))}
              </>
            )}
          </Stack>
        </SectionCard>
      </Stack>
    </>
  );
}
