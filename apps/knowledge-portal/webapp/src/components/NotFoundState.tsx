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

import { Button, Container, Stack, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link } from "react-router";
import { useDocumentTitle } from "@hooks/useDocumentTitle";

interface NotFoundStateProps {
  title?: string;
  message?: string;
}

/** Full-page "not found". Owns the page's h1 and document title. */
export default function NotFoundState({
  title = "Page not found",
  message = "The link may be incorrect, or the page may have been removed.",
}: NotFoundStateProps): JSX.Element {
  useDocumentTitle(title);
  return (
    <Container maxWidth="lg" sx={{ py: 8 }}>
      <Stack spacing={2} alignItems="flex-start">
        <Typography variant="h5" component="h1" fontWeight={700}>
          {title}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {message}
        </Typography>
        <Button
          component={Link}
          to="/"
          variant="outlined"
          startIcon={<ArrowLeft size={16} aria-hidden />}
        >
          Back to knowledge base
        </Button>
      </Stack>
    </Container>
  );
}
