import { type ReactNode } from "react";

import { Stack, Typography } from "@wso2/oxygen-ui";

import { useDeclareLayout } from "@context/layout";

import { CommentBar, SlotActions, SlotTitle } from "@features/detail/components";
import { useTitleSlotVariant } from "@features/detail/hooks";

import { type MenuOptionProps } from "@shared/components/detail";

import { COMMENT_ENABLED_TYPES } from "@shared/constants";
import type { CaseType } from "@shared/types";

type LayoutProps = {
  type: CaseType;
  title?: string;
  id?: string;
  children: ReactNode;

  /** Only pass for CASE_TYPES.DEFAULT */
  actions?: MenuOptionProps[];
};

export function Layout({ type, title, id, children, actions }: LayoutProps) {
  const { ref, variant } = useTitleSlotVariant();

  useDeclareLayout({
    title: <SlotTitle variant={variant} type={type} id={id} title={title} />,
    slots: {
      trailing: actions ? <SlotActions disabled={actions.every((o) => o.hidden)} options={actions} /> : undefined,
    },
  });

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
