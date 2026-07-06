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
  Box,
  Button,
  Card,
  Chip,
  Skeleton,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, Check, Pencil, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type ReactNode, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useGetChangeRequest } from "@features/csm-operations/api/useGetChangeRequest";
import { usePatchChangeRequest } from "@features/csm-operations/api/usePatchChangeRequest";
import EditChangeRequestDialog from "@features/csm-operations/components/EditChangeRequestDialog";
import {
  changeRequestImpactColor,
  changeRequestImpactLabel,
  changeRequestStateColor,
  changeRequestStateLabel,
} from "@features/csm-operations/utils/changeRequests";
import type { BeEntityRef } from "@api/backend/types";

const OPERATIONS_CR_PATH = "/operations?tab=change_requests";

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      dateStyle: "medium",
      timeStyle: "short",
    }) ?? "—"
  );
}

function MetaCell({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

function RefText({ value }: { value?: BeEntityRef | null }): JSX.Element {
  return <Typography variant="body2">{value?.name || "—"}</Typography>;
}

function YesNo({ value }: { value?: boolean }): JSX.Element {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
      {value ? <Check size={14} /> : <X size={14} />}
      <Typography variant="body2">{value ? "Yes" : "No"}</Typography>
    </Box>
  );
}

/**
 * A long-form plan section. The value is ServiceNow rich-text HTML, so it's
 * sanitized and rendered as HTML. Renders nothing when the field is empty or
 * has no visible content.
 */
function PlanSection({ title, html }: { title: string; html?: string | null }): JSX.Element | null {
  if (!html || isBlankHtml(html)) return null;
  const safeHtml = sanitizeRichTextHtml(html);
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
      <Typography variant="subtitle2">{title}</Typography>
      <Box
        sx={{
          color: "text.secondary",
          fontSize: "0.875rem",
          lineHeight: 1.5,
          wordBreak: "break-word",
          "& p": { my: 0.5 },
          "& p:first-of-type": { mt: 0 },
          "& p:last-child": { mb: 0 },
          "& ul, & ol": { my: 0.5, pl: 3 },
          "& a": { color: "primary.main" },
          "& img": { maxWidth: "100%", height: "auto" },
          "& table": { borderCollapse: "collapse", width: "100%" },
          "& th, & td": { border: 1, borderColor: "divider", px: 1, py: 0.5, textAlign: "left" },
        }}
        dangerouslySetInnerHTML={{ __html: safeHtml }}
      />
    </Box>
  );
}

/**
 * Read-only detail for a single change request (`GET /change-requests/{id}`):
 * its references, the change window, approval state, and the implementation /
 * rollback / test / communication plans.
 */
