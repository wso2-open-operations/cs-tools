import { capitalize } from "@utils/others";
import { Chip, type ChipProps } from "@wso2/oxygen-ui";
import type { Priority, Status } from "./ItemCard";

import { PRIORITY_CHIP_COLOR_CONFIG, STATUS_CHIP_COLOR_CONFIG } from "./config";

interface PriorityChipProps extends Omit<ChipProps, "label"> {
  prefix?: string;
  priority: Priority;
}

export function PriorityChip({ prefix, priority, ...props }: PriorityChipProps) {
  const color = PRIORITY_CHIP_COLOR_CONFIG[priority];

  return <Chip color={color} label={`${prefix ? `${prefix}: ` : ""}${capitalize(priority)}`} {...props} />;
}

interface StatusChipProps extends Omit<ChipProps, "label"> {
  status: Status;
}

export function StatusChip({ status, ...props }: StatusChipProps) {
  const color = STATUS_CHIP_COLOR_CONFIG[status];

  return <Chip color={color} label={capitalize(status)} {...props} />;
}
