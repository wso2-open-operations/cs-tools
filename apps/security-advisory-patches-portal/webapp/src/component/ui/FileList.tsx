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

import React, { useMemo } from 'react';
import {
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  Box,
  Typography,
} from '@mui/material';
import {
  Folder as FolderIcon,
  Description as FileIcon,
  PictureAsPdf as PdfIcon,
  Archive as ZipIcon,
  TableChart as ExcelIcon,
  Article as DocIcon,
  FolderOpen as FolderOpenIcon,
} from '@mui/icons-material';
import { FileItem } from '@src/types/types';
import { formatFileSize } from '@src/utils/utils';
import { getFileExtension } from '@src/utils/fileService';
import { FILE_TYPE_INFO } from '@src/constants/constants';

interface FileListProps {
  items: FileItem[];
  onFolderClick: (folderName: string) => void;
  onFileClick: (file: FileItem) => void;
}

const FileList: React.FC<FileListProps> = ({
  items,
  onFolderClick,
  onFileClick,
}) => {
  const filteredItems = items;

  // Sort: folders first, then files, both alphabetically
  const sortedItems = useMemo(() => {
    return [...filteredItems].sort((a, b) => {
      if (a.isFolder && !b.isFolder) return -1;
      if (!a.isFolder && b.isFolder) return 1;
      return a.name.localeCompare(b.name);
    });
  }, [filteredItems]);

  const getFileIcon = (item: FileItem) => {
    if (item.isFolder) {
      return <FolderIcon sx={{ color: '#FF5000', fontSize: 24 }} />;
    }

    const extension = getFileExtension(item.name);
    const color = FILE_TYPE_INFO[extension]?.color || FILE_TYPE_INFO.default.color;

    switch (extension) {
      case 'pdf':
        return <PdfIcon sx={{ color, fontSize: 24 }} />;
      case 'zip':
        return <ZipIcon sx={{ color, fontSize: 24 }} />;
      case 'xls':
      case 'xlsx':
        return <ExcelIcon sx={{ color, fontSize: 24 }} />;
      case 'doc':
      case 'docx':
        return <DocIcon sx={{ color, fontSize: 24 }} />;
      default:
        return <FileIcon sx={{ color, fontSize: 24 }} />;
    }
  };

  const getFileType = (item: FileItem) => {
    if (item.isFolder) return 'Folder';
    const extension = getFileExtension(item.name);
    return FILE_TYPE_INFO[extension]?.label || FILE_TYPE_INFO.default.label;
  };

  if (filteredItems.length === 0) {
    return (
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          height: '400px',
          color: 'text.secondary',
        }}
      >
        <FolderOpenIcon
          sx={{
            fontSize: 64,
            opacity: 0.2,
            mb: 2,
          }}
        />
        <Typography variant="body1">
          This folder is empty.
        </Typography>
      </Box>
    );
  }

  return (
    <TableContainer component={Paper} elevation={0} sx={{ border: '1px solid', borderColor: 'divider' }}>
      <Table>
        <TableHead>
          <TableRow sx={{ bgcolor: 'grey.50' }}>
            <TableCell sx={{ fontWeight: 600, width: '60%' }}>Name</TableCell>
            <TableCell sx={{ fontWeight: 600, width: '20%' }}>Type</TableCell>
            <TableCell sx={{ fontWeight: 600, width: '20%' }} align="right">
              Size
            </TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {sortedItems.map((item) => (
            <TableRow
              key={item.name}
              hover
              onClick={() => (item.isFolder ? onFolderClick(item.name) : onFileClick(item))}
              sx={{
                cursor: 'pointer',
                '&:hover': {
                  bgcolor: 'action.hover',
                },
              }}
            >
              <TableCell>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  {getFileIcon(item)}
                  <Typography
                    variant="body2"
                    sx={{
                      fontWeight: item.isFolder ? 500 : 400,
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {item.name}
                  </Typography>
                </Box>
              </TableCell>
              <TableCell>
                <Typography variant="body2" color="text.secondary">
                  {getFileType(item)}
                </Typography>
              </TableCell>
              <TableCell align="right">
                <Typography variant="body2" color="text.secondary">
                  {item.isFolder ? '—' : item.size ? formatFileSize(item.size) : '—'}
                </Typography>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
};

export default FileList;

