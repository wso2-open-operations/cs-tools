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

import LoginInvertedImage from "@/assets/images/login-page/login-screen-inverted.svg";
import LoginImage from "@/assets/images/login-page/login-screen.svg";
import { ColorSchemeImage } from "@wso2/oxygen-ui";
import { type JSX } from "react";

export default function LoginBackground(): JSX.Element {
  return (
    <ColorSchemeImage
      src={{
        light: LoginImage,
        dark: LoginInvertedImage,
      }}
      alt={{
        light: "Login Screen Image (Light)",
        dark: "Login Screen Image (Dark)",
      }}
      height={450}
      width="auto"
      sx={{ position: "absolute", bottom: 50, right: -100 }}
    />
  );
}
