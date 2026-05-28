import { Button, colors, LinearProgress, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import { Pin } from "@wso2/oxygen-ui-icons-react";

export function PromptCreateCase({ onCreateCase, pending = false }: { onCreateCase: () => void; pending?: boolean }) {
  return (
    <Stack sx={{ borderBottom: `1px solid divider`, position: "relative" }}>
      {pending && (
        <LinearProgress color="inherit" sx={{ position: "absolute", top: 0, left: 0, right: 0, height: 2 }} />
      )}

      <Stack direction="row" alignItems="center" gap={2} p={2}>
        <Pin
          size={pxToRem(12)}
          fill={colors.grey[500]}
          style={{ color: colors.grey[500], position: "absolute", right: 3, top: 5 }}
        />

        <Typography variant="body2" sx={{ opacity: pending ? 0.3 : 1 }}>
          I can create a support case with all the details we've discussed.
        </Typography>

        <Button
          variant="outlined"
          sx={{ textTransform: "initial", flexShrink: 0 }}
          onClick={onCreateCase}
          disabled={pending}
        >
          Create Case
        </Button>
      </Stack>
    </Stack>
  );
}
