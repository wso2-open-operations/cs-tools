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

// Routed at both /spl/accounts (all) and /spl/my-accounts — which one is
// active is read from the path itself, same as the source app.
import { useState } from "react";
import { useLocation } from "react-router";
import { Typography } from "@wso2/oxygen-ui";
import type { ToggleSwitchState } from "../api/accountTypes";
import ToggleSwitch from "../components/ToggleSwitch";
import ListAccounts from "../components/ListAccounts";
import ListMyAccounts from "../components/ListMyAccounts";

export default function AccountsPage() {
  const [toggleSwitchState, setToggleSwitchState] = useState<ToggleSwitchState>("active-accounts");
  const location = useLocation();

  const leaf = location.pathname.split("/").filter(Boolean).pop();
  const isMyAccounts = leaf === "my-accounts";

  const active = toggleSwitchState === "active-accounts";

  return (
    <>
      <Typography variant="h5" sx={{ mb: 2 }}>
        {isMyAccounts ? "My accounts" : "All accounts"}
      </Typography>
      <ToggleSwitch page={toggleSwitchState} onSwitchClick={setToggleSwitchState} />
      {isMyAccounts ? <ListMyAccounts active={active} /> : <ListAccounts active={active} />}
    </>
  );
}
