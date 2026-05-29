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
    remove?.mutate();
  };

  return (
    <>
      <Card component={Stack} sx={(theme) => ({ bgcolor: alpha(theme.palette.error.main, 0.2), p: 1.5 })}>
        <Typography variant="body2" fontWeight="medium" color="error">
          Danger Zone
        </Typography>
        <Typography variant="subtitle2" color="text.secondary">
          Irreversible actions that permanently affect this user. Please proceed with caution.
        </Typography>

        <Button
          variant="contained"
          color="error"
          disabled={remove?.isPending}
          startIcon={remove?.isPending ? <CircularProgress size={16} color="inherit" /> : <Trash2 />}
          sx={{ mt: 3 }}
          onClick={() => setOpen(true)}
        >
          {remove?.isPending ? "Removing..." : "Remove User from Project"}
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
