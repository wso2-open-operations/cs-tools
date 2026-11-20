// Temporary local demo, not committed.
import { createRoot } from "react-dom/client";
import { useState, type JSX } from "react";
import { Box, Card, Chip, Typography } from "@wso2/oxygen-ui";
import { ThemePreferenceProvider } from "@context/theme/ThemePreferenceContext";
import IncidentActionBar from "@features/csm-operations/components/IncidentActionBar";
import CancelIncidentDialog from "@features/csm-operations/components/CancelIncidentDialog";
import { incidentStateColor, incidentStateLabel } from "@features/csm-operations/utils/incidents";
import type { BeIncidentDetail, BeIncidentState } from "@api/backend/types";

function IncidentHeader({ number, initial }: { number: string; initial: BeIncidentState }): JSX.Element {
  const [state, setState] = useState<BeIncidentState>(initial);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const incident: BeIncidentDetail = {
    id: number, number, openedOn: "2026-11-19T08:00:00Z", subject: "Gateway returning 502s", priority: null, state, category: null,
  };
  // Same dispatch as CsmIncidentDetailPage.onIncidentAction (minus the PATCH).
  const onAction = (target: BeIncidentState): void => {
    if (target === "CANCELLED") { setConfirmOpen(true); return; }
    setState(target);
  };
  return (
    <Card variant="outlined" sx={{ p: 3, display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 2 }}>
      <Box>
        <Typography variant="overline" color="text.secondary">{number}</Typography>
        <Typography variant="h5" sx={{ mb: 1 }}>Gateway returning 502s</Typography>
        <Chip size="small" variant="outlined" color={incidentStateColor(state)} label={incidentStateLabel(state)} />
      </Box>
      <IncidentActionBar incident={incident} isPending={pending} onAction={onAction} />
      {confirmOpen && (
        <CancelIncidentDialog
          incidentNumber={number}
          isSubmitting={pending}
          onClose={() => !pending && setConfirmOpen(false)}
          onConfirm={() => { setPending(true); setTimeout(() => { setState("CANCELLED"); setPending(false); setConfirmOpen(false); }, 800); }}
        />
      )}
    </Card>
  );
}

function Demo(): JSX.Element {
  return (
    <ThemePreferenceProvider>
      <Box sx={{ minHeight: "100vh", bgcolor: "background.default", p: 4, display: "flex", flexDirection: "column", gap: 3, maxWidth: 1100, mx: "auto" }}>
        <Typography variant="h4">Incident detail: Change state demo</Typography>
        <IncidentHeader number="INC0010001" initial="NEW" />
        <IncidentHeader number="INC0010002" initial="IN_PROGRESS" />
        <IncidentHeader number="INC0010003" initial="ON_HOLD" />
      </Box>
    </ThemePreferenceProvider>
  );
}

createRoot(document.getElementById("root")!).render(<Demo />);
