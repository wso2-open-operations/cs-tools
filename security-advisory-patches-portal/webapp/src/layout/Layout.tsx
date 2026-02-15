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
import { Box } from '@mui/material';
import Header from './Header';
import Breadcrumb from './Breadcrumb';
import Footer from './Footer';

interface LayoutProps {
  children: React.ReactNode;
  username?: string;
  onLogout: () => void;
  pathSegments: string[];
  onNavigate: (segments: string[]) => void;
}

const Layout: React.FC<LayoutProps> = ({
  children,
  username,
  onLogout,
  pathSegments,
  onNavigate,
}) => {

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
      <Header
        username={username}
        onLogout={onLogout}
      />

      <Box sx={{ display: 'flex', flexGrow: 1, overflow: 'hidden' }}>
        <Box
          sx={{
            display: 'flex',
            flexDirection: 'column',
            flexGrow: 1,
            overflow: 'hidden',
          }}
        >
          <Breadcrumb pathSegments={pathSegments} onNavigate={onNavigate} />

          <Box
            sx={{
              flexGrow: 1,
              overflow: 'auto',
              p: 3,
            }}
          >
            {children}
          </Box>

          <Footer />
        </Box>
      </Box>
    </Box>
  );
};

export default Layout;
