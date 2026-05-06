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
import ReactDOM from 'react-dom/client';
import './index.css';
import App from './App';
import { SEC_ADV_REDIRECT_PATH_KEY, SEC_ADV_SIGN_IN_INIT_KEY } from './constants/constants';

// Stash /patches/... before any React or Asgardeo logic runs (OAuth returns to "/" and must restore this).
// Clear per-tab sign-in gate so a pasted deep link always triggers signIn (localStorage used to block other tabs).
try {
  const path = window.location.pathname;
  if (path.startsWith('/patches')) {
    sessionStorage.setItem(SEC_ADV_REDIRECT_PATH_KEY, path + window.location.search);
    sessionStorage.removeItem(SEC_ADV_SIGN_IN_INIT_KEY);
  }
} catch {
  /* ignore private mode / quota */
}

const rootElement = document.getElementById('root');

if (!rootElement) {
  throw new Error('Root element not found');
}

const root = ReactDOM.createRoot(rootElement);
root.render(<App />);
