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

import type { ReactNode, RefObject } from "react";
import type { SxProps, Theme } from "@wso2/oxygen-ui";
import type { ProjectCasesStats } from "@features/support/types/cases";
import type { ProjectSupportStats } from "@features/project-hub/types/projects";
import type {
  CaseAttachment,
  CaseComment,
  CaseDetails,
  CaseListItem,
  CaseMetadataResponse,
} from "@features/support/types/cases";
import type { CallRequest } from "@features/support/types/calls";
import type {
  AllConversationsFilterValues,
  ChatHistoryItem,
  Message,
  Recommendation,
} from "@features/support/types/conversations";
import type { ChatAction } from "@features/support/constants/supportConstants";
import type { ProductVersionOption } from "@features/support/types/caseCreationOptions";
import type { AssignedEngineerValue } from "@features/support/utils/support";

/** Header layout variant for case / SR / engagement detail shells. */
export type CaseDetailsHeaderVariant =
  | "default"
  | "engagement"
  | "serviceRequest";

export type CaseDetailsHeaderProps = {
  wso2CaseId?: string | null;
  caseNumber: string | null | undefined;
  title: string | null | undefined;
  severityLabel: string | null | undefined;
  statusLabel: string | null | undefined;
  assignedEngineerLabel?: string | null;
  statusChipIcon: ReactNode;
  statusChipSx: Record<string, unknown>;
  isLoading?: boolean;
  showSeverityChip?: boolean;
  showStatusChip?: boolean;
  variant?: CaseDetailsHeaderVariant;
};

export type OutstandingCasesListProps = {
  cases: CaseListItem[];
  isLoading?: boolean;
  isError?: boolean;
  onCaseClick?: (caseItem: CaseListItem) => void;
  useChangeRequestColors?: boolean;
};

export type CaseDetailsDetailsPanelProps = {
  data: CaseDetails | undefined;
  isError: boolean;
  error?: unknown;
  isEngagement?: boolean;
  isServiceRequest?: boolean;
};

export type CaseDetailsSectionProps = {
  title?: string;
  setTitle?: (value: string) => void;
  description?: string;
  setDescription?: (value: string) => void;
  issueType?: string;
  setIssueType?: (value: string) => void;
  severity?: string;
  setSeverity?: (value: string) => void;
  metadata?: unknown;
  filters?: CaseMetadataResponse;
  isLoading?: boolean;
  storageKey?: string;
  extraIssueTypes?: string[];
  extraSeverityLevels?: { id: string; label: string; description?: string }[];
  attachments?: File[];
  onAttachmentClick?: () => void;
  onAttachmentRemove?: (index: number) => void;
  isRelatedCaseMode?: boolean;
  isTitleDisabled?: boolean;
  relatedCaseNumber?: string;
  isSecurityReport?: boolean;
  excludeS0?: boolean;
  isSeverityDisabled?: boolean;
};

export type ChatMessageCardProps = {
  htmlContent: string;
  markdownContent?: string;
  renderAsMarkdown?: boolean;
  isCurrentUser: boolean;
  primaryBg: string;
  onImageClick?: (src: string) => void;
};

export type AllConversationsSearchBarProps = {
  searchTerm: string;
  onSearchChange: (value: string) => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
  filters: AllConversationsFilterValues;
  filterMetadata: CaseMetadataResponse | undefined;
  onFilterChange: (field: string, value: string) => void;
  onClearFilters: () => void;
};

export type AllConversationsFiltersProps = {
  filters: AllConversationsFilterValues;
  filterMetadata: CaseMetadataResponse | undefined;
  onFilterChange: (field: string, value: string) => void;
};

export type AllCasesStatCardsProps = {
  isLoading: boolean;
  isError?: boolean;
  stats: ProjectCasesStats | undefined;
  statEntityName?: string;
};

export type CaseCreationHeaderProps = {
  onBack?: () => void;
  hideAiChip?: boolean;
  backLabel?: string;
  subtitle?: string;
  title?: string;
};

export type RelatedCaseSummaryProps = {
  number: string;
  title: string;
  description: string;
};

export type ConversationSummaryProps = {
  conversationId?: string;
};

