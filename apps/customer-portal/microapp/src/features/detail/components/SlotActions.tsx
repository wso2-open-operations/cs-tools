import { type MouseEvent, type ReactNode, useState } from "react";

import { Button, Divider, IconButton, Popover, Stack, Typography } from "@wso2/oxygen-ui";
import { Ellipsis } from "@wso2/oxygen-ui-icons-react";

interface SlotActionsProps {
  title?: string;
  options?: SlotActionsOptionProps[];
  disabled?: boolean;
}

export interface SlotActionsOptionProps {
  label: string;
  icon: ReactNode;
  color?: "inherit" | "primary" | "secondary" | "success" | "error" | "info" | "warning";
  hidden?: boolean;
  onClick?: () => void;
}

export function SlotActions({ title = "More Options", options = [], disabled }: SlotActionsProps) {
  const [anchorEl, setAnchorEl] = useState<HTMLButtonElement | null>(null);

  const handleClick = (event: MouseEvent<HTMLButtonElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  const open = Boolean(anchorEl);

  return (
    <>
      <IconButton disabled={disabled} color="inherit" onClick={handleClick}>
        <Ellipsis />
      </IconButton>

      <Popover
        open={open}
        anchorEl={anchorEl}
        onClose={handleClose}
        anchorOrigin={{
          vertical: "bottom",
          horizontal: "right",
        }}
        transformOrigin={{
          vertical: "top",
          horizontal: "right",
        }}
      >
        <Stack divider={<Divider />}>
          <Typography variant="caption" sx={{ px: 1, py: 0.5, opacity: "80%" }}>
            {title}
          </Typography>

          {options.map((option, index) => {
            if (option.hidden) return;

            return (
              <Button
                key={index}
                variant="text"
                color={option.color ?? "inherit"}
                component={Stack}
                direction="row"
                sx={{ px: 1, borderRadius: 0, gap: 2, justifyContent: "space-between" }}
                onClick={() => {
                  handleClose();
                  if (option.onClick) option.onClick();
                }}
              >
                {option.label}
                {option.icon}
              </Button>
            );
          })}
        </Stack>
      </Popover>
    </>
  );
}
