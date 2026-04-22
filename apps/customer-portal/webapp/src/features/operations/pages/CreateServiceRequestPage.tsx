// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { CircleCheck } from "@wso2/oxygen-ui-icons-react";
import {
  useState,
  useMemo,
  useCallback,
  useEffect,
  useRef,
  type FormEvent,
  type JSX,
} from "react";
import {
  useNavigate,
  useParams,
  useLocation,
  useSearchParams,
} from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { usePostProjectDeploymentsSearchInfinite } from "@api/usePostProjectDeploymentsSearch";
import type { ProjectDeploymentItem } from "@features/project-details/types/deployments";
import {
  extractDeploymentProducts,
  usePostDeploymentProductsSearchInfinite,
} from "@features/project-details/api/usePostDeploymentProductsSearch";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useSearchCatalogs } from "@features/operations/api/useSearchCatalogs";
import { useGetCatalogItemVariables } from "@features/operations/api/useGetCatalogItemVariables";
import { usePostCase } from "@features/operations/api/usePostCase";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import {
  getFirstEmptyRequiredField,
  isContextField,
  isDescriptionField,
} from "@features/operations/utils/serviceRequestValidation";
import {
  getBaseDeploymentOptions,
  getBaseProductOptions,
  getDeploymentProductDisplayLabel,
  isUnknownPlaceholderProductLabel,
  resolveDeploymentMatch,
  resolveProductId,
} from "@features/support/utils/caseCreation";
import {
  refreshCaseQueriesAfterCreation,
  triggerPostCreationApiCalls,
} from "@features/operations/utils/caseRefresh";
import {
  filterDeploymentsForCaseCreation,
  getProjectPermissions,
  shouldRestrictToPrimaryProductionDeployments,
} from "@utils/permission";
import { htmlToPlainText } from "@features/support/utils/richTextEditor";
import type { CreateServiceRequestPayload } from "@features/operations/types/serviceRequests";
import CatalogSelector from "@features/operations/components/service-requests/CatalogSelector";
import VariableFormFields from "@features/operations/components/service-requests/VariableFormFields";
import UploadAttachmentModal from "@features/support/components/case-details/attachments-tab/UploadAttachmentModal";
import { CaseCreationHeader } from "@features/support/components/case-creation-layout/header/CaseCreationHeader";
import { BasicInformationSection } from "@features/support/components/case-creation-layout/form-sections/basic-information-section/BasicInformationSection";
import { CaseType } from "@features/support/constants/supportConstants";

function getCreateServiceRequestLoadingState(
  isProjectLoading: boolean,
  isDeploymentsLoading: boolean,
  projectId: string | undefined,
): boolean {
  if (!projectId) {
    return true;
  }
  if (isProjectLoading || isDeploymentsLoading) {
    return true;
  }
  return false;
}

function getCreateServiceRequestValidationError(params: {
  projectId: string | undefined;
  deploymentMatchExists: boolean;
  productId: string;
  selectedCatalogId: string;
  selectedCatalogItemId: string;
  firstEmptyRequiredField: string | null;
}): string | null {
  const {
    projectId,
    deploymentMatchExists,
    productId,
    selectedCatalogId,
    selectedCatalogItemId,
    firstEmptyRequiredField,
  } = params;

  if (!projectId) return "Missing project context.";
  if (!deploymentMatchExists) return "Please select a deployment type.";
  if (!productId) return "Please select a product version.";
  if (!selectedCatalogId || !selectedCatalogItemId) {
    return "Please select a request type (catalog item).";
  }
  if (firstEmptyRequiredField) {
    return `Please fill in the required field: ${firstEmptyRequiredField}`;
  }
  return null;
}

function getCanSubmitServiceRequest(params: {
  projectId: string | undefined;
  selectedDeploymentId: string;
  productId: string;
  selectedCatalogId: string;
  selectedCatalogItemId: string;
  isCreatePending: boolean;
}): boolean {
  const {
    projectId,
    selectedDeploymentId,
    productId,
    selectedCatalogId,
    selectedCatalogItemId,
    isCreatePending,
  } = params;

  if (
    !projectId ||
    !selectedDeploymentId ||
    !productId ||
    !selectedCatalogId ||
    !selectedCatalogItemId
  ) {
    return false;
  }
  if (isCreatePending) {
    return false;
  }
  return true;
}

