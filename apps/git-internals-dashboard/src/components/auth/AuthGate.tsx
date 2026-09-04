"use client";

// src/components/auth/AuthGate.tsx
import { useEffect, type ReactNode } from "react";
import { useAuthContext } from "@asgardeo/auth-react";
import { useQuery } from "@tanstack/react-query";
import { Box, Button, CircularProgress, Paper, Typography } from "@mui/material";
import { setAccessTokenGetter } from "@/lib/auth-token";
import { acrylicSurfaceSx } from "@/lib/surfaces";

function CenteredSpinner() {
  // No bgcolor — let Oxygen UI's Acrylic gradient body backdrop show through.
  return (
    <Box sx={{ display: "flex", minHeight: "100vh", alignItems: "center", justifyContent: "center" }}>
      <CircularProgress size={32} sx={{ color: "var(--sla-primary)" }} />
    </Box>
  );
}

function CenteredCard({ children }: { children: ReactNode }) {
  return (
    <Box sx={{ display: "flex", minHeight: "100vh", alignItems: "center", justifyContent: "center", px: 3 }}>
      <Paper
        variant="outlined"
        sx={{ ...acrylicSurfaceSx, width: "100%", maxWidth: 384, borderRadius: "9px", borderColor: "var(--sla-border)", p: 4, textAlign: "center" }}
      >
        {children}
      </Paper>
    </Box>
  );
}

function SignInPage() {
  const { signIn } = useAuthContext();
  return (
    <CenteredCard>
      <Box
        sx={{
          mx: "auto", mb: 2, display: "flex", height: 34, width: 34, alignItems: "center", justifyContent: "center",
          borderRadius: "9px", background: "var(--sla-primary-gradient)", color: "var(--sla-contrast-text)", fontSize: 15, fontWeight: 700,
        }}
      >
        S
      </Box>
      <Typography sx={{ mb: 0.5, fontSize: 16, fontWeight: 600, letterSpacing: "-0.01em", color: "var(--sla-fg)" }}>
        SLA Monitor
      </Typography>
      <Typography sx={{ mb: 3, fontSize: 12, color: "var(--sla-fg3)" }}>Sign in to continue</Typography>
      <Button
        fullWidth
        onClick={() => signIn()}
        sx={{ borderRadius: "9px", background: "var(--sla-primary-gradient)", color: "var(--sla-contrast-text)", py: 1, fontSize: 13, fontWeight: 500, "&:hover": { background: "var(--sla-primary-gradient)", opacity: 0.9 } }}
      >
        Sign in
      </Button>
    </CenteredCard>
  );
}

function UnauthorizedPage() {
  const { signOut } = useAuthContext();
  return (
    <CenteredCard>
      <Typography sx={{ mb: 1, fontSize: 16, fontWeight: 600, letterSpacing: "-0.01em", color: "var(--sla-fg)" }}>
        Not authorized
      </Typography>
      <Typography sx={{ mb: 3, fontSize: 13, color: "var(--sla-fg2)" }}>
        You are not authorized to access this application.
      </Typography>
      <Button
        fullWidth
        variant="outlined"
        onClick={() => signOut()}
        sx={{ borderRadius: "9px", borderColor: "var(--sla-border)", bgcolor: "var(--sla-card)", color: "var(--sla-fg)", py: 1, fontSize: 13, fontWeight: 500 }}
      >
        Sign out
      </Button>
    </CenteredCard>
  );
}

function ErrorState() {
  return (
    <CenteredCard>
      <Typography sx={{ mb: 1, fontSize: 16, fontWeight: 600, letterSpacing: "-0.01em", color: "var(--sla-fg)" }}>
        Something went wrong
      </Typography>
      <Typography sx={{ fontSize: 13, color: "var(--sla-fg2)" }}>
        Checking your session failed. Try reloading the page.
      </Typography>
    </CenteredCard>
  );
}

export function AuthGate({ children }: { children: ReactNode }) {
  const { state, getAccessToken } = useAuthContext();

  useEffect(() => {
    if (state.isAuthenticated) {
      setAccessTokenGetter(() => getAccessToken());
    }
    return () => setAccessTokenGetter(null);
  }, [state.isAuthenticated, getAccessToken]);

  const meQuery = useQuery({
    queryKey: ["auth-me"],
    queryFn: async (): Promise<{ authorized: boolean }> => {
      const token = await getAccessToken();
      const res = await fetch("/api/auth/me", {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (res.status === 403) return { authorized: false };
      if (!res.ok) throw new Error(`API ${res.status} on /auth/me`);
      return res.json() as Promise<{ authorized: boolean }>;
    },
    enabled: state.isAuthenticated,
    retry: false,
  });

  if (state.isLoading) return <CenteredSpinner />;
  if (!state.isAuthenticated) return <SignInPage />;
  if (meQuery.isLoading) return <CenteredSpinner />;
  if (meQuery.isError) return <ErrorState />;
  if (meQuery.data?.authorized === false) return <UnauthorizedPage />;

  return <>{children}</>;
}
