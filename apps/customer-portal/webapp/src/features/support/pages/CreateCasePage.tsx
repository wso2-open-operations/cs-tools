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

import { Box, Button, Grid } from "@wso2/oxygen-ui";
import { CircleCheck } from "@wso2/oxygen-ui-icons-react";
import {
  useState,
  useEffect,
  useRef,
  useMemo,
  useCallback,
  type FormEvent,
  type JSX,
} from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectContacts from "@features/settings/api/useGetProjectContacts";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { usePostProjectDeploymentsSearchInfinite } from "@api/usePostProjectDeploymentsSearch";
import {
  extractDeploymentProducts,
  usePostDeploymentProductsSearchInfinite,
} from "@features/project-details/api/usePostDeploymentProductsSearch";
import { usePostCase } from "@features/operations/api/usePostCase";
import { usePostAttachments } from "@features/support/api/usePostAttachments";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useLogger } from "@hooks/useLogger";
import type { CreateCaseRequest } from "@features/support/types/cases";
import { BasicInformationSection } from "@features/support/components/case-creation-layout/form-sections/basic-information-section/BasicInformationSection";
import { CaseCreationHeader } from "@features/support/components/case-creation-layout/header/CaseCreationHeader";
import { CaseDetailsSection } from "@features/support/components/case-creation-layout/form-sections/case-details-section/CaseDetailsSection";
import { WatchListSection } from "@features/support/components/case-creation-layout/form-sections/watch-list-section/WatchListSection";
import { ConversationSummary } from "@features/support/components/case-creation-layout/form-sections/conversation-summary-section/ConversationSummary";
import { RelatedCaseSummary } from "@features/support/components/case-creation-layout/form-sections/conversation-summary-section/RelatedCaseSummary";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import {
  buildClassificationProductLabel,
  findMatchingDeploymentLabel,
  findMatchingProductId,
  getBaseDeploymentOptions,
  getBaseProductOptions,
  getDeploymentDisplayLabelForEnvironment,
  getDeploymentProductDisplayLabel,
  isDeploymentDropdownEmpty,
  isProductDropdownEmpty,
  isUnknownPlaceholderProductLabel,
  resolveDeploymentMatch,
  resolveIssueTypeKey,
  resolveProductId,
  shouldAddClassificationProductToOptions,
} from "@features/support/utils/caseCreation";
import { isCreatedCaseSecurityReport } from "@features/support/utils/support";
import {
  refreshCaseQueriesAfterCreation,
  triggerPostCreationApiCalls,
} from "@features/operations/utils/caseRefresh";
import {
  CaseSeverity,
  CaseSeverityLevel,
  CaseType,
} from "@features/support/constants/supportConstants";
import { SecurityTabId } from "@features/security/types/security";
import {
  filterDeploymentsForCaseCreation,
  getProjectSeverityPolicy,
  shouldRestrictToPrimaryProductionDeployments,
} from "@utils/permission";
import {
  escapeHtml,
  htmlToPlainText,
} from "@features/support/utils/richTextEditor";
import { usePiiGuard } from "@features/support/hooks/usePiiGuard";
import PiiWarningDialog from "@features/support/components/dialogs/PiiWarningDialog";
import UploadAttachmentModal from "@features/support/components/case-details/attachments-tab/UploadAttachmentModal";
import AddProductModal from "@features/project-details/components/deployments/AddProductModal";
import AddDeploymentWizardModal from "@features/project-details/components/deployments/AddDeploymentWizardModal";
import { isDeploymentSetupDuringCaseCreationEnabled } from "@config/caseCreationConfig";
import type {
  ProductCategory,
  ProjectDeploymentItem,
} from "@features/project-details/types/deployments";
import type { RelatedCaseState } from "@features/support/types/createCasePage";

const DEFAULT_CASE_TITLE = "Support case";
const DEFAULT_CASE_DESCRIPTION = "Please describe your issue here.";
const ATTACHMENT_UPLOAD_WAIT_MS = 5_000;

const RELATED_DESCRIPTION_PREFIX_HTML =
  "<p>-- This is the previous description (Edit or Delete if you want to alter) --</p>";

const RELATED_DESCRIPTION_HTML_TAG_REGEX =
  /<[a-zA-Z][^>]*>[\s\S]*<\/[a-zA-Z][^>]*>|<[a-zA-Z][^>]*\/>/;

function buildRelatedCaseDescriptionHtml(rawDescription?: string): string {
  const base = (rawDescription ?? "").trim();
  if (!base) {
    return RELATED_DESCRIPTION_PREFIX_HTML;
  }

  const isLikelyHtml = RELATED_DESCRIPTION_HTML_TAG_REGEX.test(base);
  const normalizedBody = isLikelyHtml ? base : `<p>${escapeHtml(base)}</p>`;

  return `${RELATED_DESCRIPTION_PREFIX_HTML}${normalizedBody}`;
}

/**
 * CreateCasePage component to review and edit AI-generated case details.
 *
 * @returns {JSX.Element} The rendered CreateCasePage.
 */
