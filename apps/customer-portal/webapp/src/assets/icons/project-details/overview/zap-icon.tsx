import React from "react";
import type { IconProps } from "../../../../types/icon.types";
import { BaseIcon } from "../../BaseIcon";

export const ZapIcon: React.FC<IconProps> = (props) => (
  <BaseIcon {...props}>
    <path d="M4 14a1 1 0 0 1-.78-1.63l9.9-10.2a.5.5 0 0 1 .86.46l-1.92 6.02A1 1 0 0 0 13 10h7a1 1 0 0 1 .78 1.63l-9.9 10.2a.5.5 0 0 1-.86-.46l1.92-6.02A1 1 0 0 0 11 14z" />
  </BaseIcon>
);
