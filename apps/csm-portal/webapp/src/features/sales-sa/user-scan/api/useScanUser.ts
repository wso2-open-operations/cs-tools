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

import { useMutation, type UseMutationResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";

export interface ScanUserRequest {
  email: string;
  subscriptionKey: string;
  isPartner: boolean;
}

// Mirrors the Go backend's SplScanInformation (internal/handler/user_scan.go)
// — Issue/Solution/Documentation/InvitationUrl all carry `json:"...,omitempty"`,
// so an unset field is omitted from the response (comes through as
// `undefined`), never an explicit `null`. Information itself is a Go value
// type (not a pointer), so it is always present — only its own inner fields
// can be absent.
export interface ScanSystemResultInfo {
  issue?: string;
  solution?: string;
  documentation?: string;
  invitationUrl?: string;
}

export interface ScanSystemResult {
  order: number;
  label: string;
  state: boolean;
  information: ScanSystemResultInfo;
}

export interface ScanResponseItem {
  system: string;
  systemResult: ScanSystemResult[];
}

/**
 * `POST /scan-user` — analyzes a Sales/SA-supplied email + subscription
 * key against both the sales-side and CS-side entity services, returning a
 * per-system (Salesforce, ServiceNow) validation breakdown. See
 * internal/handler/user_scan.go's SplUserScanHandler for the full
 * business logic this reports on.
 */
export function useScanUser(): UseMutationResult<
  ScanResponseItem[],
  Error,
  ScanUserRequest
> {
  const api = useBackendApi();
  return useMutation<ScanResponseItem[], Error, ScanUserRequest>({
    mutationFn: (payload) =>
      api.post<ScanUserRequest, ScanResponseItem[]>("/scan-user", payload),
  });
}
