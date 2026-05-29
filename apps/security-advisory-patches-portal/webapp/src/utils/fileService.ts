// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
//
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

import { APIService } from './apiService';
import { AppConfig } from '@src/config/config';

/**
 * Download advisory bytes from `GET /files/{id}` where **`id`** is `encodeURIComponent(shareRelativePath)`.
 */
export const downloadSecurityAdvisory = async (path: string): Promise<Blob> => {
  const id = encodeURIComponent(path);
  const response = await APIService.getInstance().get(`${AppConfig.downloadFilesBaseUrl}/${id}`, {
    responseType: 'blob',
  });
  return response.data;
};

/** Last segment of a `/`-delimited path (used for UI labels and `Content-Disposition` filename hints). */
export const getFileName = (path: string): string => {
  const parts = path.split('/');
  return parts[parts.length - 1] || path;
};
