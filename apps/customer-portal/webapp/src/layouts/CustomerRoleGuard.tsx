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

import { type JSX, type ReactNode } from "react";
import { Outlet } from "react-router";
import { Box, LinearProgress, Typography } from "@wso2/oxygen-ui";
import Error403Page from "@components/error/Error403Page";
import ErrorLayout from "@layouts/ErrorLayout";
import {
  useCustomerPermissions,
  type CanonicalRole,
  type CustomerAction,
  type CustomerModule,
} from "@hooks/useCustomerPermissions";

export interface CustomerRoleGuardProps {
  module?: CustomerModule;
  action?: CustomerAction;
  requiredRoles?: CanonicalRole[];
  children?: ReactNode;
}

/**
 * Route & container guard that verifies permissions based on the customer portal RBAC matrix.
 * Unauthorized accesses render the standardized 403 Forbidden page.
 */
export default function CustomerRoleGuard({
  module,
  action,
  requiredRoles,
  children,
}: CustomerRoleGuardProps): JSX.Element {
  const { can, hasAnyRole, isLoading, isError } = useCustomerPermissions();

  if (isLoading) {
    return (
      <Box
        sx={{
          alignItems: "center",
          display: "flex",
          flex: 1,
          justifyContent: "center",
          py: 8,
        }}
      >
        <LinearProgress sx={{ maxWidth: 400, width: "60%" }} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box
        sx={{
          alignItems: "center",
          display: "flex",
          flex: 1,
          justifyContent: "center",
          py: 8,
        }}
      >
        <Typography color="text.secondary" variant="body2">
          Unable to verify access permissions. Please refresh and try again.
        </Typography>
      </Box>
    );
  }

  let isAuthorized = true;

  if (module && action) {
    isAuthorized = can(module, action);
  }

  if (isAuthorized && requiredRoles && requiredRoles.length > 0) {
    isAuthorized = hasAnyRole(requiredRoles);
  }

  if (!isAuthorized) {
    return (
      <ErrorLayout>
        <Error403Page />
      </ErrorLayout>
    );
  }

  return children ? <>{children}</> : <Outlet />;
}