export default function CsmChangeRequestDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data, isLoading, isError } = useGetChangeRequest(id);
  const { showError } = useErrorBanner();
  const patchCr = usePatchChangeRequest();
  const [editOpen, setEditOpen] = useState(false);

  const back = (): void => {
    navigate(OPERATIONS_CR_PATH);
  };

  const BackButton = (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={back}
      sx={{ alignSelf: "flex-start" }}
    >
      Back to change requests
    </Button>
  );

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rectangular" height={32} width={240} />
        <Skeleton variant="rectangular" height={260} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="body1" color="error">
          Could not load change request {id}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="h5">Change request not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No change request with id <code>{id}</code>.
        </Typography>
      </Box>
    );
  }

  const cr = data;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      {BackButton}

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1, minWidth: 0 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="h5">{cr.subject || cr.number || "Change request"}</Typography>
          {cr.state && (
            <Chip
              size="small"
              color={changeRequestStateColor(cr.state)}
              label={changeRequestStateLabel(cr.state)}
            />
          )}
          {cr.impact && (
            <Chip
              size="small"
              variant="outlined"
              color={changeRequestImpactColor(cr.impact)}
              label={`${changeRequestImpactLabel(cr.impact)} impact`}
            />
          )}
          <Button
            variant="outlined"
            size="small"
            startIcon={<Pencil size={14} />}
            onClick={() => setEditOpen(true)}
            sx={{ ml: "auto", flexShrink: 0 }}
          >
            Edit
          </Button>
        </Box>
        <Typography variant="body2" color="text.secondary" sx={{ fontFamily: "monospace" }}>
          {cr.number || cr.id}
        </Typography>
      </Box>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Overview</Typography>
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, minmax(0, 1fr))",
              md: "repeat(3, minmax(0, 1fr))",
            },
          }}
        >
          <MetaCell label="Project"><RefText value={cr.project} /></MetaCell>
          <MetaCell label="Type">
            <Typography variant="body2">{cr.type || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Linked case"><RefText value={cr.case} /></MetaCell>
          <MetaCell label="Deployment"><RefText value={cr.deployment} /></MetaCell>
          <MetaCell label="Deployed product"><RefText value={cr.deployedProduct} /></MetaCell>
          <MetaCell label="Product"><RefText value={cr.product} /></MetaCell>
          <MetaCell label="Assigned engineer"><RefText value={cr.assignedEngineer} /></MetaCell>
          <MetaCell label="Assigned team"><RefText value={cr.assignedTeam} /></MetaCell>
          <MetaCell label="Duration">
            <Typography variant="body2">{cr.duration || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Planned start">
            <Typography variant="body2">{formatDateTime(cr.plannedStartOn)}</Typography>
          </MetaCell>
          <MetaCell label="Planned end">
            <Typography variant="body2">{formatDateTime(cr.plannedEndOn)}</Typography>
          </MetaCell>
          <MetaCell label="Created">
            <Typography variant="body2">{formatDateTime(cr.createdOn)}</Typography>
          </MetaCell>
          <MetaCell label="Last updated">
            <Typography variant="body2">{formatDateTime(cr.updatedOn)}</Typography>
          </MetaCell>
          <MetaCell label="Created by">
            <Typography variant="body2">{cr.createdBy || "—"}</Typography>
          </MetaCell>
        </Box>
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Approval</Typography>
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, minmax(0, 1fr))",
              md: "repeat(3, minmax(0, 1fr))",
            },
          }}
        >
          <MetaCell label="Customer approved"><YesNo value={cr.hasCustomerApproved} /></MetaCell>
          <MetaCell label="Customer reviewed"><YesNo value={cr.hasCustomerReviewed} /></MetaCell>
          <MetaCell label="Approved by"><RefText value={cr.approvedBy} /></MetaCell>
          <MetaCell label="Approved on">
            <Typography variant="body2">{formatDateTime(cr.approvedOn)}</Typography>
          </MetaCell>
        </Box>
      </Card>

      {[
        cr.description,
        cr.justification,
        cr.impactDescription,
        cr.serviceOutage,
        cr.communicationPlan,
        cr.rollbackPlan,
        cr.testPlan,
      ].some((v) => v && !isBlankHtml(v)) && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2.5 }}>
          <Typography variant="subtitle2">Details &amp; plans</Typography>
          <PlanSection title="Description" html={cr.description} />
          <PlanSection title="Justification" html={cr.justification} />
          <PlanSection title="Impact description" html={cr.impactDescription} />
          <PlanSection title="Service outage" html={cr.serviceOutage} />
          <PlanSection title="Communication plan" html={cr.communicationPlan} />
          <PlanSection title="Rollback plan" html={cr.rollbackPlan} />
          <PlanSection title="Test plan" html={cr.testPlan} />
        </Card>
      )}

      {editOpen && (
        <EditChangeRequestDialog
          cr={cr}
          isSaving={patchCr.isPending}
          onClose={() => {
            if (!patchCr.isPending) setEditOpen(false);
          }}
          onSave={(patch) =>
            patchCr.mutate(
              { id: cr.id, patch },
              {
                onSuccess: () => setEditOpen(false),
                onError: (err) =>
                  showError("Could not update the change request.", err),
              },
            )
          }
        />
      )}
    </Box>
  );
}
