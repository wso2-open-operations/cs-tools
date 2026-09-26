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
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  IconButton,
  InputLabel,
  List,
  ListItem,
  MenuItem,
  Select,
  Skeleton,
  Step,
  StepLabel,
  Stepper,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useState,
  type ChangeEvent,
  type JSX,
} from "react";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import { usePostCreateDeployment } from "@features/project-details/api/usePostCreateDeployment";
import useGetProjectFilters from "@api/useGetProjectFilters";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import ErrorBanner from "@components/error-banner/ErrorBanner";
import {
  useProductForm,
  type AddedProductSummary,
} from "@features/project-details/components/deployments/useProductForm";
import { ProductFormFields } from "@features/project-details/components/deployments/ProductFormFields";

const INITIAL_DEPLOYMENT_FORM = {
  name: "",
  deploymentTypeKey: "",
  description: "",
};

const WIZARD_STEPS = ["Deployment details", "Add products"] as const;

export type AddDeploymentWizardModalProps = {
  open: boolean;
  projectId: string;
  onClose: () => void;
  /**
   * Fired as soon as the deployment is created (before any product is added)
   * with the deployment's name, so the case-creation form can select it in
   * its own deployment dropdown once the deployments list refetches.
   */
  onDeploymentCreated?: (name: string) => void;
  /** Fired once per successfully added product. */
  onProductAdded?: () => void;
  onError?: (message: string) => void;
};

/**
 * Single continuous "add my first deployment" wizard: deployment details,
 * then one or more products for that deployment, without closing and
 * reopening separate dialogs in between. Used only for the case where a
 * project has zero deployments - the persistent "add another deployment"
 * affordance keeps using the plain AddDeploymentModal with no forced
 * product step.
 *
 * Step 2 reuses useProductForm/ProductFormFields - the same paginated
 * product/version search logic as the standalone AddProductModal - so
 * there is one source of truth for that behavior.
 *
 * @param {AddDeploymentWizardModalProps} props - open, projectId, onClose, and per-step success/error callbacks.
 * @returns {JSX.Element} The two-step add-deployment wizard dialog.
 */
