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

interface NotFoundPageProps {
  username?: string;
  onLogout: () => void;
}

const NotFoundPage: React.FC<NotFoundPageProps> = ({ username, onLogout }) => {
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
        <Paper elevation={1} sx={{ maxWidth: 480, p: 4, width: '100%', textAlign: 'center' }}>
          <Typography variant="h4" component="h1" gutterBottom sx={{ fontWeight: 600 }}>
            404
          </Typography>
          <Typography variant="h6" color="text.secondary" gutterBottom>
            Page not found
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Open a valid link that ends with <code>.pdf</code>, or the file may be missing from storage.
          </Typography>
        </Paper>
      </Box>
    </Box>
  );
};

export default NotFoundPage;
