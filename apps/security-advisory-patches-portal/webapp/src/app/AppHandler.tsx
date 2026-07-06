// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
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
import { useLocation, useNavigate } from 'react-router-dom';
import { Box, CircularProgress } from '@mui/material';
import { useAppAuth } from '@src/context/AuthContext';
import { useAppSelector } from '@src/slices/store';
import PatchesPdfPage from '@src/view/PatchesPdf/PatchesPdfPage';
import RootLandingPage from '@src/view/RootLanding/RootLandingPage';
import NotFoundPage from '@src/view/NotFound/NotFoundPage';
import { SEC_ADV_REDIRECT_PATH_KEY, pathnameEndsWithPdf } from '@src/constants/constants';

const LOADING_SX = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  height: '100vh',
  bgcolor: '#f5f5f5',
  gap: 3,
} as const;

const LoadingScreen: React.FC<{ message: string }> = ({ message }) => (
  <Box sx={LOADING_SX}>
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
        {message}
      </Box>
    </Box>
  </Box>
);

const AppHandler: React.FC = () => {
  const { appSignOut } = useAppAuth();
  const { isAuthenticated, isLoading, user } = useAppSelector((state) => state.auth);
  const location = useLocation();
  const navigate = useNavigate();

  useLayoutEffect(() => {
    if (!isAuthenticated || isLoading) {
      return;
    }
    try {
      const redirectPath = sessionStorage.getItem(SEC_ADV_REDIRECT_PATH_KEY);
      const redirectPathOnly = redirectPath?.split('?')[0] ?? '';
      if (!pathnameEndsWithPdf(redirectPathOnly)) {
        return;
      }
      const pathOnly = location.pathname;
      const onOAuthLanding = pathOnly === '/' || pathOnly === '';
      if (onOAuthLanding) {
        navigate(redirectPath!, { replace: true });
      }
    } catch (e) {
      console.warn('Failed to restore redirect path:', e);
    }
  }, [isAuthenticated, isLoading, location.pathname, navigate]);

  useLayoutEffect(() => {
    try {
      const redirectPath = sessionStorage.getItem(SEC_ADV_REDIRECT_PATH_KEY);
      if (!redirectPath) {
        return;
      }
      const current = location.pathname + location.search;
      if (current === redirectPath) {
        sessionStorage.removeItem(SEC_ADV_REDIRECT_PATH_KEY);
      }
    } catch {
      // ignore
    }
  }, [location.pathname, location.search]);

  if (isLoading || !isAuthenticated) {
    return <LoadingScreen message="Getting things ready..." />;
  }

  const pathOnly = location.pathname;
  const oauthPdfResume =
    (pathOnly === '/' || pathOnly === '') &&
    !!sessionStorage.getItem(SEC_ADV_REDIRECT_PATH_KEY) &&
    pathnameEndsWithPdf(sessionStorage.getItem(SEC_ADV_REDIRECT_PATH_KEY)!.split('?')[0]);

  if (oauthPdfResume) {
    return <LoadingScreen message="Opening your link…" />;
  }

  if (pathOnly === '/' || pathOnly === '') {
    return <RootLandingPage username={user?.username} onLogout={appSignOut} />;
  }

  if (pathnameEndsWithPdf(pathOnly)) {
    return <PatchesPdfPage username={user?.username} onLogout={appSignOut} />;
  }

  return <NotFoundPage username={user?.username} onLogout={appSignOut} />;
};

export default AppHandler;
