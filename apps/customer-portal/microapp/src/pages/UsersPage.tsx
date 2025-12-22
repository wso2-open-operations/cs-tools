// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { Add, Search } from "@mui/icons-material";
import {
  Box,
  Card,
  Grid,
  InputBase as TextField,
  Stack,
  Typography,
  InputAdornment,
  ButtonBase as Button,
  Divider,
} from "@mui/material";

export default function UsersPage() {
  return (
    <Box p={2} mb={20}>
      <Grid spacing={1.5} container>
        <Grid size={4}>
          <Card component={Stack} p={1.2} gap={0.5} elevation={0}>
            <Typography variant="h4" fontWeight="bold">
              12
            </Typography>
            <Typography variant="subtitle1" fontWeight="medium" color="text.secondary">
              Total Users
            </Typography>
          </Card>
        </Grid>
        <Grid size={4}>
          <Card component={Stack} p={1.2} gap={0.5} elevation={0}>
            <Typography variant="h4" fontWeight="bold">
              4
            </Typography>
            <Typography variant="subtitle1" fontWeight="medium" color="text.secondary">
              Active
            </Typography>
          </Card>
        </Grid>
        <Grid size={4}>
          <Card component={Stack} p={1.2} gap={0.5} elevation={0}>
            <Typography variant="h4" fontWeight="bold">
              1
            </Typography>
            <Typography variant="subtitle1" fontWeight="medium" color="text.secondary">
              Admins
            </Typography>
          </Card>
        </Grid>
        <Grid size={12}>
          <TextField
            fullWidth
            type="search"
            placeholder="Search Users"
            startAdornment={
              <InputAdornment position="start">
                <Search />
              </InputAdornment>
            }
            sx={{
              mt: 1,
              bgcolor: "background.paper",
            }}
          />
        </Grid>
      </Grid>
      <Card component={Stack} p={2} mt={2} gap={0.5} elevation={0}>
        <Stack direction="row" justifyContent="space-between" pb={1}>
          <Typography variant="h6" fontWeight="bold">
            All Users
          </Typography>
          <Button component={Stack} direction="row" gap={2} sx={{ padding: 0 }} disableRipple>
            <Typography variant="body2" color="primary" fontWeight="medium">
              Add
            </Typography>
            <Add color="primary" />
          </Button>
        </Stack>
        <Divider />
        <Stack gap={2} pt={1} divider={<Divider />}></Stack>
      </Card>
    </Box>
  );
}
