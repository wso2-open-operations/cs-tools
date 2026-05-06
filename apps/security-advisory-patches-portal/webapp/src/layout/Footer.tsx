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
import { Box, Typography } from '@mui/material';

const Footer: React.FC = () => {
  const currentYear = new Date().getFullYear();
  
  return (
    <Box
      component="footer"
      sx={{
        py: 2,
        px: 3,
        mt: 'auto',
        backgroundColor: 'background.paper',
        borderTop: '1px solid',
        borderColor: 'divider',
      }}
    >
      <Typography variant="body2" color="text.secondary" align="center">
        © {currentYear} WSO2 LLC. All Rights Reserved.
      </Typography>
    </Box>
  );
};

export default Footer;
