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

import { useProject } from "@root/src/context/project";
import { Fab as MuiFab, useTheme } from "@wso2/oxygen-ui";
import { MessageSquareIcon } from "@wso2/oxygen-ui-icons-react";
import { useNavigate } from "react-router-dom";

export function Fab() {
  const navigate = useNavigate();
  const theme = useTheme();
  const { noveraEnabled } = useProject();

  return (
    <MuiFab
      role="link"
      size="medium"
      variant="circular"
      color="primary"
      sx={{
        textTransform: "initial",
        position: "fixed",
        right: 10,
        bottom: "calc(var(--tab-bar-height) + 60px)",
      }}
      onClick={(event) => {
        event.preventDefault();
        navigate(noveraEnabled ? "/chat" : "/create");
      }}
    >
      <MessageSquareIcon fill={theme.palette.primary.contrastText} />
    </MuiFab>
  );
}
