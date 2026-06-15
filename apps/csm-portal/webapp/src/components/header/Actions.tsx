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

import {
  Box,
  ColorSchemeToggle,
  Divider,
  Header as HeaderUI,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { useAsgardeo } from "@asgardeo/react";
import UserProfile from "@components/header/UserProfile";
import MockModeToggle from "@components/header/MockModeToggle";
import ThemeSelect from "@components/header/ThemeSelect";
import RecentViewsButton from "@features/csm-recent/components/RecentViewsButton";
import PinThisPageButton from "@features/csm-recent/components/PinThisPageButton";

export default function Actions(): JSX.Element {
  const { isSignedIn } = useAsgardeo();

  return (
    <HeaderUI.Actions>
      <MockModeToggle />
      <ThemeSelect />
      <ColorSchemeToggle />
      {isSignedIn && <PinThisPageButton />}
      {isSignedIn && <RecentViewsButton />}
      <Divider
        orientation="vertical"
        flexItem
        sx={{
          mx: 1,
          display: { xs: "none", sm: "block" },
          visibility: isSignedIn ? "visible" : "hidden",
        }}
      />
      {isSignedIn ? <UserProfile /> : <Box sx={{ width: 40, height: 40 }} />}
    </HeaderUI.Actions>
  );
}