export default function CreateCasePage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const { projectId } = useParams<{ projectId: string }>();
  const locationStateRaw = location.state as {
    relatedCase?: RelatedCaseState;
    skipChat?: boolean;
  } | null;
  const relatedCase = locationStateRaw?.relatedCase;

  // Check if creating a security report analysis case
  const searchParams = new URLSearchParams(location.search);
  const caseType = searchParams.get("type");
  const isSecurityReportPath = location.pathname.includes("security-report");
  const isSecurityReport =
    caseType === CaseType.SECURITY_REPORT_ANALYSIS || isSecurityReportPath;
  const skipChat = !!locationStateRaw?.skipChat || isSecurityReport;
  const { showLoader, hideLoader } = useLoader();
  const { data: projectDetails, isLoading: isProjectLoading } =
    useGetProjectDetails(projectId || "");
  const { data: projectFeatures, isLoading: isProjectFeaturesLoading } =
    useGetProjectFeatures(projectId || "");
  const severityPolicy =
    projectDetails && !isProjectFeaturesLoading && projectFeatures
      ? getProjectSeverityPolicy(projectDetails.type?.label, {
          projectFeatures,
        })
      : { excludeS0: true, restrictSeverityToLow: true };
  const { excludeS0, restrictSeverityToLow: forceSeverityS4 } = severityPolicy;
  const { data: filters, isLoading: isFiltersLoading } = useGetProjectFilters(
    projectId || "",
  );
  const { data: projectContacts, isLoading: isContactsLoading } =
    useGetProjectContacts(projectId || "");
  const { data: currentUser } = useGetUserDetails();
  const [title, setTitle] = useState(() => relatedCase?.title ?? "");
  const [description, setDescription] = useState(() =>
    relatedCase ? buildRelatedCaseDescriptionHtml(relatedCase.description) : "",
  );
  const [issueType, setIssueType] = useState("");
  const [product, setProduct] = useState("");
  const [deployment, setDeployment] = useState("");
  const [severity, setSeverity] = useState("");
  const [watchList, setWatchList] = useState<string[]>([]);
  const [classificationProductLabel, setClassificationProductLabel] =
    useState("");
  type AttachmentItem = { id: string; file: File };
  const [attachments, setAttachments] = useState<AttachmentItem[]>([]);
  const attachmentNamesRef = useRef<Map<string, string>>(new Map());
  const attachmentIdCounterRef = useRef(0);
  const isSubmittingRef = useRef(false);
  const [isPreparingAttachments, setIsPreparingAttachments] = useState(false);
  const [isAttachmentModalOpen, setIsAttachmentModalOpen] = useState(false);
  const [isAddProductModalOpen, setIsAddProductModalOpen] = useState(false);
  const [isDeploymentWizardOpen, setIsDeploymentWizardOpen] = useState(false);
  const deploymentsQuery = usePostProjectDeploymentsSearchInfinite(
    projectId || "",
    {
      pageSize: 10,
      enabled: !!projectId,
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
  const baseDeploymentOptions = getBaseDeploymentOptions(projectDeployments);
  const selectedDeploymentMatch = useMemo(
    () => resolveDeploymentMatch(deployment, projectDeployments, undefined),
    [deployment, projectDeployments],
  );
  const selectedDeploymentId =
    selectedDeploymentMatch?.id ?? relatedCase?.deploymentId ?? "";
  const deploymentProductsQuery = usePostDeploymentProductsSearchInfinite(
    selectedDeploymentId,
    {
      pageSize: 10,
      enabled: !!selectedDeploymentId,
      request: {
        filters: {
          productCategories: (projectFeatures?.defaultCaseProductCategories ??
            undefined) as ProductCategory[] | undefined,
        },
      },
    },
  );
  const deploymentProductsLoading = deploymentProductsQuery.isLoading;
  const deploymentProductsError = deploymentProductsQuery.isError;
  const allDeploymentProducts = useMemo(() => {
    const items =
      deploymentProductsQuery.data?.pages.flatMap((page) =>
        extractDeploymentProducts(page),
      ) ?? [];
    return items.filter((item) => {
      const label = getDeploymentProductDisplayLabel(item);
      return Boolean(label.trim()) && !isUnknownPlaceholderProductLabel(label);
    });
  }, [deploymentProductsQuery.data]);
  const baseProductOptions = getBaseProductOptions(allDeploymentProducts);

  // Sort product options in ascending order by label
  const sortedBaseProductOptions = useMemo(() => {
    return [...baseProductOptions].sort((a, b) => {
      return a.label.localeCompare(b.label, undefined, {
        numeric: true,
        sensitivity: "base",
      });
    });
  }, [baseProductOptions]);

  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();
  const piiGuard = usePiiGuard();
  const { mutate: postCase, isPending: isCreatePending } = usePostCase();
  const postAttachments = usePostAttachments();
  const [isNavigatingAfterCreate, setIsNavigatingAfterCreate] = useState(false);
  const authFetch = useAuthApiClient();
  const logger = useLogger();

  useEffect(() => {
    if (deploymentProductsError) {
      showError(
        "Could not load product options. Some options may be unavailable.",
      );
    }
  }, [deploymentProductsError, showError]);

  useEffect(() => {
    if (!projectContacts) return;
    const eligibleEmails = projectContacts
      .filter((c) => c.isCsAdmin || c.isCsIntegrationUser || c.isPortalUser || !c.isSecurityContact)
      .map((c) => c.email);
    const creatorEmail = currentUser?.email;
    const merged = creatorEmail
      ? Array.from(new Set([creatorEmail, ...eligibleEmails]))
      : Array.from(new Set(eligibleEmails));
    setWatchList(merged);
  }, [projectContacts, currentUser?.email]);

  const hasInitializedRef = useRef(false);
  const hasClassificationAppliedRef = useRef(false);
  const skipDescriptionOnChangeRef = useRef(false);
  const classificationDescriptionRef = useRef<string>("");
  const [isDeploymentManuallySet, setIsDeploymentManuallySet] = useState(false);
  const [isProductManuallySet, setIsProductManuallySet] = useState(false);
  // Set when the deployment/product dropdown was auto-filled because it had
  // exactly one option (as opposed to a manual pick or an AI-classification
  // match). Drives the same "Auto detected" affordance as classification so
  // the pre-fill isn't silent.
  const [
    isDeploymentSingleOptionAutoSelected,
    setIsDeploymentSingleOptionAutoSelected,
  ] = useState(false);
  const [
    isProductSingleOptionAutoSelected,
    setIsProductSingleOptionAutoSelected,
  ] = useState(false);
  const [isIssueTypeFromClassification, setIsIssueTypeFromClassification] =
    useState(false);
  const [isSeverityFromClassification, setIsSeverityFromClassification] =
    useState(false);
  const [classificationDeploymentLabel, setClassificationDeploymentLabel] =
    useState("");
  const [isDeploymentFromClassification, setIsDeploymentFromClassification] =
    useState(false);
  const [isTitleFromClassification, setIsTitleFromClassification] =
    useState(false);
  const [isDescriptionFromClassification, setIsDescriptionFromClassification] =
    useState(false);

  const skipChatMode = skipChat;
  const noAiMode = !!relatedCase || skipChatMode;
  // Kill switch for the whole "add deployment/product inline" feature
  // (empty-state alert, in-menu add-new, wizard, singleton auto-select).
  // Defaults to enabled - see caseCreationConfig.ts.
  const isDeploymentSetupEnabled = isDeploymentSetupDuringCaseCreationEnabled();
  const queryClient = useQueryClient();

  const locationState = location.state as {
    classificationResponse?: {
      issueType?: string;
      severityLevel?: string;
      caseInfo?: {
        description?: string;
        shortDescription?: string;
        productName?: string;
        productVersion?: string;
        environment?: string;
      };
    };
    conversationId?: string;
  } | null;

  const STORAGE_KEY = `case_classification_data_${projectId}`;
  const CONVERSATION_ID_STORAGE_KEY = `case_conversation_id_${projectId}`;

  const [classificationResponse, setClassificationResponse] = useState<
    | {
        issueType?: string;
        severityLevel?: string;
        caseInfo?: {
          description?: string;
          shortDescription?: string;
          productName?: string;
          productVersion?: string;
          environment?: string;
        };
      }
    | undefined
  >(() => {
    if (skipChat) return undefined;
    if (locationState?.classificationResponse) {
      return locationState.classificationResponse;
    }
    try {
      const stored = sessionStorage.getItem(STORAGE_KEY);
      return stored ? JSON.parse(stored) : undefined;
    } catch (e) {
      logger.error("Failed to parse stored classification data", e);
      return undefined;
    }
  });

  useEffect(() => {
    if (locationState?.classificationResponse) {
      try {
        sessionStorage.setItem(
          STORAGE_KEY,
          JSON.stringify(locationState.classificationResponse),
        );
      } catch (e) {
        logger.error(
          "Failed to store classification data in sessionStorage",
          e,
        );
      }
      setClassificationResponse(locationState.classificationResponse);
    }
  }, [locationState?.classificationResponse, STORAGE_KEY, logger]);

  // Persist conversationId to survive page refresh
  const [conversationId, setConversationId] = useState<string | undefined>(
    () => {
      if (locationState?.conversationId) {
        return locationState.conversationId;
      }
      try {
        const stored = sessionStorage.getItem(CONVERSATION_ID_STORAGE_KEY);
        return stored || undefined;
      } catch (e) {
        logger.error(
          "Failed to retrieve conversationId from sessionStorage",
          e,
        );
        return undefined;
      }
    },
  );

  useEffect(() => {
    if (locationState?.conversationId) {
      try {
        sessionStorage.setItem(
          CONVERSATION_ID_STORAGE_KEY,
          locationState.conversationId,
        );
      } catch (e) {
        logger.error("Failed to store conversationId in sessionStorage", e);
      }
      setConversationId(locationState.conversationId);
    }
  }, [locationState?.conversationId, CONVERSATION_ID_STORAGE_KEY, logger]);

  // Persist conversationId whenever it changes
  useEffect(() => {
    try {
      if (conversationId) {
        sessionStorage.setItem(CONVERSATION_ID_STORAGE_KEY, conversationId);
      } else {
        sessionStorage.removeItem(CONVERSATION_ID_STORAGE_KEY);
      }
    } catch (e) {
      logger.error("Failed to persist conversationId to sessionStorage", e);
    }
  }, [conversationId, CONVERSATION_ID_STORAGE_KEY, logger]);

  const projectDisplay = projectDetails?.name ?? "";

  const issueTypesList = useMemo(
    () => (filters?.issueTypes ?? []) as { id: string; label: string }[],
    [filters?.issueTypes],
  );
  const severityLevelsList = useMemo(
    () => (filters?.severities ?? []) as { id: string; label: string }[],
    [filters?.severities],
  );

  useEffect(() => {
    if (isProjectLoading || isProjectFeaturesLoading || isFiltersLoading) {
      showLoader();
    } else {
      hideLoader();
    }
    return () => hideLoader();
  }, [
    isProjectLoading,
    isProjectFeaturesLoading,
    isFiltersLoading,
    showLoader,
    hideLoader,
  ]);

  const handleDeploymentChange = useCallback((value: string) => {
    setDeployment(value);
    setProduct("");
    setIsDeploymentManuallySet(true);
    setIsDeploymentFromClassification(false);
    setClassificationDeploymentLabel("");
    setIsProductManuallySet(true);
    setIsDeploymentSingleOptionAutoSelected(false);
    setIsProductSingleOptionAutoSelected(false);
  }, []);

  // For Cloud Support / Cloud Evaluation Support: auto-pick the first primary
  // production deployment and keep it locked to that value.
  useEffect(() => {
    if (!isPrimaryProductionOnly) return;
    if (!baseDeploymentOptions.length) return;
    const first = baseDeploymentOptions[0];
    if (!first) return;
    setDeployment((prev) =>
      baseDeploymentOptions.includes(prev) ? prev : first,
    );
  }, [isPrimaryProductionOnly, baseDeploymentOptions]);

  const handleProductChange = useCallback((value: string) => {
    setProduct(value);
    setIsProductManuallySet(true);
    setIsProductSingleOptionAutoSelected(false);
  }, []);

  const handleIssueTypeChange = useCallback((value: string) => {
    setIssueType(value);
    setIsIssueTypeFromClassification(false);
  }, []);

  const handleSeverityChange = useCallback((value: string) => {
    setSeverity(value);
    setIsSeverityFromClassification(false);
  }, []);

  const handleTitleChange = useCallback((value: string) => {
    setTitle(value);
    setIsTitleFromClassification(false);
  }, []);

  const handleDescriptionChange = useCallback((value: string) => {
    if (skipDescriptionOnChangeRef.current) {
      skipDescriptionOnChangeRef.current = false;
      classificationDescriptionRef.current = value;
      setDescription(value);
      return;
    }
    setDescription(value);
    if (value !== classificationDescriptionRef.current) {
      setIsDescriptionFromClassification(false);
    }
  }, []);

  // Auto-fill title for security reports when deployment and product are selected
  // This will overwrite any manually entered title
  useEffect(() => {
    if (!isSecurityReport || !deployment || !product) return;

    // Get deployment type label
    const deploymentMatch = resolveDeploymentMatch(
      deployment,
      projectDeployments,
      undefined,
    );
    const deploymentId = deploymentMatch?.id;
    const deploymentObj = projectDeployments?.find(
      (d: ProjectDeploymentItem) => d.id === deploymentId,
    );
    const deploymentLabel =
      deploymentObj?.name || deploymentObj?.type?.label || deployment;

    // Get product name without version
    const selectedProduct = allDeploymentProducts.find(
      (item) =>
        item.id === product ||
        getDeploymentProductDisplayLabel(item) === product,
    );
    const productName = selectedProduct?.product?.label?.trim() || "";

    // Format today's date as YYYY-MM-DD
    const today = new Date();
    const year = today.getFullYear();
    const month = String(today.getMonth() + 1).padStart(2, "0");
    const day = String(today.getDate()).padStart(2, "0");
    const dateStr = `${year}-${month}-${day}`;

    // Generate title: "Deployment Type - Product Name - Date"
    if (productName) {
      const generatedTitle = `${deploymentLabel} - ${productName} - ${dateStr}`;
      setTitle(generatedTitle);
    }
  }, [
    isSecurityReport,
    deployment,
    product,
    projectDeployments,
    allDeploymentProducts,
  ]);

  useEffect(() => {
    if (hasInitializedRef.current) return;
    if (isFiltersLoading || isDeploymentsLoading) return;

    const initialDeployment = baseDeploymentOptions[0] ?? "";
    const initialIssueType = noAiMode ? "" : (issueTypesList[0]?.label ?? "");
    const initialSeverity = noAiMode ? "" : (severityLevelsList[0]?.id ?? "");

    queueMicrotask(() => {
      if (noAiMode) {
        if (!relatedCase) {
          // Blank by default (the user picks consciously since there's no
          // AI-classification signal to justify a guess) - EXCEPT when
          // there's exactly one deployment: forcing a manual pick from a
          // single-item dropdown isn't a real choice, just an extra click.
          if (isDeploymentSetupEnabled && baseDeploymentOptions.length === 1) {
            setDeployment(baseDeploymentOptions[0]);
            setIsDeploymentSingleOptionAutoSelected(true);
          } else {
            setDeployment("");
          }
          setTitle("");
          setDescription("");
        }
        setProduct("");
        setIssueType("");
        if (!forceSeverityS4) setSeverity("");
      } else if (!classificationResponse) {
        setDeployment(initialDeployment);
        setProduct("");
        setIssueType(initialIssueType);
        setSeverity(initialSeverity);
        setTitle(DEFAULT_CASE_TITLE);
        setDescription(DEFAULT_CASE_DESCRIPTION);
      }
    });
    hasInitializedRef.current = true;
  }, [
    baseDeploymentOptions,
    classificationResponse,
    forceSeverityS4,
    isDeploymentsLoading,
    issueTypesList,
    isFiltersLoading,
    isDeploymentSetupEnabled,
    noAiMode,
    relatedCase,
    severityLevelsList,
  ]);

  // For Development Support: lock severity to S4 (Low).
  useEffect(() => {
    if (!forceSeverityS4) return;
    if (isFiltersLoading || !severityLevelsList.length) return;
    const s4Level = severityLevelsList.find(
      (s) => s.label === CaseSeverity.LOW,
    );
    if (s4Level) {
      setSeverity(s4Level.id);
    }
  }, [forceSeverityS4, isFiltersLoading, severityLevelsList]);

  // When opening a related case, prefill title, description (with prefix), and deployment from parent.
  const hasRelatedCaseInitializedRef = useRef(false);
  useEffect(() => {
    if (!relatedCase) return;
    if (hasRelatedCaseInitializedRef.current) return;

    setTitle(relatedCase.title ?? "");
    setDescription(buildRelatedCaseDescriptionHtml(relatedCase.description));

    hasRelatedCaseInitializedRef.current = true;
  }, [relatedCase]);

  const hasRelatedCaseDeploymentInitializedRef = useRef(false);
  useEffect(() => {
    if (!relatedCase?.deploymentId && !relatedCase?.deploymentLabel) return;
    if (!projectDeployments?.length) return;
    if (hasRelatedCaseDeploymentInitializedRef.current) return;

    const deploymentOptionFromId = relatedCase.deploymentId
      ? projectDeployments
          .find((d: ProjectDeploymentItem) => d.id === relatedCase.deploymentId)
          ?.name?.trim() ||
        projectDeployments
          .find((d: ProjectDeploymentItem) => d.id === relatedCase.deploymentId)
          ?.type?.label?.trim()
      : undefined;
    const deploymentOptionFromLabel = relatedCase.deploymentLabel
      ? findMatchingDeploymentLabel(
          relatedCase.deploymentLabel,
          baseDeploymentOptions,
        )
      : undefined;
    const resolvedDeploymentOption =
      deploymentOptionFromId || deploymentOptionFromLabel;

    if (resolvedDeploymentOption) {
      setDeployment(resolvedDeploymentOption);
      hasRelatedCaseDeploymentInitializedRef.current = true;
    }
  }, [relatedCase, projectDeployments, baseDeploymentOptions]);

  // Auto-select the deployment when there is exactly one option, so the
  // customer isn't forced to click through a single-item dropdown. This is
  // deliberately narrow: it only fills gaps left by the flows above, it
  // never fights them.
  //   - `noAiMode && !relatedCase` (skipChat) is the one flow that explicitly
  //     clears deployment to "" on init (see the init effect above) as a
  //     defensive reset; that reset owns the blank state, so this effect
  //     stays out of it.
  //   - A related case that names a deployment (`deploymentId`/
  //     `deploymentLabel`) is resolved by the effect above from the parent
  //     case, not from option count, so this effect defers to it entirely
  //     even if that resolution hasn't landed yet.
  //   - Everything else (no related case, or a related case with no
  //     deployment hint, or the AI-classification flow once a
  //     classificationResponse exists) is fair game: `!deployment` guards
  //     against re-firing once a value lands from any source, so this can
  //     only ever run once per "became a singleton" transition.
  useEffect(() => {
    if (!isDeploymentSetupEnabled) return;
    if (deployment) return;
    if (isDeploymentsLoading) return;
    if (baseDeploymentOptions.length !== 1) return;
    if (noAiMode && !relatedCase) return;
    if (relatedCase?.deploymentId || relatedCase?.deploymentLabel) return;

    setDeployment(baseDeploymentOptions[0]);
    setIsDeploymentSingleOptionAutoSelected(true);
  }, [
    isDeploymentSetupEnabled,
    deployment,
    isDeploymentsLoading,
    baseDeploymentOptions,
    noAiMode,
    relatedCase,
  ]);

  // Same "undo if it turns out not to be a real singleton" guard as the
  // product effect below - covers the same query-timing race for the
  // deployments list.
  useEffect(() => {
    if (!isDeploymentSingleOptionAutoSelected) return;
    if (baseDeploymentOptions.length <= 1) return;

    setDeployment("");
    setIsDeploymentSingleOptionAutoSelected(false);
  }, [isDeploymentSingleOptionAutoSelected, baseDeploymentOptions]);

  // Auto-select the product/version when the currently selected deployment
  // resolves to exactly one product option. Mirrors the deployment effect
  // above, but product has no "intentionally left blank" flow to defer to:
  // every path clears `product` to "" on init and relies on either the
  // related-case/classification sync effect (below) or this effect to fill
  // it back in. A related case that names a product
  // (`deployedProductId`/`deployedProductLabel`) is resolved there, not from
  // option count, so this effect steps aside for it.
  useEffect(() => {
    if (!isDeploymentSetupEnabled) return;
    if (!selectedDeploymentId) return;
    if (product) return;
    if (deploymentProductsLoading) return;
    if (sortedBaseProductOptions.length !== 1) return;
    if (relatedCase?.deployedProductId || relatedCase?.deployedProductLabel)
      return;

    setProduct(sortedBaseProductOptions[0].id);
    setIsProductSingleOptionAutoSelected(true);
  }, [
    isDeploymentSetupEnabled,
    selectedDeploymentId,
    product,
    deploymentProductsLoading,
    sortedBaseProductOptions,
    relatedCase,
  ]);

  // Undoes the auto-select above if the option list later turns out to have
  // more than one item after all (e.g. the wizard's product query resolves
  // incrementally right after adding two products, and the singleton effect
  // fired on a transient one-item view before the second one caught up).
  // Only ever reverses a value THIS effect's sibling actually auto-picked
  // (`isProductSingleOptionAutoSelected`) - a manual selection is never
  // touched, even if the list happens to change afterward.
  useEffect(() => {
    if (!isProductSingleOptionAutoSelected) return;
    if (sortedBaseProductOptions.length <= 1) return;

    setProduct("");
    setIsProductSingleOptionAutoSelected(false);
  }, [isProductSingleOptionAutoSelected, sortedBaseProductOptions]);

  useEffect(() => {
    if (noAiMode) return;
    if (!classificationResponse?.caseInfo) return;
    const info = classificationResponse.caseInfo;
    if (info.shortDescription?.trim()) {
      setTitle(info.shortDescription);
      setIsTitleFromClassification(true);
    }
    if (info.description?.trim()) {
      const text = info.description.trim();
      const isLikelyHtml =
        /<[a-zA-Z][^>]*>[\s\S]*<\/[a-zA-Z][^>]*>|<[a-zA-Z][^>]*\/>/.test(text);
      const html = isLikelyHtml ? text : `<p>${escapeHtml(text)}</p>`;
      skipDescriptionOnChangeRef.current = true;
      setDescription(html);
      setIsDescriptionFromClassification(true);
    }
  }, [classificationResponse, noAiMode]);

  useEffect(() => {
    if (noAiMode) return;
    if (hasClassificationAppliedRef.current || !classificationResponse) return;
    if (isFiltersLoading) return;

    if (!severityLevelsList.length) return;

    const info = classificationResponse.caseInfo;
    const deploymentLabel = info?.environment?.trim();
    const productLabel = buildClassificationProductLabel(info);
    const issueTypeLabel = classificationResponse.issueType?.trim();
    const severityLabel = classificationResponse.severityLevel?.trim();

    hasClassificationAppliedRef.current = true;

    if (deploymentLabel) setClassificationDeploymentLabel(deploymentLabel);
    if (productLabel) setClassificationProductLabel(productLabel);
    // Product is auto-selected in the sync effect when products for the matched deployment load
    const issueTypeMatched = !!(
      issueTypeLabel &&
      issueTypesList.some(
        (t) => t.label === issueTypeLabel || t.id === issueTypeLabel,
      )
    );
    setIssueType((prev) => (issueTypeMatched ? issueTypeLabel! : prev));
    if (issueTypeMatched) setIsIssueTypeFromClassification(true);

    const severityMapping: Record<string, string> = {
      [CaseSeverityLevel.S0]: CaseSeverity.CATASTROPHIC,
      [CaseSeverityLevel.S1]: CaseSeverity.CRITICAL,
      [CaseSeverityLevel.S2]: CaseSeverity.HIGH,
      [CaseSeverityLevel.S3]: CaseSeverity.MEDIUM,
      [CaseSeverityLevel.S4]: CaseSeverity.LOW,
      S4: CaseSeverity.LOW,
    };

    const mappedLabel = severityMapping[severityLabel ?? ""] ?? severityLabel;

    const matchedSeverity = severityLevelsList.find(
      (s) =>
        s.id === severityLabel ||
        s.label === severityLabel ||
        s.label === mappedLabel,
    );
    setSeverity((prev) => (matchedSeverity ? matchedSeverity.id : prev));
    if (matchedSeverity) setIsSeverityFromClassification(true);
  }, [
    classificationResponse,
    isFiltersLoading,
    noAiMode,
    issueTypesList,
    severityLevelsList,
  ]);

  // Reactively match deployment from classification as pages load.
  useEffect(() => {
    if (
      noAiMode ||
      !classificationDeploymentLabel?.trim() ||
      isDeploymentManuallySet
    )
      return;
    if (isDeploymentFromClassification) return;
    const matched =
      getDeploymentDisplayLabelForEnvironment(
        classificationDeploymentLabel,
        projectDeployments,
      ) ??
      findMatchingDeploymentLabel(
        classificationDeploymentLabel,
        baseDeploymentOptions,
      );
    if (matched) {
      setDeployment(matched);
      setIsDeploymentFromClassification(true);
    }
  }, [
    classificationDeploymentLabel,
    projectDeployments,
    baseDeploymentOptions,
    noAiMode,
    isDeploymentManuallySet,
    isDeploymentFromClassification,
  ]);

  // Auto-fetch more deployment pages when classification suggests a deployment not yet loaded.
  useEffect(() => {
    if (
      noAiMode ||
      !classificationDeploymentLabel?.trim() ||
      isDeploymentManuallySet
    )
      return;
    if (isDeploymentFromClassification) return;
    if (deploymentsQuery.isFetchingNextPage || !deploymentsQuery.hasNextPage)
      return;
    const alreadyFound =
      getDeploymentDisplayLabelForEnvironment(
        classificationDeploymentLabel,
        projectDeployments,
      ) ??
      findMatchingDeploymentLabel(
        classificationDeploymentLabel,
        baseDeploymentOptions,
      );
    if (alreadyFound) return;
    void deploymentsQuery.fetchNextPage();
  }, [
    noAiMode,
    classificationDeploymentLabel,
    isDeploymentManuallySet,
    isDeploymentFromClassification,
    projectDeployments,
    baseDeploymentOptions,
    deploymentsQuery,
  ]);

  useEffect(() => {
    if (!selectedDeploymentId || !sortedBaseProductOptions.length) return;
    if (noAiMode && relatedCase) {
      // Pre-select the deployed product from the related case, but only once.
      setProduct((current) => {
        if (current?.trim()) return current;
        if (relatedCase.deployedProductId) {
          const found = sortedBaseProductOptions.some(
            (o) => o.id === relatedCase.deployedProductId,
          );
          if (found) return relatedCase.deployedProductId;
        }
        if (relatedCase.deployedProductLabel) {
          const fromLabel = findMatchingProductId(
            relatedCase.deployedProductLabel,
            sortedBaseProductOptions,
          );
          if (fromLabel) return fromLabel;
        }
        return current;
      });
      return;
    }
    setProduct((current) => {
      if (!current?.trim()) {
        const fromClassification = findMatchingProductId(
          classificationProductLabel,
          sortedBaseProductOptions,
        );
        if (classificationProductLabel?.trim()) {
          return fromClassification ?? "";
        }
        return "";
      }
      const found = sortedBaseProductOptions.some((o) => o.id === current);
      if (found) return current;
      const fromLabel = findMatchingProductId(
        current,
        sortedBaseProductOptions,
      );
      return fromLabel ?? "";
    });
  }, [
    sortedBaseProductOptions,
    noAiMode,
    relatedCase,
    classificationProductLabel,
    selectedDeploymentId,
  ]);

  // Auto-fetch more product pages when classification suggests a product not yet loaded.
  useEffect(() => {
    if (!classificationProductLabel?.trim()) return;
    if (!selectedDeploymentId) return;
    if (
      deploymentProductsQuery.isFetchingNextPage ||
      !deploymentProductsQuery.hasNextPage
    )
      return;
    const alreadyFound = findMatchingProductId(
      classificationProductLabel,
      sortedBaseProductOptions,
    );
    if (alreadyFound) return;
    void deploymentProductsQuery.fetchNextPage();
  }, [
    classificationProductLabel,
    sortedBaseProductOptions,
    selectedDeploymentId,
    deploymentProductsQuery,
  ]);

  const handleBack = () => {
    const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;
    if (returnTo) {
      navigate(returnTo);
      return;
    }
    navigate(-1);
  };

  const handleAttachmentClick = () => {
    setIsAttachmentModalOpen(true);
  };

  const fileSignature = (f: File) => `${f.name}-${f.size}-${f.lastModified}`;

  const handleSelectAttachment = (file: File, attachmentName?: string) => {
    setAttachments((prev) => {
      const isDuplicate = prev.some(
        (a) => fileSignature(a.file) === fileSignature(file),
      );
      if (isDuplicate) return prev;
      const uniqueId = `att-${++attachmentIdCounterRef.current}-${Date.now()}`;
      if (attachmentName?.trim()) {
        attachmentNamesRef.current.set(uniqueId, attachmentName.trim());
      }
      return [...prev, { id: uniqueId, file }];
    });
  };

  const handleAttachmentRemove = (index: number) => {
    setAttachments((prev) => {
      const item = prev[index];
      if (item) {
        attachmentNamesRef.current.delete(item.id);
      }
      return prev.filter((_, i) => i !== index);
    });
  };

  const fileToBase64Content = (file: File): Promise<string> =>
    new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        const base64 = typeof reader.result === "string" ? reader.result : "";
        const commaIndex = base64.indexOf(",");
        resolve(commaIndex >= 0 ? base64.slice(commaIndex + 1) : base64);
      };
      reader.onerror = () =>
        reject(new Error(`Failed to read file: ${file.name}`));
      reader.readAsDataURL(file);
    });

  const handleSubmit = async (e?: FormEvent, bypassPii = false) => {
    e?.preventDefault();
    if (!projectId || isNavigatingAfterCreate || isCreatePending) return;
    if (isProjectLoading || isProjectFeaturesLoading) return;

    const titlePlain = htmlToPlainText(title).trim();
    const descriptionPlain = htmlToPlainText(description).trim();
    if (!titlePlain) {
      showError("Please enter a case title.");
      return;
    }
    if (titlePlain.length > 160) {
      return;
    }
    if (!descriptionPlain) {
      showError("Please enter a description.");
      return;
    }

    // Warn about PII in the title/description before submitting. When none is
    // found the guard proceeds immediately; otherwise the dialog opens and
    // "Post anyway" re-runs the submit with the check bypassed.
    if (!bypassPii) {
      piiGuard.checkBeforeSubmit(`${title}\n${description}`, () => {
        void handleSubmit(undefined, true);
      });
      return;
    }

    if (isSecurityReport && attachments.length === 0) {
      showError("Please attach at least one security report file.");
      return;
    }

    const deploymentMatch = resolveDeploymentMatch(
      deployment,
      projectDeployments,
      undefined,
    );
    const resolvedDeploymentId =
      deploymentMatch?.id ?? relatedCase?.deploymentId ?? "";
    if (!resolvedDeploymentId) {
      showError("Please select a deployment type.");
      return;
    }

    const productId = resolveProductId(product, allDeploymentProducts);
    if (!productId) {
      showError("Please select a product version.");
      return;
    }

    // Skip issue type and severity validation for security reports
    let issueTypeKey: number | undefined;
    let severityKey: number | undefined;

    if (!isSecurityReport) {
      issueTypeKey = resolveIssueTypeKey(issueType, filters?.issueTypes);
      if (!issueTypeKey) {
        showError("Please select an issue type.");
        return;
      }
      const parsedSeverity = parseInt(severity, 10);
      if (Number.isNaN(parsedSeverity)) {
        showError("Please select a severity.");
        return;
      }
      severityKey = parsedSeverity;
    }

    if (isSubmittingRef.current) return;

    let inlineAttachments: Array<{ file: string; name: string }> | undefined;
    if (isSecurityReport) {
      isSubmittingRef.current = true;
      setIsPreparingAttachments(true);
      const attachmentsSnapshot = attachments;
      try {
        inlineAttachments = await Promise.all(
          attachmentsSnapshot.map(async (item) => ({
            file: await fileToBase64Content(item.file),
            name: attachmentNamesRef.current.get(item.id) || item.file.name,
          })),
        );
      } catch (error) {
        logger.error("Failed to read attachment file(s)", error);
        showError(
          "We couldn't read one or more attachments. Please try again.",
        );
        return;
      } finally {
        isSubmittingRef.current = false;
        setIsPreparingAttachments(false);
      }
    }

    const payload: CreateCaseRequest = {
      type: isSecurityReport
        ? CaseType.SECURITY_REPORT_ANALYSIS
        : CaseType.DEFAULT_CASE,
      deploymentId: String(resolvedDeploymentId),
      description,
      issueTypeKey,
      deployedProductId: String(productId),
      projectId,
      severityKey,
      title,
      ...(relatedCase?.relatedCaseId && {
        relatedCaseId: relatedCase.relatedCaseId,
      }),
      ...(conversationId && {
        conversationId,
      }),
      ...(watchList.length > 0 && { watchList }),
      ...(inlineAttachments && { attachments: inlineAttachments }),
    };

    postCase(payload, {
      onSuccess: async (data) => {
        setIsNavigatingAfterCreate(true);
        const caseId = data.id;
        showSuccess("Case created successfully");
        const createdCase = data as {
          isSecurityReport?: boolean;
          reportType?: string;
          type?: string | { id?: string | null; label?: string | null } | null;
        };
        const isCreatedSecurityReport = isCreatedCaseSecurityReport(
          createdCase,
          isSecurityReport,
        );

        const uploadAttachments = async (): Promise<string[]> => {
          const failed: string[] = [];
          for (const item of attachments) {
            const attachmentName =
              attachmentNamesRef.current.get(item.id) || item.file.name;
            try {
              const content = await fileToBase64Content(item.file);
              await postAttachments.mutateAsync({
                caseId,
                body: {
                  name: attachmentName,
                  type: item.file.type || "application/octet-stream",
                  content,
                },
              });
            } catch (error) {
              logger.error(
                `Failed to upload attachment ${attachmentName}`,
                error,
              );
              failed.push(attachmentName);
            }
          }
          return failed;
        };

        let failedAttachmentNames: string[] = [];
        let attachmentsStillUploading = false;
        if (!isSecurityReport && attachments.length > 0) {
          setIsPreparingAttachments(true);
          const uploadPromise = uploadAttachments();
          const timedOut = await Promise.race([
            uploadPromise.then(() => false),
            new Promise<boolean>((resolve) =>
              setTimeout(() => resolve(true), ATTACHMENT_UPLOAD_WAIT_MS),
            ),
          ]);
          if (timedOut) {
            attachmentsStillUploading = true;
            void uploadPromise.then((failed) => {
              if (failed.length > 0) {
                showError(
                  `Failed to upload: ${failed.join(", ")}. You can retry from the Attachments tab.`,
                );
              }
              // no-op on success: the case-created confirmation was already shown above
            });
          } else {
            setIsPreparingAttachments(false);
            failedAttachmentNames = await uploadPromise;
          }
        }

        // Fire-and-forget: these hit the same backend origin as the (possibly
        // still in-flight) attachment upload, so awaiting them here could queue
        // behind it under the browser's per-origin connection limit and delay
        // navigation well past the attachment wait window above.
        if (projectId) {
          triggerPostCreationApiCalls(
            authFetch,
            projectId,
            CaseType.DEFAULT_CASE,
          ).catch((error) => {
            logger.error("Failed to trigger post-creation API calls", error);
          });
          refreshCaseQueriesAfterCreation(
            queryClient,
            projectId,
            CaseType.DEFAULT_CASE,
          ).catch((error) => {
            logger.error("Failed to refresh case queries after creation", error);
          });
        }

        // Clean up sessionStorage safely
        try {
          sessionStorage.removeItem(STORAGE_KEY);
          sessionStorage.removeItem(CONVERSATION_ID_STORAGE_KEY);
        } catch (e) {
          logger.error(
            "Failed to cleanup sessionStorage after case creation",
            e,
          );
        }

        // Refetch security vulnerabilities if this was a security report
        if (isCreatedSecurityReport) {
          navigate(
            `/projects/${projectId}/security-center/security-report-analysis/${caseId}?tab=${SecurityTabId.VULNERABILITIES}`,
          );
        } else {
          navigate(`/projects/${projectId}/support/cases/${caseId}`);
        }

        if (!attachmentsStillUploading && failedAttachmentNames.length > 0) {
          showError(
            `Failed to upload: ${failedAttachmentNames.join(", ")}. You can retry from the Attachments tab.`,
          );
        }
      },
      onError: (error) => {
        setIsNavigatingAfterCreate(false);
        const msg =
          error?.message?.trim() ||
          "We couldn't create your case. Please check required fields and try again.";
        showError(msg);
      },
    });
  };

  const extraProductOptions = useMemo(() => {
    const raw = classificationProductLabel?.trim() ?? "";
    if (!raw || isUnknownPlaceholderProductLabel(raw)) {
      return [];
    }
    if (
      !shouldAddClassificationProductToOptions(
        classificationProductLabel,
        sortedBaseProductOptions,
      )
    ) {
      return [];
    }
    return [classificationProductLabel];
  }, [classificationProductLabel, sortedBaseProductOptions]);

  // "Auto detected" means the AI classification suggested this value - it
  // is misleading for a singleton auto-select (there was nothing to
  // detect, it was just the only option), so that case intentionally
  // shows no chip at all rather than reusing this one.
  const isProductAutoDetected =
    !noAiMode &&
    !!classificationProductLabel?.trim() &&
    !!product?.trim() &&
    !isProductManuallySet;

  const isDeploymentAutoDetected =
    !noAiMode && isDeploymentFromClassification && !isDeploymentManuallySet;

  const isIssueTypeAutoDetected = !noAiMode && isIssueTypeFromClassification;
  const isSeverityAutoDetected = !noAiMode && isSeverityFromClassification;

  const isDeploymentClassificationPending =
    !noAiMode &&
    !!classificationDeploymentLabel?.trim() &&
    !deployment?.trim() &&
    !isDeploymentManuallySet &&
    (deploymentsQuery.isLoading ||
      deploymentsQuery.isFetchingNextPage ||
      !!deploymentsQuery.hasNextPage);

  const isProductClassificationPending =
    !noAiMode &&
    !!classificationProductLabel?.trim() &&
    !product?.trim() &&
    !!selectedDeploymentId &&
    (deploymentProductsLoading || !!deploymentProductsQuery.hasNextPage);

  const sectionMetadata = {
    deploymentTypes: baseDeploymentOptions,
  };
  const isProductDropdownDisabled =
    !selectedDeploymentId || deploymentProductsLoading;

  // These mirror the loading/disabled flags handed to BasicInformationSection
  // below so the "no options configured" empty-state check stays consistent
  // between what the dropdown shows and whether the rest of the form is
  // blocked from submission.
  //
  // The *ClassificationPending flags only mean "still paging through
  // results looking for the AI-suggested match" - once at least one real
  // option has actually arrived, that search continuing in the background
  // must not keep masking the dropdown as loading. Without this guard, a
  // deployment/product added while classification's search was still
  // in-flight (e.g. right after using "Add Deployment"/"Add Product" from
  // an empty list) stayed hidden behind a loading skeleton until something
  // else (like re-picking the deployment) reset the classification state.
  const deploymentFieldLoading =
    isProjectLoading ||
    isDeploymentsLoading ||
    (isDeploymentClassificationPending && baseDeploymentOptions.length === 0);
  const productFieldLoading =
    (!!selectedDeploymentId && deploymentProductsLoading) ||
    (isProductClassificationPending && sortedBaseProductOptions.length === 0);

  const hasNoDeployments =
    !isPrimaryProductionOnly &&
    isDeploymentDropdownEmpty(baseDeploymentOptions, deploymentFieldLoading, false);
  // Same exclusion as hasNoDeployments above: these project types have no
  // self-service add-product affordance (see onAddProduct below), so
  // treating an empty product list as a blocking dead end here would leave
  // the customer with no way out.
  const hasNoProductsForDeployment =
    !isPrimaryProductionOnly &&
    isProductDropdownEmpty(
      sortedBaseProductOptions.length > 0,
      isProductDropdownDisabled,
      productFieldLoading,
    );
  // Nothing valid can be submitted while either dropdown is a dead end.
  const isFormBlockedByMissingOptions =
    isDeploymentSetupEnabled && (hasNoDeployments || hasNoProductsForDeployment);

  const handleAddDeployment = useCallback(() => {
    if (!projectId) return;
    // Always the combined deployment+product wizard: EVERY newly created
    // deployment starts with zero products of its own, whether it's the
    // project's first deployment or an additional one added via the
    // persistent "add another" affordance - there's no case where chaining
    // into the product step isn't the right next step.
    setIsDeploymentWizardOpen(true);
  }, [projectId]);

  const handleAddProduct = useCallback(() => {
    if (!projectId || !selectedDeploymentId) return;
    setIsAddProductModalOpen(true);
  }, [projectId, selectedDeploymentId]);

  const handleProductAdded = useCallback(() => {
    showSuccess("Product added successfully.");
  }, [showSuccess]);

  // The wizard's own usePostCreateDeployment mutation response returns the
  // newly created deployment's id directly, so its product step doesn't
  // need to wait on `selectedDeploymentId` resolving through a query
  // refetch the way the old two-dialog chaining did. All this callback
  // needs to do is make sure the case-creation form's own deployment
  // dropdown reflects the new deployment once the deployments list
  // refetches - the same singleton auto-select signal the previous
  // implementation relied on, just applied immediately instead of via a
  // watcher effect.
  const handleWizardDeploymentCreated = useCallback(
    (name: string) => {
      showSuccess("Deployment created successfully.");
      // Reuses the same "manual selection" path a dropdown change goes
      // through (resets product + both singleton-auto-select flags) rather
      // than a bare setDeployment: the user just explicitly created THIS
      // deployment, so it must win over whatever was selected before (even
      // a previous singleton auto-select) and the undo-guard above must not
      // later revert it just because the list now has more than one item.
      handleDeploymentChange(name);
    },
    [showSuccess, handleDeploymentChange],
  );

  const handleWizardProductAdded = useCallback(() => {
    showSuccess("Product added successfully.");
  }, [showSuccess]);

  const renderContent = () => (
    <Grid container spacing={3}>
      {/* left column - form content (full width when skipChat or no sidebar content) */}
      <Grid
        size={{
          xs: 12,
          md: skipChatMode || (!relatedCase && !conversationId) ? 12 : 8,
        }}
      >
        {/* case creation form */}
        <Box
          component="form"
          onSubmit={handleSubmit}
          sx={{ display: "flex", flexDirection: "column", gap: 3 }}
        >
          <BasicInformationSection
            project={projectDisplay}
            product={product}
            setProduct={handleProductChange}
            deployment={deployment}
            setDeployment={handleDeploymentChange}
            productOptionList={sortedBaseProductOptions}
            isProductAutoDetected={isProductAutoDetected}
            isDeploymentAutoDetected={isDeploymentAutoDetected}
            metadata={sectionMetadata}
            isDeploymentLoading={deploymentFieldLoading}
            isProductDropdownDisabled={isProductDropdownDisabled}
            isProductLoading={productFieldLoading}
            isRelatedCaseMode={noAiMode}
            extraProductOptions={extraProductOptions}
            isDeploymentDisabled={false}
            hideDeploymentField={isPrimaryProductionOnly}
            onAddDeployment={
              isDeploymentSetupEnabled && !isPrimaryProductionOnly
                ? handleAddDeployment
                : undefined
            }
            // Cloud Support / Cloud Evaluation Support projects are locked
            // to a single "Primary Production" deployment (the field itself
            // is hidden via hideDeploymentField below) - since we don't
            // offer these projects self-service deployment setup, offering
            // to add a product under that fixed deployment would be
            // inconsistent.
            onAddProduct={
              isDeploymentSetupEnabled && !isPrimaryProductionOnly
                ? handleAddProduct
                : undefined
            }
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

          {!isFormBlockedByMissingOptions && (
            <>
              <CaseDetailsSection
                title={title}
                setTitle={handleTitleChange}
                description={description}
                setDescription={handleDescriptionChange}
                issueType={issueType}
                setIssueType={handleIssueTypeChange}
                severity={severity}
                setSeverity={handleSeverityChange}
                isIssueTypeAutoDetected={isIssueTypeAutoDetected}
                isSeverityAutoDetected={isSeverityAutoDetected}
                isTitleFromChat={isTitleFromClassification}
                isDescriptionFromConversation={isDescriptionFromClassification}
                metadata={undefined}
                filters={filters}
                isLoading={isFiltersLoading}
                attachments={attachments.map((a) => a.file)}
                onAttachmentClick={handleAttachmentClick}
                onAttachmentRemove={handleAttachmentRemove}
                storageKey={
                  !relatedCase && projectId
                    ? `create-case-draft-${projectId}`
                    : undefined
                }
                isRelatedCaseMode={noAiMode}
                isTitleDisabled={false}
                relatedCaseNumber={relatedCase?.number ?? ""}
                isSecurityReport={isSecurityReport}
                excludeS0={excludeS0}
                isSeverityDisabled={forceSeverityS4}
              />

              <WatchListSection
                contacts={(projectContacts ?? []).filter(
                  (c) => c.isCsAdmin || c.isCsIntegrationUser || c.isPortalUser || !c.isSecurityContact,
                )}
                selectedEmails={watchList}
                onChange={setWatchList}
                isLoading={isContactsLoading}
              />

              {/* form actions container */}
              <Box sx={{ display: "flex", justifyContent: "right" }}>
                {/* submit button */}
                <Button
                  type="submit"
                  variant="contained"
                  startIcon={<CircleCheck size={18} />}
                  color="primary"
                  disabled={
                    isProjectLoading ||
                    isProjectFeaturesLoading ||
                    isFiltersLoading ||
                    isCreatePending ||
                    isNavigatingAfterCreate ||
                    isPreparingAttachments ||
                    !projectId ||
                    !selectedDeploymentId ||
                    deploymentProductsLoading ||
                    deploymentProductsError ||
                    isFormBlockedByMissingOptions
                  }
                >
                  {isPreparingAttachments
                    ? "Uploading attachments..."
                    : isCreatePending
                      ? isSecurityReport
                        ? "Submitting..."
                        : "Creating..."
                      : isNavigatingAfterCreate
                        ? "Opening case..."
                        : isSecurityReport
                          ? "Submit Security Report"
                          : relatedCase
                            ? "Create Related Case"
                            : "Create Support Case"}
                </Button>
              </Box>
            </>
          )}
        </Box>
      </Grid>

      {/* right column - sidebar (hidden when skipChat or no sidebar content) */}
      {!skipChatMode && (relatedCase || conversationId) && (
        <Grid size={{ xs: 12, md: 4 }}>
          {relatedCase ? (
            <RelatedCaseSummary
              number={relatedCase.number}
              title={relatedCase.title}
              description={relatedCase.description}
            />
          ) : (
            <ConversationSummary conversationId={conversationId} />
          )}
        </Grid>
      )}
    </Grid>
  );

  return (
    <Box sx={{ width: "100%", pt: 0, position: "relative" }}>
      {/* header section */}
      <CaseCreationHeader
        onBack={handleBack}
        hideAiChip={noAiMode || isSecurityReport}
        backLabel="Back"
        title={
          isSecurityReport
            ? "Submit Security Vulnerability Report for Analysis"
            : relatedCase
              ? "Create Related Case"
              : undefined
        }
        subtitle={
          isSecurityReport
            ? "Upload your security vulnerability report and provide details for analysis"
            : skipChatMode || relatedCase
              ? "Fill in the case details below and submit"
              : "Please review and edit the auto-populated information before submitting"
        }
      />

      {/* main content grid container */}
      {renderContent()}

      <UploadAttachmentModal
        open={isAttachmentModalOpen}
        onClose={() => setIsAttachmentModalOpen(false)}
        onSelect={handleSelectAttachment}
      />

      {projectId && (
        <AddProductModal
          open={isAddProductModalOpen}
          deploymentId={selectedDeploymentId}
          projectId={projectId}
          onClose={() => setIsAddProductModalOpen(false)}
          onSuccess={handleProductAdded}
          onError={showError}
        />
      )}

      {projectId && (
        <AddDeploymentWizardModal
          open={isDeploymentWizardOpen}
          projectId={projectId}
          onClose={() => setIsDeploymentWizardOpen(false)}
          onDeploymentCreated={handleWizardDeploymentCreated}
          onProductAdded={handleWizardProductAdded}
          onError={showError}
        />
      )}

      <PiiWarningDialog {...piiGuard.dialogProps} />
    </Box>
  );
}
