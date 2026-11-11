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

/** What a cancelled download throws: an AbortError, so {@link isDownloadAbort} knows it. */
export function downloadCancelledError(): Error {
  const error = new Error("The download was cancelled.");
  error.name = "AbortError";
  return error;
}

/** True for the error a cancelled download throws (ours, or the browser's own for an aborted fetch). */
export function isDownloadAbort(error: unknown): boolean {
  return error instanceof Error && error.name === "AbortError";
}
