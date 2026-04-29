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

import { ArrowLeftRightIcon, CheckIcon, PlusIcon } from "@wso2/oxygen-ui-icons-react";
import type { MenuOptionProps } from "@components/detail";

export function getCaseMenuOptions(
  stateKey: string,
  actions?: Partial<{
    onResolve: () => void;
    onMarkWaiting: () => void;
    onCreateRelated: () => void;
  }>,
): MenuOptionProps[] {
  return [
    {
      label: "Mark as Resolved",
      color: "success",
      icon: <CheckIcon />,
      hidden: !["1", "10", "1003", "6", "18", "1006"].includes(stateKey),
      onClick: actions?.onResolve,
    },
    {
      label: "Mark as Waiting on WSO2",
      color: "warning",
      icon: <ArrowLeftRightIcon />,
      hidden: !["6", "18"].includes(stateKey),
      onClick: actions?.onMarkWaiting,
    },
    {
      label: "Created Related Case",
      icon: <PlusIcon />,
      hidden: !["3"].includes(stateKey),
      onClick: actions?.onCreateRelated,
    },
  ];
}