export type BasicInformationSectionProps = {
  project?: string;
  product?: string;
  setProduct?: (value: string) => void;
  deployment?: string;
  setDeployment?: (value: string) => void;
  productOptionList?: ProductVersionOption[];
  isProductAutoDetected?: boolean;
  isDeploymentAutoDetected?: boolean;
  metadata?: { deploymentTypes?: string[]; products?: string[] };
  isDeploymentLoading?: boolean;
  isProductDropdownDisabled?: boolean;
  isProductLoading?: boolean;
  extraDeploymentOptions?: string[];
  extraProductOptions?: string[];
  isRelatedCaseMode?: boolean;
  isDeploymentDisabled?: boolean;
  hideDeploymentField?: boolean;
  onLoadMoreDeployments?: () => void;
  hasMoreDeployments?: boolean;
  isFetchingMoreDeployments?: boolean;
  onLoadMoreProducts?: () => void;
  hasMoreProducts?: boolean;
  isFetchingMoreProducts?: boolean;
};

export type ImageFullscreenModalProps = {
  open: boolean;
  imageSrc: string | null;
  onClose: () => void;
};

export type CommentBubbleProps = {
  comment: CaseComment;
  isCurrentUser: boolean;
  primaryBg: string;
  hideAvatar?: boolean;
  onImageClick?: (src: string) => void;
  userDetails?: {
    email?: string;
    firstName?: string;
    lastName?: string;
  } | null;
};

export type CaseDetailsActivityPanelProps = {
  projectId: string;
  caseId: string;
  conversationId?: string | null;
  caseCreatedOn?: string | null;
  focusMode?: boolean;
  caseStatus?: string | null;
};

export type ActivityCommentInputProps = {
  caseId: string;
  caseStatus?: string | null;
};

export type ActivityContentProps = {
  commentsToShow: CaseComment[];
  caseCreatedOn?: string | null;
  currentUserEmail: string;
  primaryBg: string;
  hideAvatar?: boolean;
  userDetails?: {
    email?: string;
    firstName?: string;
    lastName?: string;
  } | null;
  onImageClick?: (src: string) => void;
};

export type UploadAttachmentModalProps = {
  open: boolean;
  caseId?: string;
  deploymentId?: string;
  onClose: () => void;
  onSuccess?: () => void;
  onSelect?: (file: File, attachmentName?: string) => void;
};

export type UploadAttachmentDropZoneProps = {
  dragOver: boolean;
  onDrop: (e: React.DragEvent) => void;
  onDragOver: (e: React.DragEvent) => void;
  onDragLeave: () => void;
  onChooseFile: () => void;
};

export type SelectedFileDisplayProps = {
  fileName: string;
  fileType: string;
};

export type EditCaseAttachmentModalProps = {
  open: boolean;
  attachment: CaseAttachment | null;
  caseId: string;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
};

export type DeleteAttachmentModalProps = {
  open: boolean;
  attachmentName: string | null;
  onClose: () => void;
  onConfirm: () => void;
  isDeleting?: boolean;
};

export type CaseDetailsAttachmentsPanelProps = {
  caseId: string;
  isCaseClosed?: boolean;
};

export type AttachmentListItemProps = {
  attachment: CaseAttachment;
  onDownload: (attachment: CaseAttachment) => void;
  onDelete?: (attachment: CaseAttachment) => void;
  onEdit?: (attachment: CaseAttachment) => void;
  deleteDisabled?: boolean;
  deleteTooltip?: string;
  hideDescription?: boolean;
  isDownloadLoading?: boolean;
};

export type DeleteCallRequestModalProps = {
  open: boolean;
  call: CallRequest | null;
  userTimeZone?: string;
  onClose: () => void;
  onConfirm: (reason: string) => void;
  isDeleting?: boolean;
};

export type CallsPanelProps = {
  projectId: string;
  caseId: string;
  error?: unknown;
  isCaseClosed?: boolean;
  caseStatusLabel?: string;
  caseSeverityId?: string | null;
};

export type CallsEmptyStateProps = {
  action?: React.ReactNode;
};

export type CallRequestListProps = {
  requests: CallRequest[];
  userTimeZone?: string;
  onEditClick?: (call: CallRequest) => void;
  onDeleteClick?: (call: CallRequest) => void;
  onApproveClick?: (call: CallRequest) => void;
  onRejectClick?: (call: CallRequest) => void;
};

export type CallRequestCardProps = {
  call: CallRequest;
  userTimeZone?: string;
  onEditClick?: (call: CallRequest) => void;
  onDeleteClick?: (call: CallRequest) => void;
  onApproveClick?: (call: CallRequest) => void;
  onRejectClick?: (call: CallRequest) => void;
};

export type RequestCallModalProps = {
  open: boolean;
  projectId: string;
  caseId: string;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
  editCall?: CallRequest;
  userTimeZone?: string;
  severityAllocationMinutes?: number;
};

