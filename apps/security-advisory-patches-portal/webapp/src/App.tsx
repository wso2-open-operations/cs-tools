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

import { useEffect } from 'react';
import { Provider } from 'react-redux';
import { BrowserRouter } from 'react-router-dom';
import { ThemeProvider, CssBaseline } from '@mui/material';
import { AuthProvider } from '@asgardeo/auth-react';
import { SnackbarProvider } from 'notistack';
import { store } from './slices/store';
import { theme } from './theme';
import { AsgardeoConfig, APP_NAME } from './config/config';
import AppAuthProvider from './context/AuthContext';
import AppHandler from './app/AppHandler';

function App() {
  useEffect(() => {
    document.title = APP_NAME;
  }, []);

  return (
    <Provider store={store}>
      <BrowserRouter>
        <ThemeProvider theme={theme}>
          <CssBaseline />
          <SnackbarProvider maxSnack={3} preventDuplicate>
            <AuthProvider config={AsgardeoConfig}>
              <AppAuthProvider>
                <AppHandler />
              </AppAuthProvider>
            </AuthProvider>
          </SnackbarProvider>
        </ThemeProvider>
      </BrowserRouter>
    </Provider>
  );
}

export default App;
