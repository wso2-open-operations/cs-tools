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

import * as React from "react";
import { Tab, Tabs } from "@wso2/oxygen-ui";
import type { ToggleSwitchState } from "../api/accountTypes";

type ToggleSwitchProps = {
  onSwitchClick?: (state: ToggleSwitchState) => void;
  page: ToggleSwitchState;
};

export default function ToggleSwitch(props: ToggleSwitchProps) {
  const [value, setValue] = React.useState(0);

  const handleChange = (_event: React.SyntheticEvent, newValue: number) => {
    setValue(newValue);
  };

  const handleClick = (state: ToggleSwitchState) => {
    props.onSwitchClick?.(state);
  };

  return (
    <div style={{ border: "1px solid #ff7300", borderRadius: 25, height: "49.5px", width: "fit-content" }}>
      <Tabs
        sx={{
          justifyContent: "flex-start",
          "& button": { borderRadius: 10, boxShadow: "insert", zIndex: 1 },
          "& button.Mui-selected": { color: "#fff" },
        }}
        value={value}
        onChange={handleChange}
        slotProps={{ indicator: { sx: { backgroundColor: "#ff7300", height: "100%", borderRadius: 10 } } }}
      >
        <Tab onClick={() => handleClick("active-accounts")} label="Active Accounts" />
        <Tab onClick={() => handleClick("all-accounts")} label="All Accounts" />
      </Tabs>
    </div>
  );
}
