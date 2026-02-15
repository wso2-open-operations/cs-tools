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
//

import React, { createContext, useContext, useEffect } from 'react';
import { SecureApp, useAuthContext } from '@asgardeo/auth-react';
import { Box, CircularProgress } from '@mui/material';
import { useAppDispatch } from '@src/slices/store';
import { setAuthenticated, setUser, setLoading } from '@src/slices/authSlice/auth';
import { APIService } from '@src/utils/apiService';
import { UserInfo } from '@src/types/types';

interface AppAuthContextType {
  appSignIn: () => void;
  appSignOut: () => void;
}

const AppAuthContext = createContext<AppAuthContextType | undefined>(undefined);

export const AppAuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const dispatch = useAppDispatch();
  const { state, signIn, signOut, getBasicUserInfo, getIDToken } = useAuthContext();

  useEffect(() => {
    const isSignInInitiated = localStorage.getItem('signInInitiated') === 'true';
    
    if (state.isAuthenticated) {
      Promise.all([getBasicUserInfo(), getIDToken()]).then(
        async ([basicUserInfo, idToken]) => {
          const userInfo: UserInfo = {
            username: basicUserInfo?.username || basicUserInfo?.email || 'User',
            email: basicUserInfo?.email || '',
            sub: basicUserInfo?.sub || '',
          };

          dispatch(setUser(userInfo));
          dispatch(setAuthenticated(true));
          dispatch(setLoading(false));

          // Initialize API service with token
          new APIService(idToken || '', async () => {
            const token = await getIDToken();
            return { idToken: token || '' };
          });
          
          localStorage.setItem('signInInitiated', 'false');
        }
      ).catch((error) => {
        console.error('Auth initialization error:', error);
        dispatch(setAuthenticated(false));
        dispatch(setLoading(false));
      });
    } else if (!isSignInInitiated) {
      localStorage.setItem('signInInitiated', 'true');
      signIn();
    }
  }, [state.isAuthenticated, dispatch, getBasicUserInfo, getIDToken, signIn]);

  const appSignOut = async () => {
    try {
      dispatch(setLoading(true));
      await signOut();
      dispatch(setAuthenticated(false));
      dispatch(setUser(null));
      localStorage.setItem('signInInitiated', 'false');
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

