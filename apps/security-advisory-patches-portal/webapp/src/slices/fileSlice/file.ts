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

import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';
import { FileItem, State } from '@src/types/types';
import { listSecurityAdvisories, downloadSecurityAdvisory } from '@src/utils/fileService';
import { ERROR_MESSAGES } from '@src/constants/constants';

interface FileState {
  items: FileItem[];
  currentPath: string;
  pathSegments: string[];
  selectedFile: FileItem | null;
  state: State;
  error: string | null;
  downloadState: State;
  downloadError: string | null;
}

const initialState: FileState = {
  items: [],
  currentPath: '',
  pathSegments: [],
  selectedFile: null,
  state: State.idle,
  error: null,
  downloadState: State.idle,
  downloadError: null,
};

// Async thunk for listing file items
export const fetchFileItems = createAsyncThunk(
  'file/fetchItems',
  async (path: string, { rejectWithValue }) => {
    try {
      const items = await listSecurityAdvisories(path || undefined);
      return { items, path };
    } catch (error: any) {
      if (error.response) {
        const status = error.response.status;
        if (status === 400) {
          return rejectWithValue(ERROR_MESSAGES.INVALID_REQUEST);
        } else if (status === 500) {
          return rejectWithValue(ERROR_MESSAGES.SERVER_ERROR);
        }
        return rejectWithValue(error.response?.data?.message || ERROR_MESSAGES.GENERIC_ERROR);
      } else if (error.request) {
        return rejectWithValue(ERROR_MESSAGES.NETWORK_ERROR);
      }
      return rejectWithValue(ERROR_MESSAGES.GENERIC_ERROR);
    }
  }
);

// Async thunk for downloading a file
export const downloadFile = createAsyncThunk(
  'file/downloadFile',
  async (path: string, { rejectWithValue }) => {
    try {
      const fileData = await downloadSecurityAdvisory(path);
      return { data: fileData, path };
    } catch (error: any) {
      if (error.response) {
        // Backend always returns 500 for all errors (including file not found)
        return rejectWithValue(ERROR_MESSAGES.SERVER_ERROR);
      } else if (error.request) {
        // Network error - request sent but no response
        return rejectWithValue(ERROR_MESSAGES.NETWORK_ERROR);
      }
      // Unexpected error (e.g., request setup failure)
      return rejectWithValue(ERROR_MESSAGES.GENERIC_ERROR);
    }
  }
);

const fileSlice = createSlice({
  name: 'file',
  initialState,
  reducers: {
    setCurrentPath: (state, action: PayloadAction<string>) => {
      // Normalize the path: trim whitespace and remove duplicate/trailing slashes
      let cleanedPath = action.payload.trim();
      
      // Remove duplicate slashes
      cleanedPath = cleanedPath.replace(/\/+/g, '/');
      
      // Remove leading and trailing slashes for splitting
      cleanedPath = cleanedPath.replace(/^\/+|\/+$/g, '');
      
      // Compute path segments from cleaned path
      state.pathSegments = cleanedPath.length > 0
        ? cleanedPath.split('/').filter((s) => s.length > 0)
        : [];
      
      // Ensure currentPath ends with a single '/' if non-empty
      state.currentPath = cleanedPath.length > 0 ? cleanedPath + '/' : '';
    },
    setSelectedFile: (state, action: PayloadAction<FileItem | null>) => {
      state.selectedFile = action.payload;
    },
    navigateToPath: (state, action: PayloadAction<string[]>) => {
      state.pathSegments = action.payload;
      state.currentPath = action.payload.length > 0 ? action.payload.join('/') + '/' : '';
    },
    clearError: (state) => {
      state.error = null;
      state.downloadError = null;
    },
  },
  extraReducers: (builder) => {
    // Fetch file items
    builder.addCase(fetchFileItems.pending, (state) => {
      state.state = State.loading;
      state.error = null;
    });
    builder.addCase(fetchFileItems.fulfilled, (state, action) => {
      state.state = State.success;
      state.items = action.payload.items;
      state.currentPath = action.payload.path;
      state.error = null;
    });
    builder.addCase(fetchFileItems.rejected, (state, action) => {
      state.state = State.failed;
      state.error = action.payload as string;
      state.items = [];
    });

    // Download file
    builder.addCase(downloadFile.pending, (state) => {
      state.downloadState = State.loading;
      state.downloadError = null;
    });
    builder.addCase(downloadFile.fulfilled, (state) => {
      state.downloadState = State.success;
      state.downloadError = null;
    });
    builder.addCase(downloadFile.rejected, (state, action) => {
      state.downloadState = State.failed;
      state.downloadError = action.payload as string;
    });
  },
});

export const {
  setCurrentPath,
  setSelectedFile,
  navigateToPath,
  clearError,
} = fileSlice.actions;

export default fileSlice.reducer;

