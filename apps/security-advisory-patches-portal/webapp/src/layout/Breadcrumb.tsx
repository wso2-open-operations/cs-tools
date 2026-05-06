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
import { Breadcrumbs, Link, Typography, Paper } from '@mui/material';
import { Home as HomeIcon, NavigateNext as NavigateNextIcon } from '@mui/icons-material';

interface BreadcrumbProps {
  pathSegments: string[];
  onNavigate: (segments: string[]) => void;
}

const Breadcrumb: React.FC<BreadcrumbProps> = ({ pathSegments, onNavigate }) => {
  return (
    <Paper
      elevation={0}
      sx={{
        px: 3,
        py: 1.5,
        borderBottom: '1px solid',
        borderColor: 'divider',
        borderRadius: 0,
        backgroundColor: 'background.paper',
      }}
    >
      <Breadcrumbs separator={<NavigateNextIcon fontSize="small" />} aria-label="breadcrumb">
        <Link
          underline="hover"
          sx={{
            display: 'flex',
            alignItems: 'center',
            color: 'text.secondary',
            cursor: 'pointer',
            '&:hover': {
              color: 'primary.main',
            },
          }}
          onClick={() => onNavigate([])}
        >
          <HomeIcon
            sx={{
              mr: 0.5,
              color: 'primary.main',
            }}
            fontSize="small"
          />
          Home
        </Link>

        {pathSegments.map((segment, index) => {
          const isLast = index === pathSegments.length - 1;
          return isLast ? (
            <Typography key={index} color="primary" fontWeight="medium">
              {segment}
            </Typography>
          ) : (
            <Link
              key={index}
              underline="hover"
              sx={{
                color: 'text.secondary',
                cursor: 'pointer',
                '&:hover': {
                  color: 'primary.main',
                },
              }}
              onClick={() => onNavigate(pathSegments.slice(0, index + 1))}
            >
              {segment}
            </Link>
          );
        })}
      </Breadcrumbs>
    </Paper>
  );
};

export default Breadcrumb;
