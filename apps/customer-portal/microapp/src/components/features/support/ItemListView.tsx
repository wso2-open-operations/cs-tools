import { Link } from "react-router-dom";
import { Stack, Typography, Divider, Button, useTheme, pxToRem } from "@wso2/oxygen-ui";
import { ChevronRight } from "@wso2/oxygen-ui-icons-react";
import type { ReactNode } from "react";

interface ItemListViewProps {
  title: string;
  viewAllPath: string;
  children: ReactNode;
}

export function ItemListView({ title, viewAllPath, children }: ItemListViewProps) {
  const theme = useTheme();

  return (
    <>
      <Stack direction="row" justifyContent="space-between" alignItems="center" pb={1}>
        <Typography variant="h6">{title}</Typography>
        <Button variant="text" component={Link} to={viewAllPath} sx={{ textTransform: "initial" }}>
          <Stack direction="row" gap={1}>
            <Typography variant="body1" color="primary">
              View All
            </Typography>
            <ChevronRight size={pxToRem(18)} color={theme.palette.primary.main} />
          </Stack>
        </Button>
      </Stack>
      <Divider />
      <Stack gap={2} pt={2}>
        {children}
      </Stack>
    </>
  );
}
