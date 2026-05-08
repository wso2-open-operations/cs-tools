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

import type { ProjectStatsResponse } from "@features/project-hub/types/projects";
import type { ProjectDetails } from "@features/project-hub/types/projects";
import type {
  DeploymentDocument,
  DeploymentProductItem,
  ProjectDeploymentItem,
  SelectedDeploymentProduct,
} from "@features/project-details/types/deployments";
import type { ProductUpdate } from "@features/project-details/types/products";
import type { CaseTimeCard } from "@features/usage-metrics/types/timeTracking";
import type { Contact } from "@features/project-details/types/projectDetails";

export type ProjectInformationCardProps = {
  project?: ProjectDetails;
  slaStatus: string;
  isLoading?: boolean;
  isError?: boolean;
};

export type ProjectStatisticsCardProps = {
  stats?: ProjectStatsResponse["projectStats"];
  isLoading?: boolean;
  isError?: boolean;
  isSidebarOpen?: boolean;
  showDeploymentsStat?: boolean;
  showServiceRequestStat?: boolean;
  showChangeRequestStat?: boolean;
  showSecurityReportStat?: boolean;
};

export type ProjectNameProps = {
  name: string;
  projectKey: string;
  isLoading?: boolean;
  isError?: boolean;
};

export type ProjectDescriptionProps = {
  description: string;
  isLoading?: boolean;
  isError?: boolean;
};

export type ProjectTypeRef = {
  id: string;
  label: string;
};

export type ProjectMetadataProps = {
  createdDate: string;
  type: ProjectTypeRef;
  supportTier: string;
  slaStatus: string;
  goLivePlanDate: string;
  onboardingStatus: string;
  hideOnboardingStatus?: boolean;
  isLoading?: boolean;
  isError?: boolean;
};

export type ProjectMetadataPrimaryRowProps = {
  createdDate: string;
  type: ProjectTypeRef;
  supportTier: string;
  isLoading?: boolean;
  isError?: boolean;
};

export type ProjectMetadataSecondaryRowProps = {
  slaStatus: string;
  goLivePlanDate: string;
  onboardingStatus: string;
  hideOnboardingStatus?: boolean;
  isLoading?: boolean;
  isError?: boolean;
};

export type SubscriptionDetailsProps = {
  startDate?: string | null;
  endDate?: string | null;
  isLoading?: boolean;
  isError?: boolean;
};

export type ContactRowProps = {
  contact: Contact;
};

export type ContactInfoCardProps = {
  project?: ProjectDetails;
  isLoading?: boolean;
  isError?: boolean;
};

export type ServiceHoursAllocationsCardProps = {
  project?: ProjectDetails | null;
  isLoading?: boolean;
  isError?: boolean;
};

export type ServiceHoursStatCardsProps = {
  project?: ProjectDetails | null;
  isLoading?: boolean;
  isError?: boolean;
};

export type ProjectDeploymentsProps = {
  projectId: string;
  showServiceRequest?: boolean;
};

export type DeploymentCardProps = {
  deployment: ProjectDeploymentItem;
  selectedProduct: SelectedDeploymentProduct | null;
  onToggleProductSelect: (deploymentId: string, productItemId: string) => void;
};

export type DeploymentCardToolbarProps = {
  onEdit: () => void;
  onDelete: () => void;
  isDeleteDisabled: boolean;
};

export type DeploymentCardLicenseFooterProps = {
  createdAtLabel: string;
  updatedAtLabel: string;
  onDownloadLicense: () => void;
  isDownloading: boolean;
};

export type DeploymentHeaderProps = {
  count?: number;
  onAddClick?: () => void;
  isLoading?: boolean;
};

export type DeploymentProductListProps = {
  deploymentId: string;
  projectId: string;
  selectedProduct: SelectedDeploymentProduct | null;
  onToggleProductSelect: (deploymentId: string, productItemId: string) => void;
};

export type DeploymentProductItemRowProps = {
  item: DeploymentProductItem;
  deploymentId: string;
  isSelected: boolean;
  onToggleSelect: () => void;
  onEdit: () => void;
  onDelete: (item: DeploymentProductItem) => void;
  isDeleting: boolean;
};

export type DeploymentDocumentListProps = {
  deploymentId: string;
};

export type DeploymentDocumentRowProps = {
  doc: DeploymentDocument;
  deploymentId: string;
  currentUserEmail: string;
  onError: (message: string) => void;
};

export type AddProductModalProps = {
  open: boolean;
  deploymentId: string;
  projectId: string;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
};

export type ManageProductModalProps = {
  open: boolean;
  deploymentId: string;
  product: DeploymentProductItem | null;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
};

export type DeleteProductModalProps = {
  open: boolean;
  product: DeploymentProductItem | null;
  onClose: () => void;
  onConfirm: () => void;
  isDeleting?: boolean;
};

export type AddDeploymentModalProps = {
  open: boolean;
  projectId: string;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
};

export type EditDeploymentModalProps = {
  open: boolean;
  deployment: ProjectDeploymentItem | null;
  projectId: string;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
};

export type DeleteDeploymentModalProps = {
  open: boolean;
  deployment: ProjectDeploymentItem | null;
  onClose: () => void;
  onConfirm: () => void;
  isDeleting?: boolean;
};

export type EditDeploymentAttachmentModalProps = {
  open: boolean;
  document: DeploymentDocument | null;
  deploymentId: string;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
};

export type UpdateHistorySaveAction = "add" | "delete" | "edit";

export type UpdateHistoryTabFormState = {
  canAdd: boolean;
  isSaving: boolean;
  saveAction: UpdateHistorySaveAction | null;
  handleAdd: () => void;
};

export type UpdateHistoryTabProps = {
  updates: ProductUpdate[];
  productName: string;
  productVersion: string;
  isLoading?: boolean;
  onSaveUpdates: (updates: ProductUpdate[]) => Promise<void>;
  onFormStateChange?: (state: UpdateHistoryTabFormState | null) => void;
};

export type UpdateHistoryFormData = {
  updateLevel: string;
  date: string;
  details: string;
};

export type UpdateHistoryTimelineItemProps = {
  update: ProductUpdate;
  isEditing: boolean;
  onEdit: () => void;
  onDelete: () => void;
  onSave: (edited: ProductUpdate) => void;
  onCancelEdit: () => void;
  formatDate: (dateStr: string) => string;
  isSaving: boolean;
  availableUpdateLevels: number[];
  showUpdateLevelSkeleton: boolean;
  showEditFormSkeleton: boolean;
};

export type ProjectTimeTrackingProps = {
  projectId: string;
  project?: ProjectDetails | null;
  isProjectLoading?: boolean;
  isProjectError?: boolean;
};

export type TimeTrackingCardProps = {
  card: CaseTimeCard;
};

export type TimeCardsDateFilterProps = {
  startDate: string;
  endDate: string;
  onStartDateChange: (value: string) => void;
  onEndDateChange: (value: string) => void;
  onClear?: () => void;
  hasFilters?: boolean;
};
