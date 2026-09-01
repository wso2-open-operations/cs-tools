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
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useCallback, type JSX } from "react";
import { useProductForm } from "@features/project-details/components/deployments/useProductForm";
import { ProductFormFields } from "@features/project-details/components/deployments/ProductFormFields";
import type { AddProductModalProps } from "@features/project-details/types/projectDetailsComponents";

/**
 * Modal for adding a WSO2 product to a deployment environment.
 * Product Name and Version come from paginated APIs; Description and optional metrics are user-entered.
 * The actual paginated form fields + mutation logic live in useProductForm/ProductFormFields, shared with
 * the add-deployment wizard's product step - this component only owns the dialog chrome and the
 * close-on-success behavior standalone callers rely on.
 *
 * @param {AddProductModalProps} props - open, deploymentId, projectId, onClose, optional onSuccess/onError.
 * @returns {JSX.Element} The add product modal.
 */
export default function AddProductModal({
  open,
  deploymentId,
  projectId,
  onClose,
  onSuccess,
  onError,
}: AddProductModalProps): JSX.Element {
  const productForm = useProductForm(deploymentId, projectId);
  const { isValid, isSubmitting, submit, resetForm } = productForm;

  const handleClose = useCallback(() => {
    resetForm();
    onClose();
  }, [onClose, resetForm]);

  const handleSubmit = useCallback(async () => {
    if (!isValid) return;

    try {
      await submit();
      handleClose();
      onSuccess?.();
    } catch (error) {
      onError?.(
        error instanceof Error ? error.message : "Failed to add product",
      );
    }
  }, [isValid, submit, handleClose, onSuccess, onError]);

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="add-product-dialog-title"
      aria-describedby="add-product-dialog-description"
    >
      <DialogTitle
        id="add-product-dialog-title"
        sx={{ pr: 6, position: "relative", pb: 0.5 }}
      >
        Add WSO2 Product
        <Typography
          id="add-product-dialog-description"
          variant="body2"
          color="text.secondary"
          sx={{ mt: 0.5, fontWeight: "normal", fontSize: "0.875rem" }}
        >
          Add a WSO2 product to this deployment environment.
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

      <DialogContent
        sx={{
          pt: 1,
          "& .MuiInputBase-input::placeholder": {
            color: "text.secondary",
            opacity: 1,
          },
        }}
      >
        <ProductFormFields {...productForm} isSubmitting={isSubmitting} />
      </DialogContent>

      <DialogActions
        sx={{ px: 3, pb: 3, pt: 1, justifyContent: "flex-end", gap: 1 }}
      >
        <Button
          variant="outlined"
          onClick={handleClose}
          disabled={isSubmitting}
        >
          Cancel
        </Button>
        {isSubmitting ? (
          <Button
            variant="contained"
            color="primary"
            startIcon={<CircularProgress color="inherit" size={16} />}
            disabled
          >
            Adding...
          </Button>
        ) : (
          <Button
            type="button"
            variant="contained"
            color="primary"
            onClick={handleSubmit}
            disabled={!isValid}
          >
            Add Product
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
