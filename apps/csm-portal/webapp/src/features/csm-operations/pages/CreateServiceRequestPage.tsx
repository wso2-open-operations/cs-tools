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

import { CREATE_FORM_MAX_ATTACHMENTS } from "@config/attachmentLimitsConfig";
import {
  Alert,
  Box,
  Button,
  Card,
  CircularProgress,
  FormControl,
  FormHelperText,
  Grid,
  InputLabel,
  MenuItem,
  Select,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useRef, useState, type JSX } from "react";
import { useLocation, useSearchParams } from "react-router";

import { BackendApiError } from "@api/backend/client";
import { priorityFromSeverity } from "@api/backend/mappers";
import type { BeProject } from "@api/backend/types";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import { SEVERITY_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import AttachmentsField from "@components/attachments/AttachmentsField";
import type { PendingAttachment } from "@components/attachments/pendingAttachment";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useNewServiceRequestDraft } from "@context/new-service-request-draft/NewServiceRequestDraftContext";
import ProjectSelectionField from "@features/csm-cases/components/ProjectSelectionField";
import { useSearchDeployments } from "@features/csm-cases/api/useSearchDeployments";
import { useGetProject } from "@features/csm-projects/api/useGetProject";
import { useProjectMetadata } from "@features/csm-projects/api/useProjectMetadata";
import { useDeployedProductOptions } from "@features/csm-cases/api/useDeployedProductOptions";
import { usePostCsmCase } from "@features/csm-cases/api/usePostCsmCase";
import { usePostCsmCaseAttachment } from "@features/csm-cases/api/useCsmCaseAttachments";
import { failedUploadsMessage, uploadAttachmentsToCase } from "@features/csm-cases/api/uploadAttachmentsToCase";
import { useEngineerDisplayName } from "@hooks/useEngineerDisplayName";
import { useSearchCatalogs } from "@features/csm-operations/api/useSearchCatalogs";
import { useCatalogItemVariables } from "@features/csm-operations/api/useCatalogItemVariables";
import CatalogVariableFields from "@features/csm-operations/components/CatalogVariableFields";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import {
  encodeVariableValue,
  getFirstEmptyRequiredField,
  getFirstFieldExceedingMaxLength,
  getFirstFieldFailingValidation,
  getUserEditableVariables,
  isAttachmentField,
} from "@features/csm-operations/utils/catalogVariables";
import { canRaiseServiceRequests } from "@features/csm-projects/utils/subscriptionType";
import type { CreateServiceRequestFromCaseNavState } from "@features/csm-cases/types/csmCases";

// A service request never carries S0, so the picker offers S1-S4 only.
type ServiceRequestSeverity = Exclude<Severity, "S0">;
const SERVICE_REQUEST_SEVERITIES: ServiceRequestSeverity[] = ["S1", "S2", "S3", "S4"];

// Managed-cloud and Cloud Support projects are offered in the project picker
// (see `canRaiseServiceRequests`). Per-project eligibility is still decided
// after selection by `BeProject.hasSr` and
// `features.hasServiceRequestReadAccess`. Module-level (not per-render) so
// `ProjectSelectionField`'s `filterProject` prop keeps a stable identity
// across re-renders.
function isOfferedProject(project: BeProject): boolean {
  return canRaiseServiceRequests(project.subscriptionType);
}

/**
 * Why a deployed product can't be used for a service request, or `null` when
 * it can. Only called when the project restricts service requests to a set of
 * product categories.
 */
function getIneligibleReason(
  category: string | null | undefined,
  allowed: string[],
): string | null {
  const allowedLabel = allowed.map((c) => c.toUpperCase()).join(", ");
  if (category == null || category === "") {
    return "This product has no category set. Set a category on the deployed product to raise a service request.";
  }
  if (allowed.includes(category)) return null;
  return `Service requests can only be raised against products in these categories: ${allowedLabel}. This product's category is ${category.toUpperCase()}.`;
}