export default function AddDeploymentWizardModal({
  open,
  projectId,
  onClose,
  onDeploymentCreated,
  onProductAdded,
  onError,
}: AddDeploymentWizardModalProps): JSX.Element {
  const [step, setStep] = useState<1 | 2>(1);
  const [deploymentForm, setDeploymentForm] = useState(
    INITIAL_DEPLOYMENT_FORM,
  );
  const [filtersErrorBanner, setFiltersErrorBanner] = useState(false);
  const [createdDeployment, setCreatedDeployment] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [addedProducts, setAddedProducts] = useState<AddedProductSummary[]>(
    [],
  );

  const createDeployment = usePostCreateDeployment(projectId);
  const {
    data: filtersData,
    isLoading: isFiltersLoading,
    isError: isFiltersError,
  } = useGetProjectFilters(projectId);
  const deploymentTypes = filtersData?.deploymentTypes ?? [];

  // Always called (rules of hooks) - only meaningful once step 2 is reached
  // and createdDeployment.id is known; the product select queries underneath
  // are harmless to run with an empty deploymentId since submit() checks it.
  const productForm = useProductForm(createdDeployment?.id ?? "", projectId);

  useEffect(() => {
    if (isFiltersError) {
      setFiltersErrorBanner(true);
    }
  }, [isFiltersError]);

  const { resetForm: resetProductForm } = productForm;

  const resetWizard = useCallback(() => {
    setStep(1);
    setDeploymentForm(INITIAL_DEPLOYMENT_FORM);
    setFiltersErrorBanner(false);
    setCreatedDeployment(null);
    setAddedProducts([]);
    resetProductForm();
  }, [resetProductForm]);

  const handleClose = useCallback(() => {
    resetWizard();
    onClose();
  }, [onClose, resetWizard]);

  const isDeploymentFormValid =
    deploymentForm.name.trim() !== "" &&
    deploymentForm.deploymentTypeKey !== "" &&
    deploymentForm.description.trim() !== "";

  const handleDeploymentTextChange =
    (field: keyof typeof INITIAL_DEPLOYMENT_FORM) =>
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setDeploymentForm((prev) => ({ ...prev, [field]: event.target.value }));
    };

  const handleDeploymentTypeChange = (event: SelectChangeEvent<string>) => {
    setDeploymentForm((prev) => ({
      ...prev,
      deploymentTypeKey: event.target.value,
    }));
  };

  const handleCreateDeployment = useCallback(() => {
    if (!isDeploymentFormValid) return;

    const name = deploymentForm.name.trim();
    createDeployment.mutate(
      {
        name,
        description: deploymentForm.description.trim(),
        deploymentTypeKey: Number(deploymentForm.deploymentTypeKey),
      },
      {
        onSuccess: (data) => {
          setCreatedDeployment({ id: data.id, name });
          setStep(2);
          onDeploymentCreated?.(name);
        },
        onError: (error: Error) => {
          onError?.(error.message ?? "Failed to create deployment.");
        },
      },
    );
  }, [
    isDeploymentFormValid,
    deploymentForm,
    createDeployment,
    onDeploymentCreated,
    onError,
  ]);

  const { isValid: isProductFormValid, submit: submitProduct } = productForm;

  const handleAddAnotherProduct = useCallback(async () => {
    if (!isProductFormValid) return;
    try {
      const summary = await submitProduct();
      setAddedProducts((prev) => [...prev, summary]);
      resetProductForm();
      onProductAdded?.();
    } catch (error) {
      onError?.(
        error instanceof Error ? error.message : "Failed to add product",
      );
    }
  }, [
    isProductFormValid,
    submitProduct,
    resetProductForm,
    onProductAdded,
    onError,
  ]);

  const handleDone = useCallback(async () => {
    if (isProductFormValid) {
      try {
        const summary = await submitProduct();
        setAddedProducts((prev) => [...prev, summary]);
        onProductAdded?.();
      } catch (error) {
        onError?.(
          error instanceof Error ? error.message : "Failed to add product",
        );
        return;
      }
    }
    handleClose();
  }, [isProductFormValid, submitProduct, onProductAdded, onError, handleClose]);

  return (
    <>
      {isFiltersError && filtersErrorBanner && (
        <ErrorBanner
          message="Failed to load deployment types. Please close and try again."
          onClose={() => setFiltersErrorBanner(false)}
        />
      )}

      <Dialog
        open={open}
        onClose={handleClose}
        maxWidth="sm"
        fullWidth
        aria-labelledby="add-deployment-wizard-title"
        aria-describedby="add-deployment-wizard-description"
      >
        <DialogTitle
          id="add-deployment-wizard-title"
          sx={{ pr: 6, position: "relative", pb: 0.5 }}
        >
          {step === 1 ? "Add New Deployment" : "Add WSO2 Product"}
          <Typography
            id="add-deployment-wizard-description"
            variant="body2"
            color="text.secondary"
            sx={{ mt: 0.5, fontWeight: "normal", fontSize: "0.875rem" }}
          >
            {step === 1
              ? "Create a new deployment environment for your project."
              : `Add one or more WSO2 products to "${createdDeployment?.name}".`}
          </Typography>
          <IconButton
            aria-label="close"
            onClick={handleClose}
            sx={{ position: "absolute", right: 12, top: 12 }}
            size="small"
          >
            <X size={20} aria-hidden />
          </IconButton>
        </DialogTitle>

        <Box sx={{ px: 3, pt: 1 }}>
          <Stepper activeStep={step - 1} alternativeLabel>
            {WIZARD_STEPS.map((label) => (
              <Step key={label}>
                <StepLabel>{label}</StepLabel>
              </Step>
            ))}
          </Stepper>
        </Box>

        <DialogContent sx={{ pt: 1 }}>
          {step === 1 ? (
            <>
              <TextField
                id="deployment-name"
                label="Deployment Name *"
                placeholder="e.g., Production US-East"
                value={deploymentForm.name}
                onChange={handleDeploymentTextChange("name")}
                fullWidth
                size="small"
                sx={{ mt: 3, mb: 2 }}
                disabled={createDeployment.isPending}
              />

              {isFiltersLoading ? (
                <Skeleton
                  variant="rounded"
                  height={40}
                  sx={{ mb: 2, borderRadius: 1 }}
                />
              ) : (
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 0.5,
                    mb: 2,
                  }}
                >
                  <FormControl fullWidth size="small">
                    <InputLabel id="deployment-type-label">
                      Deployment Type *
                    </InputLabel>
                    <Select<string>
                      labelId="deployment-type-label"
                      id="deployment-type"
                      value={deploymentForm.deploymentTypeKey}
                      label="Deployment Type *"
                      onChange={handleDeploymentTypeChange}
                      disabled={createDeployment.isPending || isFiltersError}
                    >
                      {deploymentTypes.map(({ id, label }) => (
                        <MenuItem key={id} value={id}>
                          {label}
                        </MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                  {isFiltersError && (
                    <ErrorIndicator entityName="deployment types" size="small" />
                  )}
                </Box>
              )}

              <TextField
                id="deployment-description"
                label="Description *"
                placeholder="Describe this deployment environment..."
                value={deploymentForm.description}
                onChange={handleDeploymentTextChange("description")}
                fullWidth
                size="small"
                multiline
                rows={3}
                disabled={createDeployment.isPending}
              />
            </>
          ) : (
            <>
              {addedProducts.length > 0 && (
                <Box sx={{ mb: 2 }}>
                  <Typography variant="subtitle2" sx={{ mb: 1 }}>
                    Products added so far ({addedProducts.length})
                  </Typography>
                  <List dense disablePadding data-testid="added-products-list">
                    {addedProducts.map((p, index) => (
                      <ListItem
                        key={`${p.productId}-${p.versionId}-${index}`}
                        disableGutters
                        sx={{ py: 0.25 }}
                      >
                        <Chip
                          size="small"
                          label={`${p.productLabel} - ${p.versionLabel}`}
                        />
                      </ListItem>
                    ))}
                  </List>
                  <Divider sx={{ mt: 1.5 }} />
                </Box>
              )}

              <ProductFormFields
                {...productForm}
                isSubmitting={productForm.isSubmitting}
              />
            </>
          )}
        </DialogContent>

        <DialogActions
          sx={{ px: 3, pb: 3, pt: 1, justifyContent: "flex-end", gap: 1 }}
        >
          {step === 1 ? (
            <>
              <Button
                variant="outlined"
                onClick={handleClose}
                disabled={createDeployment.isPending}
              >
                Cancel
              </Button>
              {createDeployment.isPending ? (
                <Button
                  variant="contained"
                  color="primary"
                  startIcon={<CircularProgress color="inherit" size={16} />}
                  disabled
                >
                  Creating...
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="contained"
                  color="primary"
                  onClick={handleCreateDeployment}
                  disabled={!isDeploymentFormValid || isFiltersError}
                >
                  Next: Add Products
                </Button>
              )}
            </>
          ) : (
            <>
              <Button
                variant="outlined"
                onClick={handleAddAnotherProduct}
                disabled={!productForm.isValid || productForm.isSubmitting}
              >
                Add another product
              </Button>
              <Button
                type="button"
                variant="contained"
                color="primary"
                onClick={handleDone}
                disabled={productForm.isSubmitting}
                startIcon={
                  productForm.isSubmitting ? (
                    <CircularProgress color="inherit" size={16} />
                  ) : undefined
                }
              >
                Done
              </Button>
            </>
          )}
        </DialogActions>
      </Dialog>
    </>
  );
}
