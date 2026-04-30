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

// Error Messages
export const ERROR_MESSAGES = {
  GENERIC_ERROR: 'An error occurred. Please try again.',
  NETWORK_ERROR: 'Network error. Please check your connection.',
  SERVER_ERROR: 'Server error. Please try again later.',
  INVALID_REQUEST: 'Invalid path format.',
};

// Success Messages
export const SUCCESS_MESSAGES = {
  FILE_DOWNLOADED: 'File downloaded successfully.',
};

// File Extensions and their display info
export const FILE_TYPE_INFO: Record<string, { color: string; label: string }> = {
  pdf: { color: '#FF0000', label: 'PDF Document' },
  doc: { color: '#2196f3', label: 'Word Document' },
  docx: { color: '#2196f3', label: 'Word Document' },
  xls: { color: '#4caf50', label: 'Excel Spreadsheet' },
  xlsx: { color: '#4caf50', label: 'Excel Spreadsheet' },
  txt: { color: '#9e9e9e', label: 'Text File' },
  md: { color: '#000000', label: 'Markdown File' },
  zip: { color: '#FFA500', label: 'ZIP Archive' },
  default: { color: '#9e9e9e', label: 'File' },
};
