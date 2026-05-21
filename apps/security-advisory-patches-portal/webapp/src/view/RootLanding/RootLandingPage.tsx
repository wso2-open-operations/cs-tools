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

import React from 'react';
import { Box, Paper, Typography } from '@mui/material';
import Header from '@src/layout/Header';

interface RootLandingPageProps {
  username?: string;
  onLogout: () => void;
}

/**
 * Shown at site origin (`/`) after sign-in when the user opened the portal without a PDF advisory link.
 */
const RootLandingPage: React.FC<RootLandingPageProps> = ({ username, onLogout }) => {
  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#f5f5f5', display: 'flex', flexDirection: 'column' }}>
      <Header username={username} onLogout={onLogout} />
      <Box
        sx={{
          flex: 1,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          p: 3,
        }}
      >
        <Paper elevation={1} sx={{ maxWidth: 560, p: 4, width: '100%' }}>
          <Typography variant="h5" component="h1" gutterBottom sx={{ fontWeight: 600 }}>
            Use your advisory PDF link
          </Typography>
          <Typography variant="body1" color="text.secondary" paragraph>
            This site only opens a PDF when you use the link you were sent—the address path must end with{' '}
            <code>.pdf</code> (for example <code>/patches/your-doc.pdf</code> or another path your team uses).
          </Typography>
          <Typography variant="body1" color="text.secondary" paragraph>
            Paste that full URL into the address bar (for example{' '}
            <code>patches.wso2.com/patches/your-doc.pdf</code>), or sign out below and open the link from your email again.
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Signing out and opening your link again is the most reliable way to reach the right document.
          </Typography>
        </Paper>
      </Box>
    </Box>
  );
};

export default RootLandingPage;
