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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { Crown } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

/**
 * Subscription widget component props interface.
 */
interface SubscriptionWidgetProps {
  collapsed?: boolean;
}

/**
 * Subscription widget component.
 *
 * @param {SubscriptionWidgetProps} props - Props injected to the subscription widget component.
 * @returns {JSX.Element | null} - Subscription widget component.
 */
const SubscriptionWidget = ({
  collapsed,
}: SubscriptionWidgetProps): JSX.Element | null => {
  if (collapsed) {
    return null;
  }

  return (
    <>
      {/* subscription widget container */}
      <Box
        sx={{
          p: 1.5,
          m: 1.5,
          border: "1px solid",
          borderColor: "divider",
        }}
      >
        {/* subscription widget header */}
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            mb: 0.5,
          }}
        >
          <Crown size={16} />
          <Typography variant="body2" sx={{ fontWeight: 500 }}>
            Subscription
          </Typography>
        </Box>
        {/* subscription widget info */}
        <Box sx={{ mb: 1.5 }}>
          <Typography variant="caption" display="block">
            Information about
          </Typography>
          <Typography variant="caption" display="block">
            subscription
          </Typography>
        </Box>
        {/* subscription widget action button */}
        <Button variant="outlined" size="small" fullWidth color="primary">
          View Details
        </Button>
      </Box>
    </>
  );
};

export default SubscriptionWidget;