/**
 * CreateServiceRequestPage - multi-step form to create a service request.
 * Flow: Select Deployment -> Select Product -> Select Catalog Item -> Fill Variables -> Submit.
 *
 * @returns {JSX.Element} The Create Service Request page.
 */
export default function CreateServiceRequestPage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const { projectId } = useParams<{ projectId: string }>();
  const basePath = location.pathname.includes("/operations/")
    ? "operations"
    : "support";
  const { showLoader, hideLoader } = useLoader();
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();
  const queryClient = useQueryClient();
  const authFetch = useAuthApiClient();

  const [deployment, setDeployment] = useState("");
  const [product, setProduct] = useState("");
  const [selectedCatalogId, setSelectedCatalogId] = useState("");
  const [selectedCatalogItemId, setSelectedCatalogItemId] = useState("");
  const [variableValues, setVariableValues] = useState<Record<string, string>>(
    {},
  );
  type AttachmentItem = { id: string; file: File };
  const [attachments, setAttachments] = useState<AttachmentItem[]>([]);
  const attachmentNamesRef = useRef<Map<string, string>>(new Map());
  const attachmentIdCounterRef = useRef(0);
  const requestDetailsSectionRef = useRef<HTMLDivElement>(null);
  const [isAttachmentModalOpen, setIsAttachmentModalOpen] = useState(false);
  const [isNavigatingAfterCreate, setIsNavigatingAfterCreate] = useState(false);

  const { data: projectDetails, isLoading: isProjectLoading } =
    useGetProjectDetails(projectId || "");
  const { data: projectFeatures, isLoading: isFeaturesLoading } =
    useGetProjectFeatures(projectId || "");
  const srPermissions = useMemo(
    () =>
      getProjectPermissions(projectDetails?.type?.label, {
        projectFeatures,
      }),
    [projectDetails?.type?.label, projectFeatures],
  );
  const hasSR = srPermissions.hasSR;
  const deploymentsQuery = usePostProjectDeploymentsSearchInfinite(
    projectId || "",
    {
      pageSize: 10,
      enabled: !!projectId && !isProjectLoading && !isFeaturesLoading && hasSR,
    },
  );
  const allProjectDeployments = useMemo(
    () =>
      deploymentsQuery.data?.pages.flatMap((p) => p.deployments ?? []) ?? [],
    [deploymentsQuery.data],
  );
  const isPrimaryProductionOnly = shouldRestrictToPrimaryProductionDeployments(
    projectDetails?.type?.label,
  );
  const projectDeployments = useMemo(
    () =>
      filterDeploymentsForCaseCreation(
        allProjectDeployments,
        projectDetails?.type?.label,
      ),
    [allProjectDeployments, projectDetails?.type?.label],
  );
  const isDeploymentsLoading = deploymentsQuery.isLoading;

  const prefill = useMemo(() => {
    const depId = searchParams.get("deploymentId")?.trim() ?? "";
    const prodId = searchParams.get("productId")?.trim() ?? "";
    if (!depId || !projectDeployments.length)
      return { deploymentLabel: "", productId: "" };
    const dep = projectDeployments.find(
      (d: ProjectDeploymentItem) => d.id === depId,
    );
    const deploymentLabel = dep?.name?.trim() || dep?.type?.label?.trim() || "";
    return { deploymentLabel, productId: prodId };
  }, [searchParams, projectDeployments]);

  const effectiveDeployment = deployment || prefill.deploymentLabel || "";
  const effectiveProduct = product || prefill.productId || "";

  const baseDeploymentOptions = getBaseDeploymentOptions(projectDeployments);
  const selectedDeploymentMatch = useMemo(
    () =>
      resolveDeploymentMatch(
        effectiveDeployment,
        projectDeployments,
        undefined,
      ),
    [effectiveDeployment, projectDeployments],
  );
  const selectedDeploymentId = selectedDeploymentMatch?.id ?? "";

  const deploymentProductsQuery = usePostDeploymentProductsSearchInfinite(
    selectedDeploymentId,
    { pageSize: 10, enabled: !!selectedDeploymentId },
  );
  const deploymentProductsLoading = deploymentProductsQuery.isLoading;
  const deploymentProductsData = useMemo(
    () =>
      deploymentProductsQuery.data?.pages.flatMap((p) =>
        extractDeploymentProducts(p),
      ) ?? [],
    [deploymentProductsQuery.data],
  );

  const allDeploymentProducts = useMemo(
    () =>
      (deploymentProductsData ?? []).filter((item) => {
        const label = getDeploymentProductDisplayLabel(item);
        return (
          Boolean(label.trim()) && !isUnknownPlaceholderProductLabel(label)
        );
      }),
    [deploymentProductsData],
  );
  const baseProductOptions = getBaseProductOptions(allDeploymentProducts);
  const sortedProductOptions = useMemo(
    () =>
      [...baseProductOptions].sort((a, b) =>
        a.label.localeCompare(b.label, undefined, { numeric: true }),
      ),
    [baseProductOptions],
  );

  const productId = resolveProductId(effectiveProduct, allDeploymentProducts);
  const { data: catalogsData, isLoading: isCatalogsLoading } =
    useSearchCatalogs(productId);
  const { data: variablesData, isLoading: isVariablesLoading } =
    useGetCatalogItemVariables(selectedCatalogId, selectedCatalogItemId);

  const selectedCatalogItemLabel = useMemo(() => {
    if (!selectedCatalogId || !selectedCatalogItemId || !catalogsData?.catalogs)
      return undefined;
    const catalog = catalogsData.catalogs.find(
      (c) => c.id === selectedCatalogId,
    );
    const item = catalog?.catalogItems?.find(
      (i) => i.id === selectedCatalogItemId,
    );
    return item?.label;
  }, [catalogsData, selectedCatalogId, selectedCatalogItemId]);

  const { mutate: postCase, isPending: isCreatePending } = usePostCase();

  const isInitialLoading = getCreateServiceRequestLoadingState(
    isProjectLoading,
    isDeploymentsLoading,
    projectId,
  );
  useEffect(() => {
    if (isInitialLoading) {
      showLoader();
    } else {
      hideLoader();
    }
    return () => hideLoader();
  }, [isInitialLoading, showLoader, hideLoader]);

  const handleDeploymentChange = useCallback((value: string) => {
    setDeployment(value);
    setProduct("");
    setSelectedCatalogId("");
    setSelectedCatalogItemId("");
    setVariableValues({});
    setAttachments([]);
  }, []);

  useEffect(() => {
    const depId = searchParams.get("deploymentId")?.trim();
    if (!depId) return;
    if (allProjectDeployments.some((d) => d.id === depId)) return;
    if (deploymentsQuery.hasNextPage && !deploymentsQuery.isFetchingNextPage) {
      void deploymentsQuery.fetchNextPage();
    }
  }, [searchParams, allProjectDeployments, deploymentsQuery]);

  // For Cloud Support / Cloud Evaluation Support: auto-pick the first primary
  // production deployment so the hidden field still resolves to a valid value.
  useEffect(() => {
    if (!isPrimaryProductionOnly) return;
    if (!baseDeploymentOptions.length) return;
    const first = baseDeploymentOptions[0];
    if (!first) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- keep deployment aligned with restricted mode options
    setDeployment((prev) =>
      baseDeploymentOptions.includes(prev) ? prev : first,
    );
  }, [isPrimaryProductionOnly, baseDeploymentOptions]);

  const handleProductChange = useCallback((value: string) => {
    setProduct(value);
    setSelectedCatalogId("");
    setSelectedCatalogItemId("");
    setVariableValues({});
    setAttachments([]);
  }, []);

  const handleSelectCatalogItem = useCallback(
    (catalogId: string, catalogItemId: string) => {
      setSelectedCatalogId(catalogId);
      setSelectedCatalogItemId(catalogItemId);
      setVariableValues({});
      setAttachments([]);
    },
    [],
  );

  useEffect(() => {
    if (selectedCatalogItemId) {
      requestDetailsSectionRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    }
  }, [selectedCatalogItemId]);

  const handleVariableChange = useCallback(
    (variableId: string, value: string) => {
      setVariableValues((prev) => ({ ...prev, [variableId]: value }));
    },
    [],
  );

  const handleAttachmentClick = () => setIsAttachmentModalOpen(true);

  const handleSelectAttachment = (file: File, attachmentName?: string) => {
    setAttachments((prev) => {
      const sig = `${file.name}-${file.size}-${file.lastModified}`;
      if (
        prev.some(
          (a) => `${a.file.name}-${a.file.size}-${a.file.lastModified}` === sig,
        )
      )
        return prev;
      const id = `att-${++attachmentIdCounterRef.current}-${Date.now()}`;
      if (attachmentName?.trim())
        attachmentNamesRef.current.set(id, attachmentName.trim());
      return [...prev, { id, file }];
    });
  };

  const handleAttachmentRemove = (index: number) => {
    setAttachments((prev) => {
      const item = prev[index];
      if (item) attachmentNamesRef.current.delete(item.id);
      return prev.filter((_, i) => i !== index);
    });
  };

  const handleBack = () => {
    if (window.history.length > 1) {
      navigate(-1);
    } else if (projectId) {
      navigate(`/projects/${projectId}/${basePath}/service-requests`);
    } else {
      navigate("/");
    }
  };

  const fileToBase64 = (file: File): Promise<string> =>
    new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        const s = typeof reader.result === "string" ? reader.result : "";
        const i = s.indexOf(",");
        resolve(i >= 0 ? s.slice(i + 1) : s);
      };
      reader.onerror = () => reject(new Error(`Failed to read ${file.name}`));
      reader.readAsDataURL(file);
    });

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (isNavigatingAfterCreate) return;

    const deploymentMatch = resolveDeploymentMatch(
      deployment,
      projectDeployments,
      undefined,
    );

    const variables = variablesData?.variables ?? [];
    const contextValues = {
      projectDisplay: projectDetails?.name ?? "",
      deploymentDisplay: deployment,
      productDisplay:
        sortedProductOptions.find((p) => p.id === product)?.label ?? "",
    };
    const firstEmpty = getFirstEmptyRequiredField(
      variables,
      contextValues,
      variableValues,
    );

    const validationError = getCreateServiceRequestValidationError({
      projectId,
      deploymentMatchExists: !!deploymentMatch,
      productId,
      selectedCatalogId,
      selectedCatalogItemId,
      firstEmptyRequiredField: firstEmpty,
    });
    if (validationError) {
      showError(validationError);
      return;
    }
    if (!projectId || !deploymentMatch) {
      return;
    }

    const variablePayload = variables
      .filter((v) => !isContextField(v.questionText ?? ""))
      .map((v) => {
        const raw = variableValues[v.id] ?? "";
        const value = isDescriptionField(v.questionText ?? "")
          ? raw.trim()
          : htmlToPlainText(raw).trim();
        return { id: v.id, value };
      })
      .filter((v) => v.value !== "");

    let encodedAttachments: Array<{ name: string; file: string }> = [];
    if (attachments.length > 0) {
      try {
        encodedAttachments = await Promise.all(
          attachments.map(async (item) => ({
            name: attachmentNamesRef.current.get(item.id) || item.file.name,
            file: await fileToBase64(item.file),
          })),
        );
      } catch {
        showError("Failed to process attachments. Please try again.");
        return;
      }
    }

    const payload: CreateServiceRequestPayload = {
      type: "service_request",
      projectId,
      deploymentId: deploymentMatch.id,
      deployedProductId: productId,
      catalogId: selectedCatalogId,
      catalogItemId: selectedCatalogItemId,
      variables: variablePayload,
      ...(encodedAttachments.length > 0 && { attachments: encodedAttachments }),
    };

    postCase(payload, {
      onSuccess: async (data) => {
        setIsNavigatingAfterCreate(true);
        const srNumber = (data as { number?: string }).number;
        showSuccess(
          srNumber
            ? `Service request ${srNumber} created successfully`
            : "Service request created successfully",
        );

        if (projectId) {
          await triggerPostCreationApiCalls(
            authFetch,
            projectId,
            CaseType.SERVICE_REQUEST,
          );
          await refreshCaseQueriesAfterCreation(
            queryClient,
            projectId,
            CaseType.SERVICE_REQUEST,
          );
        }

        navigate(
          `/projects/${projectId}/${basePath}/service-requests/${data.id}`,
        );
      },
      onError: (error) => {
        setIsNavigatingAfterCreate(false);
        const msg =
          error?.message?.trim() ||
          "We couldn't create your service request. Please try again.";
        showError(msg);
      },
    });
  };

  const projectDisplay = projectDetails?.name ?? "";
  const isProductDropdownDisabled =
    !selectedDeploymentId || deploymentProductsLoading;
  const canSubmit = getCanSubmitServiceRequest({
    projectId,
    selectedDeploymentId,
    productId,
    selectedCatalogId,
    selectedCatalogItemId,
    isCreatePending: isCreatePending || isNavigatingAfterCreate,
  });

  if (
    !isProjectLoading &&
    !isFeaturesLoading &&
    projectDetails &&
    !srPermissions.hasSR
  ) {
    return (
      <Box sx={{ width: "100%", pt: 0, position: "relative", p: 3 }}>
        <CaseCreationHeader
          onBack={handleBack}
          hideAiChip
          backLabel="Back"
          title="New Service Request"
          subtitle="Service requests are not available for this project"
        />
        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          Service requests are not available for this project type.
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ width: "100%", pt: 0, position: "relative" }}>
      <CaseCreationHeader
        onBack={handleBack}
        hideAiChip
        backLabel="Back"
        title="New Service Request"
        subtitle="Select deployment, product, and request type, then complete the required fields"
      />

      <Box
        component="form"
        onSubmit={handleSubmit}
        sx={{ display: "flex", flexDirection: "column", gap: 3 }}
      >
        <BasicInformationSection
          project={projectDisplay}
          product={effectiveProduct}
          setProduct={handleProductChange}
          deployment={effectiveDeployment}
          setDeployment={handleDeploymentChange}
          productOptionList={sortedProductOptions}
          isProductAutoDetected={false}
          isDeploymentAutoDetected={false}
          isRelatedCaseMode
          hideDeploymentField={isPrimaryProductionOnly}
          metadata={{ deploymentTypes: baseDeploymentOptions }}
          isDeploymentLoading={isProjectLoading || isDeploymentsLoading}
          isProductDropdownDisabled={isProductDropdownDisabled}
          isProductLoading={!!selectedDeploymentId && deploymentProductsLoading}
          onLoadMoreDeployments={() => {
            if (
              deploymentsQuery.hasNextPage &&
              !deploymentsQuery.isFetchingNextPage
            ) {
              void deploymentsQuery.fetchNextPage();
            }
          }}
          hasMoreDeployments={!!deploymentsQuery.hasNextPage}
          isFetchingMoreDeployments={deploymentsQuery.isFetchingNextPage}
          onLoadMoreProducts={() => {
            if (
              deploymentProductsQuery.hasNextPage &&
              !deploymentProductsQuery.isFetchingNextPage
            ) {
              void deploymentProductsQuery.fetchNextPage();
            }
          }}
          hasMoreProducts={!!deploymentProductsQuery.hasNextPage}
          isFetchingMoreProducts={deploymentProductsQuery.isFetchingNextPage}
          projectTypeLabel={projectDetails?.type?.label}
        />

        {!!productId && (
          <CatalogSelector
            catalogs={catalogsData?.catalogs}
            isLoading={isCatalogsLoading}
            selectedCatalogId={selectedCatalogId}
            selectedCatalogItemId={selectedCatalogItemId}
            onSelectCatalogItem={handleSelectCatalogItem}
          />
        )}

        {!!selectedCatalogId && !!selectedCatalogItemId && (
          <div ref={requestDetailsSectionRef}>
            <VariableFormFields
              variables={variablesData?.variables}
              isLoading={isVariablesLoading}
              values={variableValues}
              onChange={handleVariableChange}
              selectedRequestTypeLabel={selectedCatalogItemLabel}
              contextValues={{
                projectDisplay: projectDisplay,
                deploymentDisplay: deployment,
                productDisplay:
                  sortedProductOptions.find((p) => p.id === product)?.label ?? "",
              }}
              attachments={attachments}
              onAttachmentClick={handleAttachmentClick}
              onAttachmentRemove={handleAttachmentRemove}
            />
          </div>
        )}

        <UploadAttachmentModal
          open={isAttachmentModalOpen}
          onClose={() => setIsAttachmentModalOpen(false)}
          onSelect={handleSelectAttachment}
        />

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5 }}>
          <Button variant="outlined" color="inherit" onClick={handleBack}>
            Cancel
          </Button>
          <Button
            type="submit"
            variant="contained"
            color="primary"
            startIcon={<CircleCheck size={18} />}
            disabled={!canSubmit}
          >
            {isCreatePending
              ? "Creating..."
              : isNavigatingAfterCreate
                ? "Opening request..."
                : "Create Service Request"}
          </Button>
        </Box>
      </Box>
    </Box>
  );
}
