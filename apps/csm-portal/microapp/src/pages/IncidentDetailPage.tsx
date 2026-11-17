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

import { Suspense, type ReactNode } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Chip, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { incidents } from "@src/services/incidents";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { ErrorState } from "@components/support/ErrorState";
import { SectionCard } from "@components/case-detail/SectionCard";
import { CaseActivityFeed } from "@components/case-detail/CaseActivityFeed";
import {
  incidentPriorityColor,
  incidentPriorityLabel,
  incidentStateColor,
  incidentStateLabel,
} from "@components/operations/incidentConfig";
import { formatDate } from "@utils/dateTime";
import { stripStaffNameMarker } from "@utils/staffNameMarker";

// Read-only counterpart of the webapp's CsmIncidentDetailPage.tsx (no Edit dialog, no comment
// composer, no attachments — see the scope note on OperationsPage.tsx). Layout follows this app's
// own ChangeRequestDetailPage.tsx: a stack of SectionCards with DetailRow/TextBlock fields rather
// than the webapp's responsive MetaCell grid.
export default function IncidentDetailPage() {
  const { id } = useParams<{ id: string }>();

  return (
    <Stack gap={2} sx={{ mb: 10 }}>
      <IncidentDetailErrorBoundary>
        <Suspense fallback={<IncidentDetailSkeleton />}>
          <IncidentDetailContent id={id ?? ""} />
        </Suspense>
      </IncidentDetailErrorBoundary>
    </Stack>
  );
}

function IncidentDetailContent({ id }: { id: string }) {
  const { data: incident } = useSuspenseQuery(incidents.get(id));

  const hasLinks = !!(incident.changeRequest || incident.problem || incident.causedBy);
  const hasNotes = !!(incident.additionalComments || incident.workNotes);

  return (
    <>
      <Stack gap={1}>
        <Typography variant="subtitle2" color="text.secondary" noWrap>
          {incident.number}
        </Typography>

        <Typography variant="h6">{incident.subject}</Typography>

        <Stack direction="row" gap={1} flexWrap="wrap">
          {incident.state && (
            <Chip size="small" label={incidentStateLabel(incident.state)} color={incidentStateColor(incident.state)} />
          )}
          {incident.priority && (
            <Chip
              size="small"
              variant="outlined"
              label={incidentPriorityLabel(incident.priority)}
              color={incidentPriorityColor(incident.priority)}
            />
          )}
        </Stack>
      </Stack>

      <SectionCard title="Overview">
        <Stack gap={1}>
          <DetailRow label="Caller" value={stripStaffNameMarker(incident.caller?.name) || undefined} />
          <DetailRow label="Category" value={incident.category} />
          <DetailRow label="Subcategory" value={incident.subcategory} />
          <DetailRow label="Channel" value={incident.channel} />
          <DetailRow label="Impact" value={incident.impact} />
          <DetailRow label="Urgency" value={incident.urgency} />
          <DetailRow label="Service" value={incident.service?.name} />
          <DetailRow label="Service Offering" value={incident.serviceOffering?.name} />
          <DetailRow label="Configuration Item" value={incident.configurationItem?.name} />
          <DetailRow label="Assignment Group" value={incident.assignmentGroup?.name} />
          <DetailRow label="Assigned To" value={stripStaffNameMarker(incident.assignedTo?.name) || undefined} />
          <DetailRow label="Opened" value={incident.openedOn ? formatDate(incident.openedOn) : undefined} />
          <DetailRow label="Created By" value={incident.createdBy} />
          <DetailRow label="Created On" value={incident.createdOn ? formatDate(incident.createdOn) : undefined} />
          <DetailRow label="Updated On" value={incident.updatedOn ? formatDate(incident.updatedOn) : undefined} />
        </Stack>
      </SectionCard>

      {hasLinks && (
        <SectionCard title="Linked records">
          <Stack gap={1}>
            <DetailRow label="Change Request" value={incident.changeRequest?.name} />
            <DetailRow label="Problem" value={incident.problem?.name} />
            {/* "Caused by" has no confirmed target record type (could be a change request, a
                problem, or something else), same caveat the webapp's own detail page notes — left
                as plain text rather than guessing a route. */}
            <DetailRow label="Caused By" value={stripStaffNameMarker(incident.causedBy?.name) || undefined} />
          </Stack>
        </SectionCard>
      )}

      {hasNotes && (
        <SectionCard title="Comments & notes">
          <Stack gap={1.5}>
            <TextBlock label="Additional comments (customer-visible)" value={incident.additionalComments} />
            <TextBlock label="Internal work notes" value={incident.workNotes} />
          </Stack>
        </SectionCard>
      )}

      {incident.watchList.length > 0 && (
        <SectionCard title="Watch list">
          <Stack direction="row" gap={1} flexWrap="wrap">
            {incident.watchList.map((w) => (
              <Chip key={w.id} size="small" variant="outlined" label={stripStaffNameMarker(w.name) || w.email} />
            ))}
          </Stack>
        </SectionCard>
      )}

      {incident.linkedServiceRequests.length > 0 && <LinkedServiceRequests items={incident.linkedServiceRequests} />}

      <SectionCard title="Comments">
        <IncidentCommentsSection incidentId={id} />
      </SectionCard>
    </>
  );
}

function LinkedServiceRequests({ items }: { items: { id: string; number: string; name: string }[] }) {
  const navigate = useNavigate();

  return (
    <SectionCard title="Linked service requests">
      <Stack direction="row" gap={1} flexWrap="wrap">
        {items.map((sr) => (
          <Chip
            key={sr.id}
            size="small"
            variant="outlined"
            clickable
            label={`${sr.number} — ${sr.name}`}
            onClick={() => navigate(`/cases/${sr.id}`)}
            sx={{ fontWeight: 600 }}
          />
        ))}
      </Stack>
    </SectionCard>
  );
}

// Own Suspense/error boundary, mirroring CaseActivityFeed.tsx's own CaseActivitiesTab: comments
// load independently of the rest of the detail page instead of blocking it.
function IncidentCommentsSection({ incidentId }: { incidentId: string }) {
  return (
    <IncidentCommentsErrorBoundary>
      <Suspense fallback={<Skeleton variant="rounded" height={56} />}>
        <IncidentCommentsContent incidentId={incidentId} />
      </Suspense>
    </IncidentCommentsErrorBoundary>
  );
}

function IncidentCommentsContent({ incidentId }: { incidentId: string }) {
  const { data: comments } = useSuspenseQuery(incidents.comments(incidentId));
  return <CaseActivityFeed comments={comments} audit={[]} attachments={[]} />;
}

function IncidentCommentsErrorBoundary({ children }: { children: ReactNode }) {
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

function IncidentDetailSkeleton() {
  return (
    <Stack gap={2}>
      <Skeleton variant="text" width="40%" height={24} />
      <Skeleton variant="text" width="80%" height={32} />
      <Skeleton variant="rounded" height={200} />
      <Skeleton variant="rounded" height={120} />
    </Stack>
  );
}

function IncidentDetailErrorBoundary({ children }: { children: ReactNode }) {
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
