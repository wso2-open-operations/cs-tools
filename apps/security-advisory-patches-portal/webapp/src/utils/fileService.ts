// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { APIService } from './apiService';
import { AppConfig } from '@src/config/config';
import { FileItem } from '@src/types/types';

/**
 * List security advisories from Azure File Share
 * @param path - Optional folder path (can be undefined to fetch root)
 * @returns Promise with array of file items
 */
export const listSecurityAdvisories = async (
  path?: string
): Promise<FileItem[]> => {
  const response = await APIService.getInstance().get(
    AppConfig.serviceUrls.listSecurityAdvisories,
    {
      params: path !== undefined ? { path } : {},
    }
  );
  return response.data;
};

/**
 * Download a security advisory file from Azure File Share
 * @param path - Path to the file in file share
 * @returns Promise with binary file data (Blob is a standard JavaScript type for binary data)
 */
export const downloadSecurityAdvisory = async (path: string): Promise<Blob> => {
  const response = await APIService.getInstance().get(
    AppConfig.serviceUrls.downloadSecurityAdvisory,
    {
      params: { path },
      responseType: 'blob', // axios config for binary data download
    }
  );
  return response.data;
};

/**
 * Helper to download a file to the user's device
 * @param fileData - The binary file data to download (Blob is a standard JavaScript type)
 * @param filename - Name for the downloaded file
 */
export const downloadFile = (fileData: Blob, filename: string) => {
  const url = window.URL.createObjectURL(fileData);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  
  // Delay cleanup to ensure download has started
  setTimeout(() => {
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  }, 100);
};

/**
 * Helper to extract filename from file path
 * @param path - Full file path
 * @returns Filename
 */
export const getFileName = (path: string): string => {
  const parts = path.split('/');
  return parts[parts.length - 1] || path;
};

/**
 * Helper to get file extension
 * @param filename - File name
 * @returns Extension (lowercase)
 */
export const getFileExtension = (filename: string): string => {
  const parts = filename.split('.');
  return parts.length > 1 ? parts[parts.length - 1].toLowerCase() : '';
};
