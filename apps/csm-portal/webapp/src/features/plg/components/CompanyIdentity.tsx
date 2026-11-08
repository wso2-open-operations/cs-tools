/**
 * The reveal: the company id and starred email until the eye is clicked. The
 * three pieces share one useCompanyReveal entry per organisation.
 */
import { CircularProgress, IconButton, Tooltip, Typography } from "@wso2/oxygen-ui";
import { Eye, EyeOff } from "@wso2/oxygen-ui-icons-react";
import type { MouseEvent } from "react";

import { useCompanyReveal } from "@features/plg/api/hooks";
import type { CompanyIdentity } from "@features/plg/api/types";

/** What stands in for a hidden email. */
export const MASKED_EMAIL = "••••••••";

/** A revealed identity that actually carries values to show. */
function shown(identity: CompanyIdentity | undefined): identity is CompanyIdentity {
  return !!identity && (identity.source === "DATABASE" || identity.source === "CLOUD");
}

/** The company id, or the organisation's name once revealed. */
export function CompanyName({
  organizationId,
  companyId,
  variant = "body2",
  fontWeight,
}: {
  organizationId: string;
  companyId: string | null;
  variant?: "body2" | "h4";
  fontWeight?: number;
}) {
  const { identity } = useCompanyReveal(organizationId);
  if (shown(identity) && identity.organizationName) {
    return (
      <Typography variant={variant} fontWeight={fontWeight} component="span">
        {identity.organizationName}
      </Typography>
    );
  }
  if (!companyId) {
    return (
      <Typography variant={variant} color="text.secondary" component="span">
        No company id
      </Typography>
    );
  }
  return (
    <Typography
      variant={variant}
      fontWeight={fontWeight}
      component="span"
      sx={{ fontFamily: "monospace" }}
      aria-label={`Company id ${companyId}`}
    >
      {companyId}
    </Typography>
  );
}

/** Stars, or the email once revealed. */
export function MaskedEmail({ organizationId }: { organizationId: string }) {
  const { identity } = useCompanyReveal(organizationId);
  if (shown(identity)) {
    return (
      <Typography variant="body2" component="span" color={identity.email ? undefined : "text.secondary"}>
        {identity.email ?? "Not available"}
      </Typography>
    );
  }
  return (
    <Typography variant="body2" component="span" color="text.secondary" aria-label="Email hidden">
      {MASKED_EMAIL}
    </Typography>
  );
}

function revealTooltip(identity: CompanyIdentity | undefined, notFound: boolean, failed: boolean): string {
  if (notFound) return "This organisation no longer exists";
  if (failed || identity?.source === "FAILED") return "Couldn't reach the cloud just now. Click to try again";
  if (identity?.source === "UNRESOLVED") return "No name or email is known for this organisation";
  if (identity?.source === "CLOUD") return "Shown from the cloud. Click to hide";
  if (identity?.source === "DATABASE") return "Click to hide";
  return "Reveal name and email";
}

/** The eye: reveals the row's name and email, or hides them again. */
export function RevealButton({ organizationId }: { organizationId: string }) {
  const { identity, notFound, isFetching, error, reveal, hide } = useCompanyReveal(organizationId);
  const isShown = shown(identity);
  const label = revealTooltip(identity, notFound, !!error);

  const onClick = (e: MouseEvent) => {
    // Rows navigate on click; the eye must not open the organisation.
    e.stopPropagation();
    if (isShown) hide();
    else reveal();
  };

  return (
    <Tooltip title={label}>
      <span>
        <IconButton
          size="small"
          onClick={onClick}
          disabled={isFetching || notFound}
          aria-label={isShown ? "Hide name and email" : "Reveal name and email"}
          aria-pressed={isShown}
        >
          {isFetching ? <CircularProgress size={16} /> : isShown ? <EyeOff size={16} /> : <Eye size={16} />}
        </IconButton>
      </span>
    </Tooltip>
  );
}
