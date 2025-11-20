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

import React, { useEffect } from 'react';
import {
  Box,
  Paper,
  Typography,
  IconButton,
  Button,
  Avatar,
  CircularProgress,
  Alert,
} from '@mui/material';
import {
  Close as CloseIcon,
  Download as DownloadIcon,
  PictureAsPdf as PdfIcon,
  Description as FileIcon,
  Archive as ZipIcon,
  TableChart as ExcelIcon,
  Article as DocIcon,
} from '@mui/icons-material';
import { FileItem as FileItemType } from '@src/types/types';
import { useAppDispatch, useAppSelector } from '@src/slices/store';
import { downloadFile } from '@src/slices/fileSlice/file';
import { getFileName, getFileExtension } from '@src/utils/fileService';
import { formatFileSize } from '@src/utils/utils';
import { useSnackbar } from 'notistack';
import { SUCCESS_MESSAGES, FILE_TYPE_INFO } from '@src/constants/constants';

interface FileViewerProps {
  file: FileItemType;
  onClose: () => void;
}

const FileViewer: React.FC<FileViewerProps> = ({ file, onClose }) => {
  const dispatch = useAppDispatch();
  const { enqueueSnackbar } = useSnackbar();
  const { downloadError } = useAppSelector((state) => state.file);
  const [previewUrl, setPreviewUrl] = React.useState<string>('');
  const [isLoadingPreview, setIsLoadingPreview] = React.useState(true);

  const fileName = getFileName(file.name);
  const extension = getFileExtension(fileName);

  // Fetch file content for preview when component mounts
  useEffect(() => {
    let objectUrl = '';
    let isMounted = true;
    
    const fetchFileContent = async () => {
      setIsLoadingPreview(true);
      const result = await dispatch(downloadFile(file.name));
      
      // Only update state if component is still mounted and file hasn't changed
      if (isMounted && downloadFile.fulfilled.match(result)) {
        const fileData = result.payload.data;
        
        // Create object URL for preview
        objectUrl = URL.createObjectURL(fileData);
        setPreviewUrl(objectUrl);
      }
      
      if (isMounted) {
        setIsLoadingPreview(false);
      }
    };

    fetchFileContent();

    return () => {
      // Mark component as unmounted to prevent stale updates
      isMounted = false;
      
      // Clean up object URL to prevent memory leaks
      if (objectUrl) {
        URL.revokeObjectURL(objectUrl);
      }
      
      // Clean up the preview URL if it exists
      if (previewUrl) {
        URL.revokeObjectURL(previewUrl);
        setPreviewUrl('');
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dispatch, file.name]);

  const handleDownload = () => {
    // If we already have the preview URL (object URL from file data), use it
    if (previewUrl) {
      const link = document.createElement('a');
      link.href = previewUrl;
      link.download = fileName;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      enqueueSnackbar(SUCCESS_MESSAGES.FILE_DOWNLOADED, { variant: 'success' });
    }
  };

  const getFileIcon = () => {
    switch (extension) {
      case 'pdf':
        return <PdfIcon sx={{ fontSize: 40 }} />;
      case 'zip':
        return <ZipIcon sx={{ fontSize: 40 }} />;
      case 'xls':
      case 'xlsx':
        return <ExcelIcon sx={{ fontSize: 40 }} />;
      case 'doc':
      case 'docx':
        return <DocIcon sx={{ fontSize: 40 }} />;
      default:
        return <FileIcon sx={{ fontSize: 40 }} />;
    }
  };

  const getFileColor = () => {
    return FILE_TYPE_INFO[extension]?.color || FILE_TYPE_INFO.default.color;
  };

  const canPreview = () => {
    const previewableExtensions = ['pdf', 'png', 'jpg', 'jpeg', 'gif', 'webp', 'svg'];
    return previewableExtensions.includes(extension);
  };

  const renderPreview = () => {
    // Show loading spinner only when actually loading
    if (isLoadingPreview) {
      return (
        <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', p: 4 }}>
          <CircularProgress size={60} />
          <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
            Loading preview...
          </Typography>
        </Box>
      );
    }

    // Show error state if download failed
    if (downloadError) {
      return (
        <Paper
          elevation={1}
          sx={{
            p: 4,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            maxWidth: 500,
          }}
        >
          <Alert severity="error" sx={{ mb: 3, width: '100%' }}>
            {downloadError === 'Server error. Please try again later.' 
              ? 'This file could not be loaded. It may have been moved or deleted.'
              : downloadError}
          </Alert>
          <Typography variant="body2" color="text.secondary" align="center">
            Unable to preview this file.
          </Typography>
        </Paper>
      );
    }

    // Show empty state if no preview URL is available
    if (!previewUrl) {
      return (
        <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', p: 4 }}>
          <Typography variant="body2" color="text.secondary">
            No preview available
          </Typography>
        </Box>
      );
    }

    // PDF Preview using object element
    if (extension === 'pdf') {
      return (
        <Box sx={{ width: '100%', height: '100%', minHeight: '600px' }}>
          <object
            data={previewUrl}
            type="application/pdf"
            width="100%"
            height="100%"
            style={{ borderRadius: '4px' }}
          >
            <Box sx={{ p: 4, textAlign: 'center' }}>
              <Typography variant="body1" gutterBottom>
                PDF preview is not available in your browser.
              </Typography>
              <Button variant="contained" onClick={handleDownload} sx={{ mt: 2 }}>
                Download PDF
              </Button>
            </Box>
          </object>
        </Box>
      );
    }

    // Image Preview
    if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg'].includes(extension)) {
      return (
        <Box
          sx={{
            width: '100%',
            height: '100%',
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            p: 2,
            bgcolor: '#f5f5f5',
          }}
        >
          <img
            src={previewUrl}
            alt={fileName}
            style={{
              maxWidth: '100%',
              maxHeight: '100%',
              objectFit: 'contain',
              borderRadius: '4px',
              boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
            }}
          />
        </Box>
      );
    }

    return null;
  };

  return (
    <Box
      sx={{
        width: '100%',
        borderLeft: 1,
        borderColor: 'divider',
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
      }}
    >
      <Paper
        elevation={0}
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          px: 2,
          py: 1.5,
          backgroundColor: 'secondary.main',
          color: 'white',
          borderRadius: 0,
        }}
      >
        <Typography variant="subtitle1" noWrap>
          {fileName}
        </Typography>
        <IconButton
          size="small"
          onClick={onClose}
          sx={{
            color: 'white',
          }}
        >
          <CloseIcon />
        </IconButton>
      </Paper>

      <Box
        sx={{
          flexGrow: 1,
          overflow: 'hidden',
          bgcolor: 'background.default',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        {/* File Info Bar */}
        <Box
          sx={{
            p: 2,
            borderBottom: 1,
            borderColor: 'divider',
            bgcolor: 'background.paper',
          }}
        >
          {file.size && file.size > 0 && (
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
              <Typography variant="body2" color="text.secondary">
                Size: {formatFileSize(file.size)}
              </Typography>
            </Box>
          )}

          <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
            <Button
              variant="contained"
              color="primary"
              size="small"
              startIcon={<DownloadIcon />}
              onClick={handleDownload}
              disabled={isLoadingPreview || !previewUrl || !!downloadError}
            >
              {isLoadingPreview ? 'Preparing...' : 'Download'}
            </Button>
          </Box>
        </Box>

        {/* Preview Area */}
        <Box
          sx={{
            flexGrow: 1,
            overflow: 'auto',
            p: 2,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          {canPreview() ? (
            renderPreview()
          ) : (
            <Paper
              elevation={1}
              sx={{
                p: 4,
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Avatar
                sx={{
                  width: 80,
                  height: 80,
                  bgcolor: getFileColor(),
                  mb: 3,
                }}
              >
                {getFileIcon()}
              </Avatar>

              <Typography variant="h6" gutterBottom align="center">
                Preview not available
              </Typography>

              <Typography variant="body2" color="text.secondary" align="center">
                This file type cannot be previewed. Please download to view.
              </Typography>
            </Paper>
          )}
        </Box>
      </Box>
    </Box>
  );
};

export default FileViewer;

