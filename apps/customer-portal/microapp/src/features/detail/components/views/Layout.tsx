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
import { type ReactNode } from "react";

import { Stack, Typography } from "@wso2/oxygen-ui";

import { useDeclareLayout } from "@context/layout";

import { CommentBar, SlotActions, type SlotActionsOptionProps, SlotTitle } from "@features/detail/components";
import { useTitleSlotVariant } from "@features/detail/hooks";

import { COMMENT_ENABLED_TYPES, Tab } from "@shared/constants";
import type { CaseType } from "@shared/types";

type LayoutProps = {
  type: CaseType;
  title?: string;
  id?: string;
  children: ReactNode;

  /** Only pass for CASE_TYPES.DEFAULT */
  actions?: SlotActionsOptionProps[];
};

export function Layout({ type, title, id, children, actions }: LayoutProps) {
  const { ref, variant } = useTitleSlotVariant();

  useDeclareLayout(
    {
      tabIndex: Tab.Support,
      title: <SlotTitle variant={variant} type={type} id={id} title={title} />,
      visibility: {
        backAction: true,
      },
      slots: {
        trailing: actions ? <SlotActions disabled={actions.every((o) => o.hidden)} options={actions} /> : undefined,
      },
    },
    { enabled: true },
    [type, title, id, actions, variant],
  );

  return (
    <>
      <Stack gap={2} mb={10}>
        <Typography ref={ref} variant="h5" fontWeight="medium">
          {title}
        </Typography>
        {children}
      </Stack>

      {COMMENT_ENABLED_TYPES.includes(type) && <CommentBar />}
    </>
  );
}
