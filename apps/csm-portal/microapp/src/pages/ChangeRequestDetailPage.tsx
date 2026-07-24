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

import { Suspense, useState, type ReactNode } from "react";
import { useParams } from "react-router-dom";
import { Button, Chip, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { useQueryClient, useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { changeRequests } from "@src/services/changeRequests";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { ErrorState } from "@components/support/ErrorState";
import { SectionCard } from "@components/case-detail/SectionCard";
import {
  changeRequestImpactColor,
  changeRequestImpactLabel,
  changeRequestStateColor,
  changeRequestStateLabel,
} from "@components/operations/config";
import { EditChangeRequestDialog } from "@components/operations/EditChangeRequestDialog";
import { formatDate } from "@utils/dateTime";

export default function ChangeRequestDetailPage() {
  const { id } = useParams<{ id: string }>();

  return (
    <Stack gap={2} sx={{ mb: 10 }}>
      <ChangeRequestDetailErrorBoundary>
        <Suspense fallback={<ChangeRequestDetailSkeleton />}>
          <ChangeRequestDetailContent id={id ?? ""} />
        </Suspense>
      </ChangeRequestDetailErrorBoundary>
    </Stack>
  );
}

function ChangeRequestDetailContent({ id }: { id: string }) {
  const queryClient = useQueryClient();
  const { data: cr } = useSuspenseQuery(changeRequests.get(id));
  const [editOpen, setEditOpen] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit: Parameters<typeof EditChangeRequestDialog>[0]["onSubmit"] = (fields) => {
    setIsSubmitting(true);
    setError(null);
    changeRequests
      .patch(id, fields)
      .then(() => {
        setEditOpen(false);
        void queryClient.invalidateQueries({ queryKey: ["change-request", id] });
        void queryClient.invalidateQueries({ queryKey: ["change-requests", "infinite"] });
      })
      .catch(() => setError("Could not save changes. Please try again."))
      .finally(() => setIsSubmitting(false));
  };

  const hasPlans = [
    cr.description,
    cr.justification,
    cr.impactDescription,
    cr.serviceOutage,
    cr.communicationPlan,
    cr.rollbackPlan,
    cr.testPlan,
  ].some((v) => v && v.trim().length > 0);

  return (
    <>
      <Stack gap={1}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={1}>
          <Typography variant="subtitle2" color="text.secondary" noWrap>
            {cr.number}
          </Typography>
          <Button size="small" variant="outlined" onClick={() => setEditOpen(true)}>
            Edit
          </Button>
        </Stack>

        <Typography variant="h6">{cr.subject}</Typography>

        <Stack direction="row" gap={1} flexWrap="wrap">
          {cr.state && (
            <Chip size="small" label={changeRequestStateLabel(cr.state)} color={changeRequestStateColor(cr.state)} />
          )}
          {cr.impact && (
            <Chip
              size="small"
              label={changeRequestImpactLabel(cr.impact)}
              color={changeRequestImpactColor(cr.impact)}
            />
          )}
        </Stack>

        {error && (
          <Typography variant="caption" color="error.main">
            {error}
          </Typography>
        )}
      </Stack>

      <SectionCard title="Overview">
        <Stack gap={1}>
          <DetailRow label="Project" value={cr.project?.name} />
          <DetailRow label="Linked case" value={cr.case?.name} />
          <DetailRow label="Deployment" value={cr.deployment?.name} />
          <DetailRow label="Product" value={cr.product?.name} />
          <DetailRow label="Assigned Engineer" value={cr.assignedEngineer?.name} />
          <DetailRow label="Assigned Team" value={cr.assignedTeam?.name} />
          <DetailRow label="Duration" value={cr.duration} />
          <DetailRow
            label="Planned Start"
            value={cr.plannedStartOn ? formatDate(new Date(cr.plannedStartOn)) : undefined}
          />
          <DetailRow label="Planned End" value={cr.plannedEndOn ? formatDate(new Date(cr.plannedEndOn)) : undefined} />
          <DetailRow label="Created By" value={cr.createdBy} />
          <DetailRow label="Created On" value={formatDate(cr.createdOn)} />
          <DetailRow label="Updated On" value={formatDate(cr.updatedOn)} />
        </Stack>
      </SectionCard>

      <SectionCard title="Approval">
        <Stack gap={1}>
          <DetailRow label="Customer Approved" value={cr.hasCustomerApproved ? "Yes" : "No"} />
          <DetailRow label="Customer Reviewed" value={cr.hasCustomerReviewed ? "Yes" : "No"} />
          <DetailRow label="Approved By" value={cr.approvedBy?.name} />
          <DetailRow label="Approved On" value={cr.approvedOn ? formatDate(cr.approvedOn) : undefined} />
        </Stack>
      </SectionCard>

      {hasPlans && (
        <SectionCard title="Details & plans">
          <Stack gap={1.5}>
            <TextBlock label="Description" value={cr.description} />
            <TextBlock label="Justification" value={cr.justification} />
            <TextBlock label="Impact description" value={cr.impactDescription} />
            <TextBlock label="Service outage" value={cr.serviceOutage} />
            <TextBlock label="Communication plan" value={cr.communicationPlan} />
            <TextBlock label="Rollback plan" value={cr.rollbackPlan} />
            <TextBlock label="Test plan" value={cr.testPlan} />
          </Stack>
        </SectionCard>
      )}

      {editOpen && (
        <EditChangeRequestDialog
          changeRequest={cr}
          isSubmitting={isSubmitting}
          onClose={() => setEditOpen(false)}
          onSubmit={handleSubmit}
        />
      )}
    </>
  );
}

function DetailRow({ label, value }: { label: string; value?: string | null }) {
  if (!value) return null;

  return (
    <Stack direction="row" justifyContent="space-between" gap={2}>
      <Typography variant="body2" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2" color="text.primary" textAlign="right">
        {value}
      </Typography>
    </Stack>
  );
}

function TextBlock({ label, value }: { label: string; value?: string | null }) {
  if (!value || value.trim().length === 0) return null;

  return (
    <Stack gap={0.25}>
      <Typography variant="subtitle2" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2" color="text.primary" sx={{ whiteSpace: "pre-wrap" }}>
        {value}
      </Typography>
    </Stack>
  );
}

function ChangeRequestDetailSkeleton() {
  return (
    <Stack gap={2}>
      <Skeleton variant="text" width="40%" height={24} />
      <Skeleton variant="text" width="80%" height={32} />
      <Skeleton variant="rounded" height={200} />
      <Skeleton variant="rounded" height={120} />
    </Stack>
  );
}

function ChangeRequestDetailErrorBoundary({ children }: { children: ReactNode }) {
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
    >
      {children}
    </ErrorBoundary>
  );
}
