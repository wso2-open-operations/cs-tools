import { Box, Collapse, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import { ListChevronsDownUp, ListChevronsUpDown } from "@wso2/oxygen-ui-icons-react";
import { useState, type ReactNode } from "react";
import { TYPE_CONFIG } from "../features/support/config";
import type { ItemCardProps } from "../features/support";
import { PLURALS } from "@root/src/config/constants";

export function GroupAccordion({
  type,
  count,
  children,
}: {
  type: ItemCardProps["type"];
  count?: number;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(true);
  const { icon: Icon } = TYPE_CONFIG[type];

  return (
    <Box
      sx={{
        bgcolor: "transparent",
        pb: 1,
        mx: -2,
        borderBottom: "3px solid",
        borderColor: "divider",
        display: count === 0 ? "none" : "block",
      }}
    >
      <Stack
        direction="row"
        onClick={() => setOpen(!open)}
        sx={{
          cursor: "pointer",
          mb: 1,
          justifyContent: "space-between",
          p: 1,
        }}
      >
        <Stack direction="row" alignItems="center" gap={1}>
          <Box color="text.secondary">
            <Icon size={pxToRem(18)} />
          </Box>
          <Typography variant="body1" sx={{ flex: 1 }}>
            {PLURALS[type]}
          </Typography>
        </Stack>
        <Stack direction="row" gap={1} alignItems="center">
          {count !== undefined && count}
          {open ? <ListChevronsDownUp size={pxToRem(18)} /> : <ListChevronsUpDown size={pxToRem(18)} />}
        </Stack>
      </Stack>
      <Collapse in={open}>
        <Stack gap={2} px={2} py={1}>
          {children}
        </Stack>
      </Collapse>
    </Box>
  );
}
