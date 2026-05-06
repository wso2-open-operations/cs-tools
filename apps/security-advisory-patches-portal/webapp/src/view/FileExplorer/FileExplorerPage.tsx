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

import React, { useEffect, useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Box, Typography } from '@mui/material';
import { useAppDispatch, useAppSelector } from '@src/slices/store';
import {
  fetchFileItems,
  setSelectedFile,
  navigateToPath,
} from '@src/slices/fileSlice/file';
import { State, FileItem } from '@src/types/types';
import FileList from '@src/component/ui/FileList';
import FileViewer from '@src/component/ui/FileViewer';
import LoadingScreen from '@src/component/common/LoadingScreen';
import ErrorHandler from '@src/component/common/ErrorHandler';
import Layout from '@src/layout/Layout';
import { buildPath, parsePathSegments, buildUrlPath } from '@src/utils/utils';

interface FileExplorerPageProps {
  username?: string;
  onLogout: () => void;
}

const FileExplorerPage: React.FC<FileExplorerPageProps> = ({ username, onLogout }) => {
  const navigate = useNavigate();
  const location = useLocation();
  const dispatch = useAppDispatch();
  const { items, currentPath, pathSegments, selectedFile, state, error } = useAppSelector(
    (state) => state.file
  );

  const [explorerWidth, setExplorerWidth] = useState(50); // Percentage width of the explorer
  const [isResizing, setIsResizing] = useState(false);

  // Sync URL with Redux state on mount and location change
  useEffect(() => {
    const urlPath = location.pathname.replace(/^\//, ''); // Remove leading slash
    
    if (!urlPath) {
      // Root path
      dispatch(navigateToPath([]));
      dispatch(fetchFileItems(''));
      return;
    }
    
    // Parse URL segments (converts hyphens back to spaces)
    const allSegments = parsePathSegments(urlPath);
    
    // Check if last segment is a file (contains extension)
    const lastSegment = allSegments[allSegments.length - 1] || '';
    const isFile = /\.[a-zA-Z0-9]+$/.test(lastSegment);
    
    if (isFile) {
      // URL contains a file: /Security-Patches/file.pdf
      const pathSegments = allSegments.slice(0, -1); // All except last (file name)
      const fileName = lastSegment;
      const dirPath = buildPath(pathSegments);
      
      // Update state and fetch directory items
      dispatch(navigateToPath(pathSegments));
      dispatch(fetchFileItems(dirPath));
      
      // Set selected file for preview
      const fullFilePath = dirPath + fileName;
      dispatch(setSelectedFile({ 
        name: fullFilePath, 
        isFolder: false 
      }));
    } else {
      // URL contains only directory path
      const newPath = buildPath(allSegments);
      dispatch(navigateToPath(allSegments));
      dispatch(fetchFileItems(newPath));
      dispatch(setSelectedFile(null)); // Clear file selection
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname, dispatch]);

  // Handle folder navigation
  const handleFolderClick = (folderName: string) => {
    // Build new path by appending folder name to current path
    const newSegments = [...pathSegments, folderName];
    const newPath = buildPath(newSegments);
    
    // Update URL with properly encoded segments
    const urlPath = buildUrlPath(newSegments);
    navigate(urlPath);
    
    // Update Redux state
    dispatch(navigateToPath(newSegments));
    dispatch(fetchFileItems(newPath));
  };

  // Handle file selection
  const handleFileClick = (file: FileItem) => {
    // Build complete file path by combining current path with filename
    const fullPath = currentPath + file.name;
    const fileWithFullPath = { ...file, name: fullPath };
    
    // Update URL to include file name (shareable link)
    const urlPath = buildUrlPath(pathSegments, file.name);
    navigate(urlPath, { replace: true }); // Use replace to avoid cluttering history
    
    dispatch(setSelectedFile(fileWithFullPath));
  };

  // Handle breadcrumb navigation
  const handleNavigate = (segments: string[]) => {
    const newPath = buildPath(segments);
    
    // Update URL with properly encoded segments
    const urlPath = buildUrlPath(segments);
    navigate(urlPath);
    
    // Update Redux state
    dispatch(navigateToPath(segments));
    dispatch(fetchFileItems(newPath));
    dispatch(setSelectedFile(null));
  };

  // Handle retry
  const handleRetry = () => {
    dispatch(fetchFileItems(currentPath));
  };

  const getCurrentFolderName = () => {
    if (pathSegments.length === 0) return 'Security Advisories';
    return pathSegments[pathSegments.length - 1];
  };

  // Handle resize start
  const handleMouseDown = () => {
    setIsResizing(true);
  };

  // Handle resize
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizing) return;

      const container = document.getElementById('file-explorer-container');
      if (!container) return;

      const containerRect = container.getBoundingClientRect();
      const newWidth = ((e.clientX - containerRect.left) / containerRect.width) * 100;

      // Limit between 20% and 80%
      if (newWidth >= 20 && newWidth <= 80) {
        setExplorerWidth(newWidth);
      }
    };

    const handleMouseUp = () => {
      setIsResizing(false);
    };

    if (isResizing) {
      document.addEventListener('mousemove', handleMouseMove);
      document.addEventListener('mouseup', handleMouseUp);
    }

    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, [isResizing]);

  return (
    <Layout
      username={username}
      onLogout={onLogout}
      pathSegments={pathSegments}
      onNavigate={handleNavigate}
    >
      <Box 
        id="file-explorer-container"
        sx={{ 
          display: 'flex', 
          height: '100%', 
          overflow: 'hidden',
          position: 'relative',
          userSelect: isResizing ? 'none' : 'auto',
        }}
      >
        <Box
          sx={{
            overflow: 'auto',
            width: selectedFile ? `${explorerWidth}%` : '100%',
            transition: isResizing ? 'none' : 'width 0.2s ease',
          }}
        >
          {state === State.loading && <LoadingScreen message="Loading files..." />}

          {state === State.failed && <ErrorHandler message={error} onRetry={handleRetry} />}

          {state === State.success && (
            <>
              <Box sx={{ mb: 3 }}>
                <Typography variant="h5" fontWeight="medium" color="text.primary">
                  {getCurrentFolderName()}
                </Typography>
              </Box>

              <FileList
                items={items}
                onFolderClick={handleFolderClick}
                onFileClick={handleFileClick}
              />
            </>
          )}
        </Box>

        {selectedFile && (
          <>
            {/* Resize Handle */}
            <Box
              onMouseDown={handleMouseDown}
              sx={{
                width: '4px',
                cursor: 'col-resize',
                backgroundColor: isResizing ? 'primary.main' : 'divider',
                transition: 'background-color 0.2s',
                '&:hover': {
                  backgroundColor: 'primary.main',
                },
                zIndex: 10,
                position: 'relative',
              }}
            >
              {/* Visual indicator for better UX */}
              <Box
                sx={{
                  position: 'absolute',
                  top: '50%',
                  left: '50%',
                  transform: 'translate(-50%, -50%)',
                  width: '20px',
                  height: '40px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  backgroundColor: 'background.paper',
                  borderRadius: '4px',
                  boxShadow: 1,
                  opacity: isResizing ? 1 : 0,
                  transition: 'opacity 0.2s',
                  pointerEvents: 'none',
                }}
              >
                <Box
                  sx={{
                    width: '2px',
                    height: '20px',
                    backgroundColor: 'primary.main',
                    mx: '2px',
                  }}
                />
                <Box
                  sx={{
                    width: '2px',
                    height: '20px',
                    backgroundColor: 'primary.main',
                    mx: '2px',
                  }}
                />
              </Box>
            </Box>

            <Box
              sx={{
                width: `${100 - explorerWidth}%`,
                overflow: 'hidden',
                transition: isResizing ? 'none' : 'width 0.2s ease',
              }}
            >
              <FileViewer 
                file={selectedFile} 
                onClose={() => {
                  // Remove file from URL when closing preview
                  const urlPath = buildUrlPath(pathSegments);
                  navigate(urlPath, { replace: true });
                  dispatch(setSelectedFile(null));
                }} 
              />
            </Box>
          </>
        )}
      </Box>
    </Layout>
  );
};

export default FileExplorerPage;