export type ApproveCallRequestModalProps = {
  open: boolean;
  call: CallRequest | null;
  projectId: string;
  caseId: string;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
  userTimeZone?: string;
  severityAllocationMinutes?: number;
  approveStateKey?: number;
};

export type RejectCallRequestModalProps = {
  open: boolean;
  call: CallRequest | null;
  onClose: () => void;
  onConfirm: (reason: string) => void;
  isRejecting?: boolean;
};

export type MissingTimezoneDialogVariant = "informational" | "required";

export type MissingTimezoneDialogProps = {
  open: boolean;
  onClose: () => void;
  onSetTimeZone: () => void;
  variant?: MissingTimezoneDialogVariant;
};

export type CaseDetailsContentProps = {
  data: CaseDetails | undefined;
  isLoading: boolean;
  isError: boolean;
  error?: unknown;
  caseId: string;
  onBack: () => void;
  onOpenRelatedCase?: () => void;
  projectId?: string;
  hideActionRow?: boolean;
  isServiceRequest?: boolean;
};

export type CaseDetailsCardProps = {
  title: string;
  icon: ReactNode;
  children: ReactNode;
  rightAction?: ReactNode;
};

export type AssignedEngineerDisplayProps = {
  assignedEngineer: AssignedEngineerValue;
};

export type CaseDetailsTabsProps = {
  value: number;
  onChange: (_e: unknown, newValue: number) => void;
  focusMode?: boolean;
  onFocusModeToggle?: () => void;
  attachmentCount?: number;
  callCount?: number;
  hideCallsTab?: boolean;
  hideKnowledgeBaseTab?: boolean;
  knowledgeBaseCount?: number;
  knowledgeBaseCountLoading?: boolean;
  hideRelatedChangeRequestsTab?: boolean;
};

export type CaseDetailsTabPanelsProps = {
  panelIndex: number;
  caseId: string;
  data?: CaseDetails;
  isError?: boolean;
  error?: unknown;
  projectId?: string;
  focusMode?: boolean;
  isEngagement?: boolean;
  isServiceRequest?: boolean;
};

export type CaseDetailsSkeletonProps = {
  hideActionRow?: boolean;
  hideAssignedEngineer?: boolean;
  headerVariant?: CaseDetailsHeaderVariant;
};

export type CaseDetailsBackButtonProps = {
  onClick: () => void;
  sx?: SxProps<Theme>;
};

export type CaseDetailsActionRowProps = {
  assignedEngineer: AssignedEngineerValue;
  engineerInitials: string;
  statusLabel?: string | null;
  closedOn?: string | null;
  onOpenRelatedCase?: () => void;
  projectId?: string;
  caseId?: string;
  isLoading?: boolean;
  hideAssignedEngineer?: boolean;
  restrictToCloseOnly?: boolean;
};

export type CasesOverviewStatCardProps = {
  isLoading: boolean;
  isError?: boolean;
  stats: ProjectSupportStats | undefined;
  onStatClick?: (key: keyof ProjectSupportStats) => void;
};

export type ChatHistoryListProps = {
  items: ChatHistoryItem[];
  isLoading?: boolean;
  isError?: boolean;
  onItemAction?: (chatId: string, action: ChatAction) => void;
};

export type ChatInputProps = {
  inputValue: string;
  setInputValue: (value: string) => void;
  onSend: () => void;
  onCreateCase?: () => void;
  isSending?: boolean;
  isCreateCaseLoading?: boolean;
  resetTrigger?: number;
  forceRichText?: boolean;
  disabled?: boolean;
};

export type ChatHeaderProps = {
  onBack: () => void;
  onCreateCase?: () => void;
  isCreateCaseLoading?: boolean;
  chatNumber?: string;
};

export type RecommendationsCardProps = {
  recommendations: Recommendation[];
};

export type EscalationBannerProps = {
  visible: boolean;
  onCreateCase: () => void;
  isLoading?: boolean;
  isCreateCaseDisabled?: boolean;
};

export type ChatMessageListProps = {
  messages: Message[];
  messagesEndRef: RefObject<HTMLDivElement | null>;
  onCreateCase?: () => void;
  onThumbsUp?: (messageId: string) => void;
  onThumbsDown?: (messageId: string) => void;
  onFetchOlder?: () => void;
  isFetchingOlder?: boolean;
  onSolutionWorked?: () => void;
};

export type ChatMessageBubbleProps = {
  message: Message;
  onCreateCase?: () => void;
  onThumbsUp?: (messageId: string) => void;
  onThumbsDown?: (messageId: string) => void;
  onSolutionWorked?: () => void;
};
