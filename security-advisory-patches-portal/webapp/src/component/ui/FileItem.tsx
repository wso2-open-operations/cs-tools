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

import React from 'react';
import { Card, CardActionArea, CardContent, Typography, Avatar } from '@mui/material';
import { 
  Folder as FolderIcon, 
  Description as FileIcon,
  PictureAsPdf as PdfIcon,
  Archive as ZipIcon,
  TableChart as ExcelIcon,
  Article as DocIcon,
} from '@mui/icons-material';
import { FileItem as FileItemType } from '@src/types/types';
import { formatFileSize } from '@src/utils/utils';
import { getFileExtension } from '@src/utils/fileService';
import { FILE_TYPE_INFO } from '@src/constants/constants';

interface FileItemProps {
  item: FileItemType;
  onClick: () => void;
}

const FileItemCard: React.FC<FileItemProps> = ({ item, onClick }) => {
  const isFolder = item.isFolder;

  const getFileColor = () => {
    if (isFolder) return '#FF5000';
    const extension = getFileExtension(item.name);
    return FILE_TYPE_INFO[extension]?.color || FILE_TYPE_INFO.default.color;
  };

  const getDisplayName = () => {
    // Name is already just the item name without path prefix
    return item.name;
  };

  const getFileLabel = () => {
    if (isFolder) return 'Folder';
    const extension = getFileExtension(item.name);
    return FILE_TYPE_INFO[extension]?.label || FILE_TYPE_INFO.default.label;
  };

  const getFileIcon = () => {
    if (isFolder) return <FolderIcon />;
    
    const extension = getFileExtension(item.name);
    switch (extension) {
      case 'pdf':
        return <PdfIcon />;
      case 'zip':
        return <ZipIcon />;
      case 'xls':
      case 'xlsx':
        return <ExcelIcon />;
      case 'doc':
      case 'docx':
        return <DocIcon />;
      default:
        return <FileIcon />;
    }
  };

  return (
    <Card
      elevation={1}
      sx={{
        transition: 'all 0.2s ease-in-out',
        '&:hover': {
          transform: 'translateY(-4px)',
          boxShadow: 3,
          borderColor: 'primary.main',
        },
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <CardActionArea
        onClick={onClick}
        sx={{
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          py: 2,
        }}
      >
        <Avatar
          sx={{
            bgcolor: getFileColor(),
            width: 56,
            height: 56,
            mb: 1,
          }}
        >
          {getFileIcon()}
        </Avatar>
        <CardContent
          sx={{
            textAlign: 'center',
            pt: 1,
            pb: '8px !important',
            px: 2,
            width: '100%',
          }}
        >
          <Typography variant="body2" fontWeight="medium" noWrap title={getDisplayName()}>
            {getDisplayName()}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {getFileLabel()}
          </Typography>
          {!isFolder && item.size && item.size > 0 && (
            <Typography variant="caption" color="text.secondary" display="block">
              {formatFileSize(item.size)}
            </Typography>
          )}
        </CardContent>
      </CardActionArea>
    </Card>
  );
};

export default FileItemCard;

