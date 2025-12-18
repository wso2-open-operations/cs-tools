import React from "react";
import type { IconProps } from "../../../../types/icon.types";
import { BaseIcon } from "../../BaseIcon";

export const FolderIcon: React.FC<IconProps> = (props) => (
  <BaseIcon {...props}>
    <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" />
  </BaseIcon>
);
