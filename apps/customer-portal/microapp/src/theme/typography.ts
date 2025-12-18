// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

export const pxToRem = (px: number) => `${px / 16}rem`;

export const typography = {
  fontFamily: '"Plus Jakarta Sans", "Roboto", "Arial", sans-serif',
  fontSize: 14,
  fontWeightLight: 300,
  fontWeightRegular: 400,
  fontWeightMedium: 550,
  fontWeightBold: 600,
  h1: {
    fontSize: pxToRem(40),
  },
  h2: {
    fontSize: pxToRem(32),
  },
  h3: {
    fontSize: pxToRem(28),
  },
  h4: {
    fontSize: pxToRem(23),
  },
  h5: {
    fontSize: pxToRem(20),
  },
  h6: {
    fontSize: pxToRem(17),
  },
  subtitle1: {
    fontSize: pxToRem(15),
  },
  subtitle2: {
    fontSize: pxToRem(14),
  },
  body1: {
    fontSize: pxToRem(16),
  },
  body2: {
    fontSize: pxToRem(15.5),
  },
  button: {
    fontSize: pxToRem(16),
  },
  caption: {
    fontSize: pxToRem(12),
  },
  overline: {
    fontSize: pxToRem(11),
  },
};
