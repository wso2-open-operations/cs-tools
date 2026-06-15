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

import { type JSX } from "react";
import { Header as HeaderUI } from "@wso2/oxygen-ui";
import Brand from "@components/header/Brand";
import Actions from "@components/header/Actions";
import PinnedTabs from "@features/csm-recent/components/PinnedTabs";
import QuickNav from "@features/csm-recent/components/QuickNav";

interface HeaderProps {
  onToggleSidebar: () => void;
  collapsed?: boolean;
  hideProjectControls?: boolean;
}

export default function Header({
  onToggleSidebar,
  collapsed = false,
  hideProjectControls = false,
}: HeaderProps): JSX.Element {
  return (
    <HeaderUI>
      {!hideProjectControls && (
        <HeaderUI.Toggle collapsed={collapsed} onToggle={onToggleSidebar} />
      )}
      <Brand />
      <QuickNav />
      <PinnedTabs />
      <Actions />
    </HeaderUI>
  );
}
