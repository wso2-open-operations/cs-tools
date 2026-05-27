import { useState } from "react";

import { goToMyAppsScreen } from "@src/bridge";
import { IconButton, Typography } from "@wso2/oxygen-ui";
import { Grip } from "@wso2/oxygen-ui-icons-react";

import { ConfirmDialog } from "@shared/components/common";

export function ExitButton() {
  const [open, setOpen] = useState(false);

  return (
    <>
      <IconButton disableRipple color="error" sx={{ gap: 1, p: 0, m: 0.8 }} onClick={() => setOpen(true)}>
        <Grip size={20} />
        <Typography>Go to Apps</Typography>
      </IconButton>

      <ConfirmDialog
        open={open}
        title="Return to Apps"
        description="Are you sure you want to leave this application?"
        confirmColor="error"
        confirmLabel="Leave"
        onClose={() => setOpen(false)}
        onConfirm={goToMyAppsScreen}
      />
    </>
  );
}
