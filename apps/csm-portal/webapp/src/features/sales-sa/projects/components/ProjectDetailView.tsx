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

// Report links (SLA/CS/Timelogs) point at /spl/projects/:number/... — the
// actual report pages are a different domain's scope (reports/schedule/
// usage-metrics/user-scan fork) in this merge; these links assume that
// fork's routes land at the same paths the source app used, just under /spl.
import { useState, type ReactNode } from "react";
import { Alert, Box, Button, Paper, Stack, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { FileTextIcon } from "@wso2/oxygen-ui-icons-react";
import { useGetSplProject } from "../api/useProjectsApi";
import LinearLoading from "./LinearLoading";
import ProjectPathView from "./ProjectPathView";
import ProjectContactsTable from "./ProjectContactsTable";
import ProjectCasesTable from "./ProjectCasesTable";
import type { ProjectDetails } from "../projectTypes";

const PROJECT_TYPE_CLOUD_SUPPORT = "Cloud Support";
const PROJECT_TYPE_CLOUD_EVALUATION_SUPPORT = "Cloud Evaluation Support";

export default function ProjectDetailView({ id }: { id: string }) {
  const [tabValue, setTabValue] = useState(0);
  const { data, isLoading, error } = useGetSplProject(id);

  const isTypeCloud =
    data?.projectType === PROJECT_TYPE_CLOUD_SUPPORT || data?.projectType === PROJECT_TYPE_CLOUD_EVALUATION_SUPPORT;

  if (isLoading) return <LinearLoading />;
  if (error) return <Alert severity="error">{error.message}</Alert>;
  if (!data) return null;

  return (
    <>
      <ProjectPathView accountName={data.accountName} accountNumber={data.accountId} projectKey={data.key} />
      <ProjectHeading data={data} />
      <Tabs value={tabValue} onChange={(_e, v) => setTabValue(v)} aria-label="Project Tabs">
        <Tab label="Basic Info" />
        <Tab label="Contacts" />
        <Tab label="Cases" />
      </Tabs>
      <Paper variant="outlined">
        <TabPanel value={tabValue} index={0}>
          <BasicInfoTable data={data} />
        </TabPanel>
        <TabPanel value={tabValue} index={1}>
          <ProjectContactsTable id={id} />
        </TabPanel>
        <TabPanel value={tabValue} index={2}>
          <ProjectCasesTable id={id} isTypeCloud={Boolean(isTypeCloud)} />
        </TabPanel>
      </Paper>
    </>
  );
}

function TabPanel({ children, value, index }: { children: ReactNode; value: number; index: number }) {
  return (
    <div role="tabpanel" hidden={value !== index}>
      {value === index && <Box sx={{ p: 3 }}>{children}</Box>}
    </div>
  );
}

function ProjectHeading({ data }: { data: ProjectDetails }) {
  const name = data.name || "Loading...";
  const key = data.key || " Project";

  const openReport = (path: string) => {
    if (!data.sysId) return;
    window.open(`/spl/projects/${data.number}/${path}/${data.sysId}`);
  };

  const openTimelogs = () => {
    window.open(`/spl/projects/${data.number}/timelogs-report`);
  };

  return (
    <Stack spacing={0} sx={{ mb: 2 }}>
      <Box display="flex" justifyContent="center">
        <Typography variant="h4" gutterBottom fontWeight="bold">
          {name} ( {key} )
        </Typography>
      </Box>
      <Box display="flex" justifyContent="flex-end" gap={1.5}>
        <Button
          variant="contained"
          onClick={() => openReport("sla-report")}
          startIcon={<FileTextIcon size={16} />}
          disabled={!data.sysId}
        >
          SLA Report
        </Button>
        <Button variant="contained" onClick={openTimelogs} startIcon={<FileTextIcon size={16} />}>
          Time Report
        </Button>
        <Button
          variant="contained"
          onClick={() => openReport("cs-report")}
          startIcon={<FileTextIcon size={16} />}
          disabled={!data.sysId}
        >
          CS Report
        </Button>
      </Box>
    </Stack>
  );
}

function BasicInfoTable({ data }: { data: ProjectDetails }) {
  const rows: [string, string][] = [
    ["State", data.closureState || "N/A"],
    ["Start Date", data.startDate || "N/A"],
    ["End Date", data.endDate || "N/A"],
  ];
  const hoursRows: [string, string][] = [
    ["Total Query Hours", data.totalQueryHours || "N/A"],
    ["Remaining Query Hours", data.remainingQueryHours || "N/A"],
  ];

  return (
    <Box sx={{ display: "flex", gap: 8 }}>
      <InfoTable rows={rows} />
      <InfoTable rows={hoursRows} />
    </Box>
  );
}

function InfoTable({ rows }: { rows: [string, string][] }) {
  return (
    <Box>
      {rows.map(([label, value]) => (
        <Box key={label} sx={{ display: "flex", gap: 2, py: 0.5 }}>
          <Typography sx={{ fontWeight: 600, minWidth: 160 }}>{label}:</Typography>
          <Typography>{value}</Typography>
        </Box>
      ))}
    </Box>
  );
}
