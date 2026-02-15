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
import { Routes, Route, useLocation, useNavigate } from 'react-router-dom';
import { Box, CircularProgress } from '@mui/material';
import { useAppAuth } from '@src/context/AuthContext';
import { useAppSelector } from '@src/slices/store';
import FileExplorerPage from '@src/view/FileExplorer/FileExplorerPage';

const REDIRECT_PATH_KEY = 'sec_adv_redirect_path';

const AppHandler: React.FC = () => {
  const { appSignOut } = useAppAuth();
  const { isAuthenticated, isLoading, user } = useAppSelector((state) => state.auth);
  const location = useLocation();
  const navigate = useNavigate();

  // Store the intended path when user is not authenticated
  useEffect(() => {
    if (!isAuthenticated && !isLoading && location.pathname !== '/') {
      try {
        sessionStorage.setItem(REDIRECT_PATH_KEY, location.pathname);
      } catch (e) {
        console.warn('Failed to store redirect path:', e);
      }
    }
  }, [isAuthenticated, isLoading, location.pathname]);

  // Redirect after authentication
  useEffect(() => {
    if (isAuthenticated && !isLoading) {
      try {
        const redirectPath = sessionStorage.getItem(REDIRECT_PATH_KEY);
        if (redirectPath && redirectPath !== '/') {
          sessionStorage.removeItem(REDIRECT_PATH_KEY);
          // Navigate after a small delay to ensure app is fully initialized
          const timer = setTimeout(() => {
            navigate(redirectPath, { replace: true });
          }, 100);
          return () => clearTimeout(timer);
        }
      } catch (e) {
        console.warn('Failed to retrieve redirect path:', e);
      }
    }
  }, [isAuthenticated, isLoading, navigate]);

  // Show loader while loading OR not authenticated (redirecting to Asgardeo)
  if (isLoading || !isAuthenticated) {
    return (
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100vh',
          bgcolor: '#f5f5f5', // Greyish white background
          gap: 3,
        }}
      >
        <CircularProgress size={60} />
        <Box sx={{ textAlign: 'center' }}>
          <Box
            component="span"
            sx={{
              fontSize: '1.1rem',
              color: 'text.secondary',
              fontWeight: 500,
            }}
          >
            Getting things ready...
          </Box>
        </Box>
      </Box>
    );
  }

  return (
    <Routes>
      <Route path="/" element={<FileExplorerPage username={user?.username} onLogout={appSignOut} />} />
      <Route path="/*" element={<FileExplorerPage username={user?.username} onLogout={appSignOut} />} />
    </Routes>
  );
};

export default AppHandler;

