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
import { Button, Card, CardActions, colors, Grid, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { ArrowRight, CircleAlert, MessageSquare } from "@wso2/oxygen-ui-icons-react";

import { useProject } from "@context/project";

import type { Project } from "@features/projects/types/project.model";

import { useNavigation } from "@shared/hooks";

export function ProjectItem(props: Project) {
  const { toHome } = useNavigation();
  const { setProjectId } = useProject();

  return (
    <Card sx={{ bgcolor: "background.paper", mb: 1.5 }}>
      <Stack sx={{ p: 2, gap: 1 }}>
        <Typography variant="subtitle2" sx={{ opacity: 0.8 }}>
          {props.projectKey}
        </Typography>
        <Typography variant="h6" mt={-0.8}>
          {props.name}
        </Typography>
      </Stack>

      <Grid sx={{ p: 2, spacing: 1, bgcolor: "background.default" }} container>
        <Grid size={{ xs: 6 }} sx={{ display: "flex", direction: "row", alignItems: "center", gap: 1 }}>
          <CircleAlert size={18} />
          <Typography variant="body2">Outstanding:</Typography>
          <Typography variant="body2" color={colors.red[300]}>
            {props.metrics.outstanding ?? <Skeleton width={30} height={20} />}
          </Typography>
        </Grid>

        <Grid size={{ xs: 6 }} sx={{ display: "flex", direction: "row", alignItems: "center", gap: 1 }}>
          <MessageSquare size={18} />
          <Typography variant="body2">Chats:</Typography>
          <Typography variant="body2" color={colors.indigo[300]}>
            {props.metrics.chats ?? <Skeleton width={30} height={20} />}
          </Typography>
        </Grid>
      </Grid>

      <CardActions sx={{ px: 2, py: 3 }}>
        <Button
          fullWidth
          variant="contained"
          endIcon={<ArrowRight size={18} />}
          sx={{ textTransform: "initial" }}
          onClick={() => {
            setProjectId(props.id);
            toHome({ replace: true });
          }}
        >
          View Dashboard
        </Button>
      </CardActions>
    </Card>
  );
}

export function ProjectItemSkeleton() {
  return (
    <Card sx={{ bgcolor: "background.paper", mb: 1.5 }}>
      <Stack p={2} gap={1}>
        <Skeleton variant="text" width="30%" sx={{ fontSize: "subtitle2.fontSize" }} />
        <Skeleton variant="text" width="100%" height={32} sx={{ mt: -0.8 }} />
      </Stack>

      <Grid p={2} spacing={1.5} sx={{ bgcolor: "background.default" }} container>
        {Array.from({ length: 2 }).map((_, i) => (
          <Grid key={i} size={{ xs: 6 }}>
            <Skeleton variant="text" width="90%" height={24} />
          </Grid>
        ))}
      </Grid>

      <Stack p={2} pt={3}>
        <Skeleton variant="rounded" width="100%" height={36} />
      </Stack>
    </Card>
  );
}
