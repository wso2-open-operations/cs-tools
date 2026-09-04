"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Button } from "@mui/material";

// Only the global dashboard filters survive the trip back; list-scoped params
// (bucket, status, q) are dropped so they can't leak into later drills.
export function BackButton() {
  const router = useRouter();
  const params = useSearchParams() ?? new URLSearchParams();

  const next = new URLSearchParams();
  for (const key of ["repo", "priority"] as const) {
    const v = params.get(key);
    if (v) next.set(key, v);
  }
  const search = next.toString();

  return (
    <Button
      onClick={() => router.push(search ? `/?${search}` : "/")}
      sx={{
        display: "inline-flex", alignItems: "center", gap: 0.75, borderRadius: "9px",
        border: "1px solid var(--sla-border)", bgcolor: "var(--sla-card)", px: 1.5, py: 1,
        fontSize: 13, fontWeight: 500, color: "var(--sla-fg2)", textTransform: "none",
        "&:hover": { borderColor: "var(--sla-fg3)", color: "var(--sla-fg)", bgcolor: "var(--sla-card)" },
      }}
    >
      ← Back to dashboard
    </Button>
  );
}
