"use client";

// src/app/AsgardeoAuthWrapper.tsx
// Isolated into its own module, loaded only via next/dynamic({ ssr: false })
// from providers.tsx: @asgardeo/auth-react references browser globals (e.g.
// `origin`) at module top-level, which throws during Next's SSR pass of
// client components. Keeping the import out of any server-rendered path also
// means stub-mode deployments never fetch or evaluate this vendor bundle.
import type { ReactNode } from "react";
import { AuthProvider } from "@asgardeo/auth-react";
import { AuthGate } from "@/components/auth/AuthGate";

export default function AsgardeoAuthWrapper({ children }: { children: ReactNode }) {
  return (
    <AuthProvider
      config={{
        signInRedirectURL: window.location.origin,
        signOutRedirectURL: window.location.origin,
        clientID: process.env.NEXT_PUBLIC_ASGARDEO_CLIENT_ID ?? "",
        baseUrl: process.env.NEXT_PUBLIC_ASGARDEO_BASE_URL ?? "",
        scope: (process.env.NEXT_PUBLIC_ASGARDEO_SCOPES ?? "openid groups").split(" "),
      }}
    >
      <AuthGate>{children}</AuthGate>
    </AuthProvider>
  );
}
