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

// The account detail screen's four tabs (Basic Info / Projects /
// Escalations / Artifacts Repository).
import { useState, type ReactNode } from "react";
import {
  Box,
  Button,
  CircularProgress,
  Paper,
  Stack,
  Tab,
  Tabs,
  Table,
  TableBody,
  TableCell,
  TableRow,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { AlertTriangleIcon, FileTextIcon } from "@wso2/oxygen-ui-icons-react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { usePermissions } from "@features/spl/api/permissionsContext";
import { useGetSplAccount } from "../api/useAccountsApi";
import type { AccountDetails } from "../api/accountTypes";
import ListAccountProjects from "./ListAccountProjects";
import ListAccountEscalations from "./ListAccountEscalations";
import ListAccountSolutionDocument from "./ListAccountSolutionDocument";
import EscalateDialog from "./EscalateDialog";
import PathView from "./PathView";
import TeamMembersDrawer from "./TeamMembersDrawer";
import { openExternalUrl } from "@utils/openExternalUrl";

function TabPanel({ children, value, index }: { children: ReactNode; value: number; index: number }) {
  return (
    <div role="tabpanel" hidden={value !== index}>
      {value === index && <Box sx={{ p: 3 }}>{children}</Box>}
    </div>
  );
}

export default function ListAccountDetail({ id }: { id: string }) {
  const [value, setTabValue] = useState(0);
  const { data, isLoading, error } = useGetSplAccount(id);

  return (
    <>
      {isLoading ? (
        <Stack alignItems="center" sx={{ mt: 4 }}>
          <CircularProgress size={20} />
        </Stack>
      ) : error ? (
        <Typography color="text.secondary" sx={{ mt: 2 }}>
          Couldn't load this account.
        </Typography>
      ) : (
        data && (
          <>
            <PathView accountName={data.name} accountNumber={data.id} />
            <AccountHeading data={data} showViewOnDriveButton={value === 3} />
            <Tabs value={value} onChange={(_e, v: number) => setTabValue(v)} aria-label="Account Tabs">
              <Tab label="Basic Info" />
              <Tab label="Projects" />
              <Tab label="Escalations" />
              <Tab label="Artifacts Repository" />
            </Tabs>
            <Paper>
              <TabPanel value={value} index={0}>
                <BasicInfoTable data={data} />
              </TabPanel>
              <TabPanel value={value} index={1}>
                <ListAccountProjects id={id} />
              </TabPanel>
              <TabPanel value={value} index={2}>
                <ListAccountEscalations id={id} />
              </TabPanel>
              <TabPanel value={value} index={3}>
                <ListAccountSolutionDocument driveLocation={data.driveLocation || ""} />
              </TabPanel>
            </Paper>
          </>
        )
      )}
    </>
  );
}

function AccountHeading({ data, showViewOnDriveButton }: { data: AccountDetails; showViewOnDriveButton: boolean }) {
  const name = data.name || "Loading...";
  const number = data.number || " Account ";
  const [openEscalate, setOpenEscalate] = useState(false);
  const permissions = usePermissions();
  const { showError } = useErrorBanner();

  const goToGoogleDrive = () => {
    if (!openExternalUrl(data.driveLocation)) {
      showError("Drive location is not available.");
    }
  };

  return (
    <>
      <EscalateDialog isEscalateDialogOpen={openEscalate} data={data} setIsEscalateDialogOpen={setOpenEscalate} />
      <Stack spacing={0}>
        <Box display="flex" justifyContent="center">
          <Typography variant="h4" gutterBottom fontWeight="bold">
            {name} ( {number} )
          </Typography>
        </Box>
        <Box display="flex" justifyContent="flex-end" marginRight="5%">
          {permissions.canAddEscalations ? (
            <>
              <Button
                variant="contained"
                sx={{ color: "white", mb: -1.25, mr: 1.25 }}
                onClick={() => setOpenEscalate(true)}
                startIcon={<AlertTriangleIcon size={16} />}
              >
                Escalate
              </Button>
              {showViewOnDriveButton && (
                <Button
                  variant="contained"
                  sx={{ color: "white", mb: -1.25, mr: 1.25 }}
                  onClick={goToGoogleDrive}
                  startIcon={<FileTextIcon size={16} />}
                >
                  View on Drive
                </Button>
              )}
            </>
          ) : (
            <>
              <Tooltip title="You don't have the permission">
                <span>
                  <Button variant="contained" disabled startIcon={<AlertTriangleIcon size={16} />}>
                    Escalate
                  </Button>
                </span>
              </Tooltip>
              {showViewOnDriveButton && (
                <Tooltip title="Go to Google Drive">
                  <Button
                    variant="contained"
                    sx={{ color: "white", mb: -1.25, mr: 1.25 }}
                    onClick={goToGoogleDrive}
                    startIcon={<AlertTriangleIcon size={16} />}
                  >
                    View on Drive
                  </Button>
                </Tooltip>
              )}
            </>
          )}
        </Box>
      </Stack>
    </>
  );
}

function BasicInfoTable({ data }: { data: AccountDetails }) {
  const rating = data.rating || "N/A";
  const accountManager = data.accountManager || "N/A";
  const technicalOwner = data.technicalOwner || "N/A";
  const customerSuccessManager = data.customerSuccessManager || "N/A";
  const integrationCSTeamName = (data.integrationCSTeamName as string) || "N/A";
  const integrationCSTeamEmail = (data.integrationCSTeamEmail as string) || "N/A";
  const integrationCSTeamManagerName = (data.integrationCSTeamManagerName as string) || "N/A";
  const integrationCSTeamManagerEmail = (data.integrationCSTeamManagerEmail as string) || "N/A";
  const integrationCSTeamSysId = (data.integrationCSTeamSysId as string) || "";
  const region = data.region || "N/A";
  const country = data.country || "N/A";
  const city = data.city || "N/A";
  const arr = data.arr || "N/A";

  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const { showError } = useErrorBanner();

  const handleTeamScheduleClick = () => {
    if (integrationCSTeamSysId !== "") {
      // /spl/team-schedule, not the bare /team-schedule the source app uses
      // — everything in this port lives under the /spl prefix.
      window.open(`/spl/team-schedule/${integrationCSTeamSysId}`, "_blank");
    } else {
      showError("CRT Team was not found.");
    }
  };

  return (
    <Box sx={{ display: "flex" }}>
      <Box sx={{ flex: 1, mr: "10rem" }}>
        <Table sx={{ width: "100%" }}>
          <TableBody>
            <TableRow>
              <TableCell variant="head" sx={{ border: "none" }}>
                State:
              </TableCell>
              <TableCell sx={{ border: "none" }}>{rating}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell variant="head" sx={{ border: "none" }}>
                Region:
              </TableCell>
              <TableCell sx={{ border: "none" }}>{region}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell variant="head" sx={{ border: "none" }}>
                Country:
              </TableCell>
              <TableCell sx={{ border: "none" }}>{country}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell variant="head" sx={{ border: "none" }}>
                City:
              </TableCell>
              <TableCell sx={{ border: "none" }}>{city}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell variant="head" sx={{ border: "none" }}>
                ARR:
              </TableCell>
              <TableCell sx={{ border: "none" }}>{arr}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell variant="head" sx={{ border: "none" }}>
                Account Manager:
              </TableCell>
              <TableCell sx={{ border: "none" }}>{accountManager.replace(" ⓦ", "")}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell variant="head" sx={{ border: "none" }}>
                Technical Owner:
              </TableCell>
              <TableCell sx={{ border: "none" }}>{technicalOwner.replace(" ⓦ", "")}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell variant="head" sx={{ border: "none" }}>
                Customer Success Manager:
              </TableCell>
              <TableCell sx={{ border: "none" }}>{customerSuccessManager}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell variant="head" sx={{ border: "none" }}>
                CRE Team:
              </TableCell>
              <TableCell sx={{ border: "none" }}>
                {integrationCSTeamName !== "N/A" ? (
                  <>
                    <Button size="small" sx={{ color: "#e96900", ml: -0.9 }} onClick={() => setIsDrawerOpen(true)}>
                      {integrationCSTeamName}
                    </Button>
                    <Box component="span" sx={{ ml: 1 }}>
                      ({integrationCSTeamEmail})
                    </Box>
                  </>
                ) : (
                  "N/A"
                )}
              </TableCell>
            </TableRow>
            {integrationCSTeamName !== "N/A" && (
              <TableRow>
                <TableCell variant="head" sx={{ border: "none" }} />
                <TableCell sx={{ border: "none" }}>
                  <Button size="small" onClick={handleTeamScheduleClick} sx={{ color: "#e96900", ml: -0.6 }}>
                    Team Schedule
                  </Button>
                </TableCell>
              </TableRow>
            )}
            <TableRow>
              <TableCell variant="head" sx={{ border: "none" }}>
                CRE Team Lead:
              </TableCell>
              <TableCell sx={{ border: "none" }}>
                {integrationCSTeamManagerName.replace(" ⓦ", "")}
                <Box component="span" sx={{ ml: 1 }}>
                  ({integrationCSTeamManagerEmail})
                </Box>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
        <TeamMembersDrawer
          open={isDrawerOpen}
          onClose={() => setIsDrawerOpen(false)}
          teamName={(data.integrationCSTeamName as string) || ""}
          teamId={integrationCSTeamSysId}
        />
      </Box>
    </Box>
  );
}
