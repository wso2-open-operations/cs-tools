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

// Base URL for the API service.
export const BACKEND_BASE_URL =
  window.config?.CSM_PORTAL_BACKEND_BASE_URL ??
  (import.meta.env?.MODE === "test" ? "http://localhost:8080" : undefined);

if (!BACKEND_BASE_URL) {
  throw new Error(
    "Missing required configuration: CSM_PORTAL_BACKEND_BASE_URL",
  );
}

// Base URL for the case-activity SSE stream (a separate Choreo REST
// endpoint backed by csm-portal-backend's dedicated :9092 listener — see
// that backend's cmd/server/main.go). Deliberately optional, unlike
// backendUrl: this backend only stands the stream listener up when Event
// Hub is configured, so an environment without it simply has no value here.
// useCaseActivityStream checks for that and no-ops rather than throwing, so
// the rest of the app functions normally (falling back to the existing
// manual-refresh/staleTime polling) when it's unset.
export const STREAM_BASE_URL = window.config?.CSM_PORTAL_STREAM_BASE_URL;

// Master on/off switch for the case-activity SSE stream, independent of
// whether STREAM_BASE_URL is configured — an explicit `true` opt-in, so a
// deployment can have the URL wired up ahead of time without the feature
// actually going live until this flag is flipped too. Defaults to false
// (rather than "on whenever a URL happens to be set") so a merge that
// carries this feature's code into an environment doesn't silently turn the
// stream on there.
export const STREAM_ENABLED = window.config?.CSM_PORTAL_STREAM_ENABLED === true;

// Interface for the API configuration.
interface ApiConfig {
  backendUrl: string;
  streamUrl?: string;
  streamEnabled: boolean;
}

// Configuration for the API service.
export const apiConfig: ApiConfig = {
  backendUrl: BACKEND_BASE_URL,
  streamUrl: STREAM_BASE_URL,
  streamEnabled: STREAM_ENABLED,
};
