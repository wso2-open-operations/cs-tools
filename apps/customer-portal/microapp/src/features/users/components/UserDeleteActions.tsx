import { useState } from "react";

import { alpha, Button, Card, CircularProgress, Stack, Typography } from "@wso2/oxygen-ui";
import { Trash2 } from "@wso2/oxygen-ui-icons-react";

import { useUserMutations } from "@features/users/hooks";

import { ConfirmDialog } from "@shared/components/common";

export function UserDeleteActions() {
  const { remove } = useUserMutations();
  const [open, setOpen] = useState(false);

  const handleConfirm = () => {
    setOpen(false);
    remove.mutate();
  };

  return (
    <>
      <Card component={Stack} sx={(theme) => ({ bgcolor: alpha(theme.palette.error.main, 0.2), p: 1.5 })}>
        <Typography variant="body2" fontWeight="medium" color="error">
          Danger Zone
        </Typography>
        <Typography variant="subtitle2" color="text.secondary">
          Send an email invitation directly to a user to join this project. The invitation link will be valid for 7
          days.
        </Typography>

        <Button
          variant="contained"
          color="error"
          disabled={remove.isPending}
          startIcon={remove.isPending ? <CircularProgress size={16} color="inherit" /> : <Trash2 />}
          sx={{ mt: 3 }}
          onClick={() => setOpen(true)}
        >
          {remove.isPending ? "Removing..." : "Remove User from Project"}
        </Button>
      </Card>

      <ConfirmDialog
        open={open}
        title="Remove User"
        description="Are you sure you want to remove this user?"
        confirmColor="error"
        confirmLabel="Remove"
        onClose={() => setOpen(false)}
        onConfirm={handleConfirm}
      />
    </>
  );
}
