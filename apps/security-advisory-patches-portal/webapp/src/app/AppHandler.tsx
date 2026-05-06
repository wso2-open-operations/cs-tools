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

import React, { useLayoutEffect } from 'react';
import { Routes, Route, useLocation, useNavigate } from 'react-router-dom';
import { Box, CircularProgress } from '@mui/material';
import { useAppAuth } from '@src/context/AuthContext';
import { useAppSelector } from '@src/slices/store';
import FileExplorerPage from '@src/view/FileExplorer/FileExplorerPage';
import { SEC_ADV_REDIRECT_PATH_KEY } from '@src/constants/constants';

const AppHandler: React.FC = () => {
  const { appSignOut } = useAppAuth();
  const { isAuthenticated, isLoading, user } = useAppSelector((state) => state.auth);
  const location = useLocation();
  const navigate = useNavigate();

  // After OAuth, the browser is typically sent to signInRedirectURL (often "/" only). Restore the
  // deep link we stored in AuthContext right before signIn(). useLayoutEffect avoids a flash of "/".
  useLayoutEffect(() => {
    if (!isAuthenticated || isLoading) {
      return;
    }
    try {
      const redirectPath = sessionStorage.getItem(SEC_ADV_REDIRECT_PATH_KEY);
      if (!redirectPath?.startsWith('/patches')) {
        return;
      }
      // IdP often returns to "/" or exactly "/patches" (configured sign-in redirect)
      const pathOnly = location.pathname;
      const onOAuthLanding =
        pathOnly === '/' ||
        pathOnly === '' ||
        pathOnly === '/patches';
      if (onOAuthLanding) {
        sessionStorage.removeItem(SEC_ADV_REDIRECT_PATH_KEY);
        navigate(redirectPath, { replace: true });
      }
    } catch (e) {
      console.warn('Failed to restore redirect path:', e);
    }
  }, [isAuthenticated, isLoading, location.pathname, navigate]);

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
      <Route
        path="*"
        element={<FileExplorerPage username={user?.username} onLogout={appSignOut} />}
      />
    </Routes>
  );
};

export default AppHandler;

