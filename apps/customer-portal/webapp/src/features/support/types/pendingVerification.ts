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

// Wire types for backend-v2's /pending-verifications endpoints. Mirrors
// apps/customer-portal/backend-v2/internal/dto/pending_verification.go exactly —
// keep in sync with that file's json tags, not entity-service's internal
// UPPER_SNAKE_CASE vocabulary.

export type PendingVerificationAddedReason = "auto-closed" | "manual";

export type PendingVerificationPreviousStatus =
  | "Solution Proposed"
  | "Awaiting Info";

export type PendingVerificationRecordType =
  | "Case"
  | "Engagement"
  | "Security Report"
  | "Service Request"
  | "Announcement"
  | "Change Request";

export interface PendingVerificationView {
  id: string;
  /** The real work_item UUID — use this for navigation, not caseId. */
  workItemId: string;
  /** The record's human-readable number (e.g. "CS0441731") — display only. */
  caseId: string;
  /** The record's own internal/WSO2 id (e.g. "CUPRSUB-1170"), distinct from
   * caseId — absent when the underlying record has none. */
  internalId?: string;
  caseTitle: string;
  severity?: string;
  recordType: PendingVerificationRecordType;
  addedReason: PendingVerificationAddedReason;
  previousStatus?: PendingVerificationPreviousStatus;
  addedAt: string;
  addedBy: string;
  verifiedAt?: string;
  verifiedBy?: string;
  note?: string;
}

export interface CreatePendingVerificationRequest {
  workItemId: string;
  addedReason: PendingVerificationAddedReason;
  previousStatus?: PendingVerificationPreviousStatus;
  note?: string;
}

export interface CreatePendingVerificationResponse {
  message: string;
  pendingVerification: PendingVerificationView;
}

export interface PendingVerificationSearchFilters {
  workItemId?: string;
  recordTypes?: PendingVerificationRecordType[];
  /** Narrows to entries added for this one reason — backs the Support page's
   * Auto-closed/Manual stat boxes, which link into a pre-filtered view. */
  addedReason?: PendingVerificationAddedReason;
  searchQuery?: string;
  includeVerified?: boolean;
  /** Overrides includeVerified entirely — narrows to verified rows only. */
  verifiedOnly?: boolean;
}

export interface PendingVerificationSearchRequest {
  projectId: string;
  pagination: { limit: number; offset: number };
  filters: PendingVerificationSearchFilters;
}

export interface PendingVerificationTypeCount {
  recordType: PendingVerificationRecordType;
  count: number;
}

export interface PendingVerificationSearchResponse {
  pendingVerifications: PendingVerificationView[];
  totalRecords: number;
  autoClosedCount: number;
  manualCount: number;
  typeCounts: PendingVerificationTypeCount[];
  limit: number;
  offset: number;
  hasMore: boolean;
}

export interface VerifyPendingVerificationResponse {
  message: string;
  pendingVerification: PendingVerificationView;
}
