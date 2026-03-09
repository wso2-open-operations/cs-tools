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

import {
  Box,
  Button,
  FormControl,
  Grid,
  MenuItem,
  Paper,
  Select,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, CircleCheck } from "@wso2/oxygen-ui-icons-react";
import {
  useState,
  useMemo,
  useCallback,
  useEffect,
  useRef,
  type FormEvent,
  type JSX,
} from "react";
import { useNavigate, useParams } from "react-router";
import { useGetProjectDeployments } from "@api/useGetProjectDeployments";
import { useGetDeploymentsProducts } from "@api/useGetDeploymentsProducts";
import { useSearchCatalogs } from "@api/useSearchCatalogs";
import { useGetCatalogItemVariables } from "@api/useGetCatalogItemVariables";
import { usePostCase } from "@api/usePostCase";
import useGetProjectDetails from "@api/useGetProjectDetails";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import {
  getFirstEmptyRequiredField,
  isContextField,
  isDescriptionField,
} from "@utils/serviceRequestValidation";
import {
  getBaseDeploymentOptions,
  getBaseProductOptions,
  resolveDeploymentMatch,
  resolveProductId,
} from "@utils/caseCreation";
import { htmlToPlainText } from "@utils/richTextEditor";
import type { CreateServiceRequestPayload } from "@models/requests";
import CatalogSelector from "@components/support/service-requests/CatalogSelector";
import VariableFormFields from "@components/support/service-requests/VariableFormFields";
import UploadAttachmentModal from "@components/support/case-details/attachments-tab/UploadAttachmentModal";

/**
 * CreateServiceRequestPage - multi-step form to create a service request.
 * Flow: Select Deployment -> Select Product -> Select Catalog Item -> Fill Variables -> Submit.
 *
 * @returns {JSX.Element} The Create Service Request page.
 */
