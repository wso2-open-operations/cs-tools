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

import { Box, Divider, Link, Stack, Typography } from "@wso2/oxygen-ui";
import { type JSX } from "react";

export default function LoginFooter(): JSX.Element {
  return (
    <Box component="footer" sx={{ mt: 8 }}>
      <Typography sx={{ textAlign: "center" }}>
        © Copyright {new Date().getFullYear()} WSO2 LLC.
      </Typography>
      <Stack direction="row" justifyContent="center" sx={{ mt: 2 }} spacing={1}>
        <Link
          href="https://wso2.com/privacy-policy/"
          color="inherit"
          underline="hover"
          target="_blank"
          rel="noopener noreferrer"
        >
          Privacy Policy
        </Link>
        <Divider orientation="vertical" flexItem sx={{ mx: 1 }} />
        <Link
          href="https://wso2.com/terms-of-use/"
          color="inherit"
          underline="hover"
          target="_blank"
          rel="noopener noreferrer"
        >
          Terms of Use
        </Link>
      </Stack>
    </Box>
  );
}
