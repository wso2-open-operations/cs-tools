import { Button, Card, colors, Grid, pxToRem, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { ArrowRight, CircleAlert, MessageSquare } from "@wso2/oxygen-ui-icons-react";

import { useProject } from "@context/project";

import type { Project } from "@features/projects/types/project.model";

export function ProjectItem(props: Project) {
  const { setProjectId } = useProject();

  return (
    <Card sx={{ bgcolor: "background.paper" }}>
      <Stack p={2} gap={1}>
        <Typography variant="subtitle2">{props.projectKey}</Typography>
        <Typography variant="h6" mt={-0.8}>
          {props.name}
        </Typography>
      </Stack>

      <Grid spacing={1.5} sx={{ p: 2, bgcolor: "background.default" }} container>
        <Grid size={{ xs: 6 }} sx={{ p: 2, direction: "row", alignItems: "center" }} spacing={1} container>
          <CircleAlert size={18} />
          <Typography variant="body2">Outstanding:</Typography>
          <Typography variant="body2" color={colors.red[300]}>
            {props.metrics.outstanding ?? <Skeleton width={30} height={20} />}
          </Typography>
        </Grid>

        <Grid size={{ xs: 6 }} sx={{ p: 2, direction: "row", alignItems: "center" }} spacing={1} container>
          <MessageSquare size={18} />
          <Typography variant="body2">Chats:</Typography>
          <Typography variant="body2" color={colors.indigo[300]}>
            {props.metrics.chats ?? <Skeleton width={30} height={20} />}
          </Typography>
        </Grid>
      </Grid>

      <Button
        variant="contained"
        endIcon={<ArrowRight size={pxToRem(18)} />}
        sx={{ textTransform: "initial", width: "100%", m: 2, mt: 3 }}
        onClick={() => setProjectId(props.id)}
      >
        View Dashboard
      </Button>
    </Card>
  );
}

export function ProjectItemSkeleton() {
  return (
    <Card sx={{ bgcolor: "background.paper" }}>
      <Stack p={2} gap={1}>
        <Skeleton variant="text" width="30%" sx={{ fontSize: "subtitle2.fontSize" }} />
        <Skeleton variant="text" width="100%" height={32} sx={{ mt: -0.8 }} />
      </Stack>

      <Grid p={2} spacing={1.5} sx={{ bgcolor: "background.default" }} container>
        {Array.from({ length: 2 }).map((_, i) => (
          <Grid key={i} size={{ xs: 6 }}>
            <Stack gap={0.5}>
              <Skeleton variant="text" width="40%" height={14} />
              <Skeleton variant="text" width="60%" height={24} />
            </Stack>
          </Grid>
        ))}
      </Grid>

      <Stack p={2} pt={3}>
        <Skeleton variant="rounded" width="100%" height={36} />
      </Stack>
    </Card>
  );
}