export default function CreateServiceRequestPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const { showLoader, hideLoader } = useLoader();
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();

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
  const [isAttachmentModalOpen, setIsAttachmentModalOpen] = useState(false);

  const { data: projectDetails, isLoading: isProjectLoading } =
    useGetProjectDetails(projectId || "");
  const { data: projectDeployments, isLoading: isDeploymentsLoading } =
    useGetProjectDeployments(projectId || "");

  const baseDeploymentOptions = getBaseDeploymentOptions(projectDeployments);
  const selectedDeploymentMatch = useMemo(
    () => resolveDeploymentMatch(deployment, projectDeployments, undefined),
    [deployment, projectDeployments],
  );
  const selectedDeploymentId = selectedDeploymentMatch?.id ?? "";

  const {
    data: deploymentProductsData,
    isLoading: deploymentProductsLoading,
  } = useGetDeploymentsProducts(selectedDeploymentId);

  const allDeploymentProducts = useMemo(
    () =>
      (deploymentProductsData ?? []).filter((item) =>
        item.product?.label?.trim(),
      ),
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

  const productId = resolveProductId(product, allDeploymentProducts);
  const { data: catalogsData, isLoading: isCatalogsLoading } =
    useSearchCatalogs(productId);
  const { data: variablesData, isLoading: isVariablesLoading } =
    useGetCatalogItemVariables(selectedCatalogId, selectedCatalogItemId);

  const selectedCatalogItemLabel = useMemo(() => {
    if (!selectedCatalogId || !selectedCatalogItemId || !catalogsData?.catalogs)
      return undefined;
    const catalog = catalogsData.catalogs.find((c) => c.id === selectedCatalogId);
    const item = catalog?.catalogItems?.find(
      (i) => i.id === selectedCatalogItemId,
    );
    return item?.label;
  }, [catalogsData, selectedCatalogId, selectedCatalogItemId]);

  const { mutate: postCase, isPending: isCreatePending } = usePostCase();

  const isInitialLoading =
    isProjectLoading || isDeploymentsLoading || !projectId;
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

  const handleVariableChange = useCallback((variableId: string, value: string) => {
    setVariableValues((prev) => ({ ...prev, [variableId]: value }));
  }, []);

  const handleAttachmentClick = () => setIsAttachmentModalOpen(true);

  const handleSelectAttachment = (file: File, attachmentName?: string) => {
    setAttachments((prev) => {
      const sig = `${file.name}-${file.size}-${file.lastModified}`;
      if (prev.some((a) => `${a.file.name}-${a.file.size}-${a.file.lastModified}` === sig))
        return prev;
      const id = `att-${++attachmentIdCounterRef.current}-${Date.now()}`;
      if (attachmentName?.trim()) attachmentNamesRef.current.set(id, attachmentName.trim());
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
      navigate(`/${projectId}/support/service-requests`);
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
    if (!projectId) return;

    const deploymentMatch = resolveDeploymentMatch(
      deployment,
      projectDeployments,
      undefined,
    );
    if (!deploymentMatch) {
      showError("Please select a deployment type.");
      return;
    }

    if (!productId) {
      showError("Please select a product version.");
      return;
    }

    if (!selectedCatalogId || !selectedCatalogItemId) {
      showError("Please select a request type (catalog item).");
      return;
    }

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
    if (firstEmpty) {
      showError(`Please fill in the required field: ${firstEmpty}`);
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
      } catch (err) {
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
      onSuccess: (data) => {
        const srNumber = (data as { number?: string }).number;
        showSuccess(
          srNumber
            ? `Service request ${srNumber} created successfully`
            : "Service request created successfully",
        );
        navigate(`/${projectId}/support/service-requests/${data.id}`);
      },
      onError: (error) => {
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
  const canSubmit =
    !!projectId &&
    !!selectedDeploymentId &&
    !!productId &&
    !!selectedCatalogId &&
    !!selectedCatalogItemId &&
    !isCreatePending;

  return (
    <Box sx={{ width: "100%", pt: 0, position: "relative" }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 2,
          mb: 3,
        }}
      >
        <Button
          variant="text"
          color="secondary"
          startIcon={<ArrowLeft size={18} />}
          onClick={handleBack}
          sx={{ borderRadius: 0 }}
        >
          Back
        </Button>
        <Typography variant="h5" fontWeight={600}>
          New Service Request
        </Typography>
      </Box>

      <Box
        component="form"
        onSubmit={handleSubmit}
        sx={{ display: "flex", flexDirection: "column", gap: 3 }}
      >
        <Paper variant="outlined" sx={{ p: 3, borderRadius: 0 }}>
          <Typography variant="h6" sx={{ mb: 2 }}>
            Basic Information
          </Typography>
          <Grid container spacing={3}>
            <Grid size={{ xs: 12 }}>
              <Typography variant="caption" sx={{ display: "block", mb: 1 }}>
                Project
              </Typography>
              <TextField
                fullWidth
                size="small"
                disabled
                value={projectDisplay}
                sx={{ "& .MuiOutlinedInput-root": { borderRadius: 0 } }}
              />
            </Grid>
            <Grid size={{ xs: 12 }}>
              <Typography variant="caption" sx={{ display: "block", mb: 1 }}>
                Deployment Type <Box component="span" sx={{ color: "warning.main" }}>*</Box>
              </Typography>
              {isDeploymentsLoading ? (
                <Skeleton variant="rounded" height={40} />
              ) : (
                <FormControl fullWidth size="small">
                  <Select
                    value={deployment}
                    onChange={(e) => handleDeploymentChange(e.target.value)}
                    displayEmpty
                    renderValue={(v) =>
                      v === "" ? "Select Deployment Type..." : v
                    }
                    sx={{ borderRadius: 0 }}
                  >
                    <MenuItem value="" disabled>
                      Select Deployment Type...
                    </MenuItem>
                    {baseDeploymentOptions.map((d) => (
                      <MenuItem key={d} value={d}>
                        {d}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              )}
            </Grid>
            <Grid size={{ xs: 12 }}>
              <Typography variant="caption" sx={{ display: "block", mb: 1 }}>
                Product Version <Box component="span" sx={{ color: "warning.main" }}>*</Box>
              </Typography>
              {deploymentProductsLoading ? (
                <Skeleton variant="rounded" height={40} />
              ) : (
                <FormControl
                  fullWidth
                  size="small"
                  disabled={isProductDropdownDisabled}
                >
                  <Select
                    value={product}
                    onChange={(e) => handleProductChange(e.target.value)}
                    displayEmpty
                    renderValue={(v) => {
                      if (v === "")
                        return isProductDropdownDisabled
                          ? "Select deployment first"
                          : "Select Product Version...";
                      const opt = sortedProductOptions.find((o) => o.id === v);
                      return opt?.label ?? v;
                    }}
                    sx={{ borderRadius: 0 }}
                  >
                    <MenuItem value="" disabled>
                      {isProductDropdownDisabled
                        ? "Select deployment first"
                        : "Select Product Version..."}
                    </MenuItem>
                    {sortedProductOptions.map((p) => (
                      <MenuItem key={p.id} value={p.id}>
                        {p.label}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              )}
            </Grid>
          </Grid>
        </Paper>

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
            onAttachmentAdd={(file, name) => handleSelectAttachment(file, name)}
          />
        )}

        <UploadAttachmentModal
          open={isAttachmentModalOpen}
          onClose={() => setIsAttachmentModalOpen(false)}
          onSelect={handleSelectAttachment}
        />

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5 }}>
          <Button
            variant="outlined"
            color="inherit"
            onClick={handleBack}
            sx={{ borderRadius: 0 }}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            variant="contained"
            color="primary"
            startIcon={<CircleCheck size={18} />}
            disabled={!canSubmit}
            sx={{ borderRadius: 0 }}
          >
            {isCreatePending ? "Creating..." : "Create Service Request"}
          </Button>
        </Box>
      </Box>
    </Box>
  );
}