export default function CreateServiceRequestPage(): JSX.Element {
  const navigate = useNavTransition();
  const { showError } = useErrorBanner();

  // Set when opened from a case's "Create service request" action (Related
  // tab, Linked service requests card), which navigates here with router
  // state (not query params) so the case's project/deployment/product carry
  // over and the new SR is filed linked to that case in one step. See
  // CsmCaseDetailPage.tsx's "Create service request" button and
  // CreateRelatedCaseNavState's read in CsmCaseCreatePage.tsx for the
  // analogous case-create pattern.
  const location = useLocation();
  const relatedCaseState = location.state as
    | CreateServiceRequestFromCaseNavState
    | undefined;

  // Set when opened from a project's page Create menu
  // (`/operations/service-requests/new?projectId=…`, state: { from:
  // "/customers/projects/:id" }), so Back/Cancel return there instead of the
  // hardcoded operations list, and the newly created service request's own
  // Back button (reading this same convention) returns there too.
  const backState = location.state as { from?: string } | undefined;
  const backTarget = backState?.from ?? "/operations";

  // When opened from a project's page
  // (`/operations/service-requests/new?projectId=…`), the project is fixed
  // and shown read-only, mirroring CsmCaseCreatePage's `?projectId=` lock —
  // the engineer can't accidentally file against the wrong project. Opened
  // without either source (the operations-list entry), the searchable picker
  // is shown.
  const [searchParams] = useSearchParams();
  const lockedProjectId =
    searchParams.get("projectId") ?? relatedCaseState?.projectId ?? "";
  const relatedCaseId = relatedCaseState?.relatedCaseId;
  const relatedCaseNumber = relatedCaseState?.relatedCaseNumber;

  // The draft outlives this page (it lives above the routes), so switching to
  // an open case tab and back does not lose what was entered. It is restored
  // only into a form opened the same way it was started (same locked project
  // and related case); a different entry starts fresh rather than inheriting a
  // stale draft. Cleared on a successful submit and on Cancel.
  const draftStore = useNewServiceRequestDraft();
  const entryKey = `${lockedProjectId}|${relatedCaseId ?? ""}`;
  const [restored] = useState(() => {
    const draft = draftStore.get();
    return draft?.entryKey === entryKey ? draft : undefined;
  });
  // The entry this form was started from. What is entered belongs to it: if the
  // same route is reached again with another entry while the form stays mounted,
  // the initialisers below do not run again, so the values on screen must not be
  // saved under the new entry's key.
  const [startedFrom] = useState(entryKey);

  const [projectId, setProjectId] = useState(restored?.projectId ?? lockedProjectId);
  const [deploymentId, setDeploymentId] = useState(
    restored?.deploymentId ?? relatedCaseState?.deploymentId ?? "",
  );
  // Raw selection (possibly prefilled from the related case or a restored
  // draft). The effective `deployedProductId` below drops it when that product
  // turns out to be ineligible for service requests.
  const [selectedDeployedProductId, setDeployedProductId] = useState(
    restored?.deployedProductId ?? relatedCaseState?.deployedProductId ?? "",
  );
  const [catalogId, setCatalogId] = useState(restored?.catalogId ?? "");
  const [catalogItemId, setCatalogItemId] = useState(restored?.catalogItemId ?? "");
  // "" means untouched: the field is left out of the payload and the server
  // applies its configured default. It is deliberately not preset to a value.
  const [severity, setSeverity] = useState<ServiceRequestSeverity | "">(restored?.severity ?? "");
  // Variable answers, keyed by variable id.
  const [answers, setAnswers] = useState<Record<string, string>>(restored?.answers ?? {});
  // Files are not kept in the draft (they cannot be stored); they are chosen again.
  const [attachments, setAttachments] = useState<PendingAttachment[]>([]);

  // `hasSr` is precomputed by the backing data source, so an ineligible
  // project is caught before the engineer fills out the rest of the form
  // rather than on a rejected submit. `hasServiceRequestReadAccess` (from
  // project metadata, checked below once it's loaded) is a separate,
  // viewer-permission check — see BeProject.hasSr's doc comment for why both
  // exist. Either being false makes the project ineligible.
  const selectedProject = useGetProject(projectId || undefined);
  const isIneligibleForSr = projectId
    ? selectedProject.data?.hasSr === false
    : false;
  // Fail closed: a project-load error or a 404 (data === null) means
  // eligibility is unknown, not confirmed — canSubmit must not treat unknown
  // as eligible.
  const projectLoadFailed =
    !!projectId &&
    !selectedProject.isLoading &&
    (selectedProject.isError || selectedProject.data === null);

  // Service requests are only raisable against a subset of a project's
  // deployed products, keyed by category — mirrors CP's
  // CreateServiceRequestPage, which restricts its deployed-product picker the
  // same way. Ineligible products are still listed (disabled, with a reason
  // tooltip) so the engineer can see why a product can't be used, rather than
  // wondering where it went. The backend remains the authority and rejects
  // creation against them; this is a display of data eligibility only.
  // The full, unnarrowed product list is therefore fetched and classified
  // client-side. A missing/empty/failed-to-load srProductCategories never
  // disables anything: fail open, not closed.
  const projectMetadata = useProjectMetadata(projectId || undefined);
  const srProductCategories = projectMetadata.data?.features?.srProductCategories;
  // `hasServiceRequestReadAccess` gates the SR feature itself (viewer
  // permission), separately from `hasSr` (project eligibility) above. Fail
  // open while metadata is still loading/unloaded (`undefined`) — only an
  // explicit `false` once loaded blocks submission; the same fail-closed
  // `projectLoadFailed`/`isLoading` guard on `canSubmit` below already
  // prevents submitting before metadata resolves.
  const hasNoSrReadAccess =
    projectMetadata.data?.features?.hasServiceRequestReadAccess === false;

  const deployments = useSearchDeployments(projectId || undefined);
  const deployedProducts = useDeployedProductOptions(deploymentId || undefined);
  const deployedProductOptions = useMemo(() => {
    const options = deployedProducts.data ?? [];
    const restricted = !!srProductCategories && srProductCategories.length > 0;
    return options.map((o) => ({
      ...o,
      ineligibleReason: restricted
        ? getIneligibleReason(o.category, srProductCategories)
        : null,
    }));
  }, [deployedProducts.data, srProductCategories]);
  const eligibleProductCount = deployedProductOptions.filter(
    (o) => o.ineligibleReason === null,
  ).length;
  // A prefilled (or otherwise stale) selection that resolves to an ineligible
  // product is treated as unselected, so the form can't submit a doomed
  // request. A product not (yet) in the list is left alone.
  const deployedProductId = deployedProductOptions.some(
    (o) => o.id === selectedDeployedProductId && o.ineligibleReason !== null,
  )
    ? ""
    : selectedDeployedProductId;
  // Reason tooltips open on pointer hover and on keyboard focus of a disabled
  // option, so keyboard users can read why it can't be chosen.
  const [hoveredProductId, setHoveredProductId] = useState<string | null>(null);
  const [focusedProductId, setFocusedProductId] = useState<string | null>(null);
  const catalogs = useSearchCatalogs(deployedProductId || undefined);
  const variables = useCatalogItemVariables(
    catalogId || undefined,
    catalogItemId || undefined,
  );
  const postCase = usePostCsmCase();
  const postAttachment = usePostCsmCaseAttachment();
  const uploadedBy = useEngineerDisplayName();
  // Spans the whole submit (create + post-create attachment uploads).
  const [submitting, setSubmitting] = useState(false);

  // Mirror the form into the draft store on every change. `submitted` stops it
  // writing the draft back after a successful create (or a Cancel) has cleared
  // it, and `submitting` holds it off while a create is in flight: submit
  // clears the draft up front, so a form reopened mid-create starts empty
  // instead of offering a complete, enabled copy that could be sent twice. A
  // failed create flips `submitting` back and the draft is saved again.
  const submitted = useRef(false);
  useEffect(() => {
    if (submitted.current || submitting || entryKey !== startedFrom) return;
    draftStore.set({
      entryKey,
      projectId,
      deploymentId,
      deployedProductId,
      catalogId,
      catalogItemId,
      severity,
      answers,
    });
  }, [
    draftStore,
    entryKey,
    startedFrom,
    projectId,
    deploymentId,
    deployedProductId,
    catalogId,
    catalogItemId,
    severity,
    answers,
    submitting,
  ]);

  const catalogItems = useMemo(
    () => catalogs.data?.find((c) => c.id === catalogId)?.catalogItems ?? [],
    [catalogs.data, catalogId],
  );

  // ServiceNow returns context/hidden fields mixed in; only user-editable
  // (non-attachment) variables get a rendered input. Attachments go to the
  // shared attachments section below.
  const allVariables = useMemo(() => variables.data ?? [], [variables.data]);
  const renderableVars = useMemo(
    () => getUserEditableVariables(allVariables).filter((v) => !isAttachmentField(v)),
    [allVariables],
  );
  const firstEmptyRequired = useMemo(
    () => getFirstEmptyRequiredField(allVariables, answers),
    [allVariables, answers],
  );
  // Checked in this order — empty-required first, since a field that's both
  // empty and has a maxLength/validation rule should report the more
  // actionable "fill this in" message, not a pattern mismatch on nothing.
  const firstExceedingMaxLength = useMemo(
    () =>
      firstEmptyRequired ? null : getFirstFieldExceedingMaxLength(allVariables, answers),
    [allVariables, answers, firstEmptyRequired],
  );
  const firstFailingValidation = useMemo(
    () =>
      firstEmptyRequired || firstExceedingMaxLength
        ? null
        : getFirstFieldFailingValidation(allVariables, answers),
    [allVariables, answers, firstEmptyRequired, firstExceedingMaxLength],
  );
  // Send user-editable, non-attachment variables, encoded by type, omitting
  // empties. Context/hidden fields are excluded by getUserEditableVariables.
  const variablePayload = useMemo(
    () =>
      getUserEditableVariables(allVariables)
        .filter((v) => !isAttachmentField(v))
        .map((v) => ({ id: v.id, value: encodeVariableValue(v, answers[v.id] ?? "") }))
        .filter((v) => v.value !== ""),
    [allVariables, answers],
  );
  // The backend refuses a service request that carries no answers at all
  // ("variables are required for service_request"): a catalog item whose
  // questions are all optional and left blank, or one with no questions.
  const noVariableAnswered = variables.isSuccess && variablePayload.length === 0;


  // A deployed product with no catalogs is the common non-ServiceNow case;
  // tell the engineer rather than leaving an empty dropdown.
  const noCatalogs =
    !!deployedProductId &&
    catalogs.isSuccess &&
    (catalogs.data?.length ?? 0) === 0;

  // Cascade resets: a parent change invalidates every dependent field below it.
  const onProjectChange = (next: string): void => {
    setProjectId(next);
    setDeploymentId("");
    setDeployedProductId("");
    setCatalogId("");
    setCatalogItemId("");
    setAnswers({});
  };
  const onDeploymentChange = (next: string): void => {
    setDeploymentId(next);
    setDeployedProductId("");
    setCatalogId("");
    setCatalogItemId("");
    setAnswers({});
  };
  const onDeployedProductChange = (next: string): void => {
    // Disabled options are keyboard-focusable (so their reason is
    // discoverable), and the Select still routes Enter/Space on one through
    // its change handler. Ineligible products must never become the value.
    if (
      deployedProductOptions.some((o) => o.id === next && o.ineligibleReason !== null)
    ) {
      return;
    }
    setDeployedProductId(next);
    setCatalogId("");
    setCatalogItemId("");
    setAnswers({});
  };
  const onCatalogChange = (next: string): void => {
    setCatalogId(next);
    setCatalogItemId("");
    setAnswers({});
  };
  const onCatalogItemChange = (next: string): void => {
    setCatalogItemId(next);
    setAnswers({});
  };

  const hasOptionsError =
    deployments.isError || deployedProducts.isError || catalogs.isError;
  const retryOptions = (): void => {
    if (deployments.isError) void deployments.refetch();
    if (deployedProducts.isError) void deployedProducts.refetch();
    if (catalogs.isError) void catalogs.refetch();
  };

  const canSubmit = useMemo(
    () =>
      !!projectId &&
      !!deploymentId &&
      !!deployedProductId &&
      !!catalogId &&
      !!catalogItemId &&
      // isSuccess, not !isLoading && !isError: a query paused offline is
      // neither, and would otherwise allow submit with no questions loaded.
      variables.isSuccess &&
      // Required-ness now comes from each variable's own `mandatory` flag
      // where the backend supplies one (see `isVariableRequired`), falling
      // back to the old "every typable field required" hot fix only where
      // it doesn't.
      firstEmptyRequired === null &&
      firstExceedingMaxLength === null &&
      firstFailingValidation === null &&
      !noVariableAnswered &&
      !isIneligibleForSr &&
      !hasNoSrReadAccess &&
      // A prefilled product is kept until classification can run, so don't
      // allow submit while the product list or the project's category
      // restriction is still loading (it might turn out to be ineligible).
      !deployedProducts.isLoading &&
      !projectMetadata.isLoading &&
      // Fail closed while a selected project's eligibility is still loading
      // or couldn't be confirmed at all — see projectLoadFailed above.
      (!projectId || (!selectedProject.isLoading && !projectLoadFailed)) &&
      !submitting,
    [
      projectId,
      deploymentId,
      deployedProductId,
      catalogId,
      catalogItemId,
      variables.isSuccess,
      firstEmptyRequired,
      firstExceedingMaxLength,
      firstFailingValidation,
      noVariableAnswered,
      isIneligibleForSr,
      hasNoSrReadAccess,
      deployedProducts.isLoading,
      projectMetadata.isLoading,
      selectedProject.isLoading,
      projectLoadFailed,
      submitting,
    ],
  );

  const handleSubmit = async (): Promise<void> => {
    if (!canSubmit) return;
    draftStore.clear();
    setSubmitting(true);
    try {
      const created = await postCase.mutateAsync({
        type: "service_request",
        projectId,
        deploymentId,
        deployedProductId,
        catalogId,
        catalogItemId,
        variables: variablePayload,
        // Only sent when the engineer picked one; otherwise the server default applies.
        ...(severity ? { severity: priorityFromSeverity(severity) } : {}),
        relatedCaseId,
      });
      // The draft was cleared when this create started and nothing has saved one
      // since (the mirroring effect holds off while `submitting`), so none of this
      // submission is left to clear. Clearing again here would discard a draft
      // that a form reopened mid-create has started since.
      submitted.current = true;
      // Files are uploaded to the new case afterwards. A partial failure still
      // lands the case.
      const failed = await uploadAttachmentsToCase(
        postAttachment.mutateAsync,
        created.id,
        attachments,
        uploadedBy,
      );
      if (failed.length > 0) showError(failedUploadsMessage("service request", failed));
      navigate(`/cases/${created.id}`, { state: { from: backTarget } });
    } catch (err) {
      setSubmitting(false);
      // The backend surfaces real validation messages on 4xx; show them.
      const msg =
        err instanceof BackendApiError && err.status < 500 && err.message
          ? err.message
          : "Could not create the service request. Please try again.";
      showError(msg, err);
    }
  };

  return (
    <Box sx={{ width: "100%", px: 3, py: 3 }}>
      <Button
        variant="text"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(backTarget)}
        sx={{ mb: 1 }}
      >
        Back
      </Button>
      <Typography variant="h5" sx={{ mb: relatedCaseId ? 0.5 : 2 }}>
        New service request
      </Typography>
      {relatedCaseId && (
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Creating a service request linked to case {relatedCaseNumber ?? "the case"} — its id is carried through automatically.
        </Typography>
      )}

      <Card variant="outlined" sx={{ p: 3 }}>
        {hasOptionsError && (
          <Box
            sx={{
              mb: 2,
              display: "flex",
              alignItems: "center",
              gap: 1.5,
              flexWrap: "wrap",
            }}
          >
            <Typography variant="body2" color="error.main">
              Some dropdown options failed to load. See the failed fields below for details.
            </Typography>
            <Button size="small" variant="outlined" onClick={retryOptions}>
              Retry all
            </Button>
          </Box>
        )}

        {projectLoadFailed && (
          <Alert
            severity="error"
            sx={{ mb: 2 }}
            action={
              <Button size="small" onClick={() => void selectedProject.refetch()}>
                Retry
              </Button>
            }
          >
            Could not load this project. Its service-request eligibility can't be confirmed.
          </Alert>
        )}

        {!!projectId &&
          !selectedProject.isLoading &&
          !projectLoadFailed &&
          (isIneligibleForSr || hasNoSrReadAccess) && (
            <Alert severity="error" sx={{ mb: 2 }}>
              This project isn't eligible to raise service requests.
            </Alert>
          )}

        <Grid container spacing={2.5}>
          <Grid size={{ xs: 12, md: 4 }}>
            <ProjectSelectionField
              value={projectId}
              onChange={onProjectChange}
              lockedProjectId={lockedProjectId}
              required
              filterProject={isOfferedProject}
            />
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel
                id="sr-deployment-label"
                shrink={deploymentId !== ""}
                sx={{ top: "0px !important" }}
              >
                Deployment
              </InputLabel>
              <Select
                labelId="sr-deployment-label"
                label="Deployment"
                value={deploymentId}
                onChange={(e) => onDeploymentChange(String(e.target.value))}
                disabled={!projectId || deployments.isLoading}
                notched={deploymentId !== ""}
              >
                {(deployments.data ?? []).map((d) => (
                  <MenuItem key={d.id} value={d.id}>
                    {d.name ?? d.id}
                  </MenuItem>
                ))}
              </Select>
              {!projectId ? (
                <FormHelperText>Select a project first</FormHelperText>
              ) : deployments.isError ? (
                <FormHelperText error>Failed to load deployments.</FormHelperText>
              ) : deployments.isLoading ? (
                <FormHelperText>Loading deployments…</FormHelperText>
              ) : (deployments.data ?? []).length === 0 ? (
                <FormHelperText>No deployments found for this project.</FormHelperText>
              ) : null}
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel
                id="sr-product-label"
                shrink={deployedProductId !== ""}
                sx={{ top: "0px !important" }}
              >
                Deployed product
              </InputLabel>
              <Select
                labelId="sr-product-label"
                label="Deployed product"
                value={deployedProductId}
                onChange={(e) => onDeployedProductChange(String(e.target.value))}
                disabled={!deploymentId || deployedProducts.isLoading}
                notched={deployedProductId !== ""}
                // Let keyboard users reach disabled options (the reason is
                // exposed on them); selection is still refused in
                // onDeployedProductChange.
                MenuProps={{ MenuListProps: { disabledItemsFocusable: true } }}
              >
                {deployedProductOptions.map((dp) => (
                  <MenuItem
                    key={dp.id}
                    value={dp.id}
                    disabled={dp.ineligibleReason !== null}
                    aria-description={dp.ineligibleReason ?? undefined}
                    onFocus={() => setFocusedProductId(dp.id)}
                    onBlur={() =>
                      setFocusedProductId((cur) => (cur === dp.id ? null : cur))
                    }
                  >
                    {dp.ineligibleReason !== null ? (
                      // A disabled MenuItem has `pointer-events: none`, so the
                      // Tooltip target re-enables pointer events on its own
                      // span. The item itself stays a direct Select child, so
                      // it remains skipped by keyboard navigation and
                      // unselectable. `describeChild` keeps the option's
                      // accessible name as the product label and exposes the
                      // reason as its description.
                      <Tooltip
                        title={dp.ineligibleReason}
                        describeChild
                        open={hoveredProductId === dp.id || focusedProductId === dp.id}
                        onOpen={() => setHoveredProductId(dp.id)}
                        onClose={() =>
                          setHoveredProductId((cur) => (cur === dp.id ? null : cur))
                        }
                      >
                        <Box
                          component="span"
                          sx={{ display: "block", width: "100%", pointerEvents: "auto" }}
                        >
                          {dp.label}
                        </Box>
                      </Tooltip>
                    ) : (
                      dp.label
                    )}
                  </MenuItem>
                ))}
              </Select>
              {!deploymentId ? (
                <FormHelperText>Select a deployment first</FormHelperText>
              ) : deployedProducts.isError ? (
                <FormHelperText error>Failed to load deployed products.</FormHelperText>
              ) : deployedProducts.isLoading ? (
                <FormHelperText>Loading products…</FormHelperText>
              ) : deployedProductOptions.length === 0 ? (
                <FormHelperText>No deployed products found for this deployment.</FormHelperText>
              ) : eligibleProductCount === 0 ? (
                <FormHelperText>
                  No products in this deployment can be used for a service request.
                </FormHelperText>
              ) : null}
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 6 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel
                id="sr-catalog-label"
                shrink={catalogId !== ""}
                sx={{ top: "0px !important" }}
              >
                Catalog
              </InputLabel>
              <Select
                labelId="sr-catalog-label"
                label="Catalog"
                value={catalogId}
                onChange={(e) => onCatalogChange(String(e.target.value))}
                disabled={!deployedProductId || catalogs.isLoading || noCatalogs}
                notched={catalogId !== ""}
              >
                {(catalogs.data ?? []).map((c) => (
                  <MenuItem key={c.id} value={c.id}>
                    {c.name ?? c.id}
                  </MenuItem>
                ))}
              </Select>
              {!deployedProductId ? (
                <FormHelperText>Select a deployed product first</FormHelperText>
              ) : catalogs.isError ? (
                <FormHelperText error>Failed to load catalogs.</FormHelperText>
              ) : catalogs.isLoading ? (
                <FormHelperText>Loading catalogs…</FormHelperText>
              ) : noCatalogs ? (
                <FormHelperText>
                  No service catalogs are available for this product.
                </FormHelperText>
              ) : null}
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 6 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel
                id="sr-catalog-item-label"
                shrink={catalogItemId !== ""}
                sx={{ top: "0px !important" }}
              >
                Catalog item
              </InputLabel>
              <Select
                labelId="sr-catalog-item-label"
                label="Catalog item"
                value={catalogItemId}
                onChange={(e) => onCatalogItemChange(String(e.target.value))}
                disabled={!catalogId}
                notched={catalogItemId !== ""}
              >
                {catalogItems.map((ci) => (
                  <MenuItem key={ci.id} value={ci.id}>
                    {ci.name ?? ci.id}
                  </MenuItem>
                ))}
              </Select>
              {!catalogId ? (
                <FormHelperText>Select a catalog first</FormHelperText>
              ) : catalogItems.length === 0 ? (
                <FormHelperText>No items found in this catalog.</FormHelperText>
              ) : null}
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small">
              <InputLabel id="sr-severity-label" shrink sx={{ top: "0px !important" }}>
                Severity
              </InputLabel>
              <Select
                labelId="sr-severity-label"
                label="Severity"
                value={severity}
                displayEmpty
                notched
                onChange={(e) => setSeverity(e.target.value as ServiceRequestSeverity | "")}
                disabled={submitting}
              >
                <MenuItem value="">Default</MenuItem>
                {SERVICE_REQUEST_SEVERITIES.map((s) => (
                  <MenuItem key={s} value={s}>
                    {s} · {SEVERITY_LABEL[s]}
                  </MenuItem>
                ))}
              </Select>
              <FormHelperText>
                Internal only, never shown to the customer. Leave as Default to use the standard
                severity.
              </FormHelperText>
            </FormControl>
          </Grid>

          {/* Dynamic variable form for the chosen catalog item. */}
          {catalogItemId && (
            <Grid size={{ xs: 12 }}>
              {variables.isLoading ? (
                <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, py: 1 }}>
                  <CircularProgress size={18} />
                  <Typography variant="body2" color="text.secondary">
                    Loading request form…
                  </Typography>
                </Box>
              ) : variables.isError ? (
                <QueryErrorState
                  message={variables.error instanceof Error && variables.error.message.trim() ? variables.error.message : "Could not load the request form for this catalog item."}
                  error={variables.error}
                />
              ) : renderableVars.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  This catalog item has no additional fields.
                </Typography>
              ) : (
                <CatalogVariableFields
                  variables={renderableVars}
                  values={answers}
                  onChange={(id, value) =>
                    setAnswers((prev) => ({ ...prev, [id]: value }))
                  }
                />
              )}
            </Grid>
          )}

          {/* Optional supporting attachments. */}
          {catalogItemId && !variables.isLoading && !variables.isError && (
            <Grid size={{ xs: 12 }}>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ display: "block", mb: 0.5 }}
              >
                Attachments
              </Typography>
              <AttachmentsField
                attachments={attachments}
                onChange={setAttachments}
                onError={showError}
                maxFiles={CREATE_FORM_MAX_ATTACHMENTS}
              />
            </Grid>
          )}
        </Grid>

        <Box
          sx={{
            display: "flex",
            justifyContent: "flex-end",
            alignItems: "center",
            gap: 1.5,
            mt: 2.5,
          }}
        >
          {!variables.isLoading && (firstEmptyRequired || firstExceedingMaxLength || firstFailingValidation || (!!catalogItemId && noVariableAnswered)) && (
            <Typography variant="caption" color="text.secondary" sx={{ mr: "auto" }}>
              {firstEmptyRequired
                ? `Required field: ${firstEmptyRequired}`
                : firstExceedingMaxLength
                  ? `${firstExceedingMaxLength.label} exceeds ${firstExceedingMaxLength.maxLength} characters`
                  : firstFailingValidation
                    ? `${firstFailingValidation.label}: ${firstFailingValidation.message}`
                    : renderableVars.length === 0
                      ? "A service request needs at least one answer, and this catalog item has no questions to answer."
                      : "Answer at least one question."}
            </Typography>
          )}
          <Button
            variant="outlined"
            onClick={() => {
              // An explicit discard: drop the draft too (flag first, so the
              // mirroring effect cannot write it back) or it would reappear
              // the next time the form opens.
              submitted.current = true;
              draftStore.clear();
              navigate(backTarget);
            }}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={() => void handleSubmit()}
            disabled={!canSubmit}
          >
            {submitting ? "Creating…" : "Create service request"}
          </Button>
        </Box>
      </Card>
    </Box>
  );
}
