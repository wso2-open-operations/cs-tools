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

import React, { createContext, useContext, useEffect } from 'react';
import { SecureApp, useAuthContext } from '@asgardeo/auth-react';
import { Box, CircularProgress } from '@mui/material';
import { useAppDispatch } from '@src/slices/store';
import { setAuthenticated, setUser, setLoading, logout } from '@src/slices/authSlice/auth';
import { APIService } from '@src/utils/apiService';
import { UserInfo } from '@src/types/types';
import { SEC_ADV_REDIRECT_PATH_KEY, SEC_ADV_SIGN_IN_INIT_KEY, pathnameEndsWithPdf } from '@src/constants/constants';

interface AppAuthContextType {
  appSignIn: () => void;
  appSignOut: () => void;
}

const AppAuthContext = createContext<AppAuthContextType | undefined>(undefined);

export const AppAuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const dispatch = useAppDispatch();
  const { state, signIn, signOut, getBasicUserInfo, getIDToken } = useAuthContext();

  useEffect(() => {
    const isSignInInitiated = sessionStorage.getItem(SEC_ADV_SIGN_IN_INIT_KEY) === 'true';

    if (state.isAuthenticated) {
      getBasicUserInfo().then(
        async (basicUserInfo) => {
          const userInfo: UserInfo = {
            username: basicUserInfo?.username || basicUserInfo?.email || 'User',
            email: basicUserInfo?.email || '',
            sub: basicUserInfo?.sub || '',
          };

          dispatch(setUser(userInfo));
          dispatch(setAuthenticated(true));
          dispatch(setLoading(false));

          if (pathnameEndsWithPdf(window.location.pathname)) {
            sessionStorage.removeItem(SEC_ADV_REDIRECT_PATH_KEY);
          }

          new APIService(async () => {
            const token = await getIDToken();
            return { idToken: token || '' };
          });

          sessionStorage.setItem(SEC_ADV_SIGN_IN_INIT_KEY, 'false');
        }
      ).catch((error) => {
        console.error('Auth initialization error:', error);
        sessionStorage.setItem(SEC_ADV_SIGN_IN_INIT_KEY, 'false');
        dispatch(logout());
      });
    } else if (!isSignInInitiated) {
      sessionStorage.setItem(SEC_ADV_SIGN_IN_INIT_KEY, 'true');
      const path = window.location.pathname + window.location.search;
      if (pathnameEndsWithPdf(window.location.pathname)) {
        sessionStorage.setItem(SEC_ADV_REDIRECT_PATH_KEY, path);
      }
      signIn();
    } else if (pathnameEndsWithPdf(window.location.pathname)) {
      sessionStorage.setItem(SEC_ADV_REDIRECT_PATH_KEY, window.location.pathname + window.location.search);
      signIn();
    }
  }, [state.isAuthenticated, dispatch, getBasicUserInfo, getIDToken, signIn]);

  const appSignOut = async () => {
    try {
      dispatch(setLoading(true));
      await signOut();
      dispatch(logout());
      sessionStorage.setItem(SEC_ADV_SIGN_IN_INIT_KEY, 'false');
    } catch (error) {
      console.error('Sign out error:', error);
    } finally {
      dispatch(setLoading(false));
    }
  };

  const appSignIn = () => {
    signIn();
  };

  const authContext: AppAuthContextType = {
    appSignIn,
    appSignOut,
  };

  return (
    <AppAuthContext.Provider value={authContext}>
      <SecureApp
        fallback={
          <Box
            sx={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100vh',
              bgcolor: '#f5f5f5',
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
        }
      >
        {children}
      </SecureApp>
    </AppAuthContext.Provider>
  );
};

export const useAppAuth = (): AppAuthContextType => {
  const context = useContext(AppAuthContext);
  if (!context) {
    throw new Error('useAppAuth must be used within AppAuthProvider');
  }
  return context;
};

export default AppAuthProvider;
