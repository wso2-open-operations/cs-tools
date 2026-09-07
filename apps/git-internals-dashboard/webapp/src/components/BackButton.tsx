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

// Port of v3's src/components/BackButton.tsx (Next's useRouter/useSearchParams
// -> react-router's useNavigate/useSearchParams).
import { Button } from "@mui/material";
import { useNavigate, useSearchParams } from "react-router";

// Only the global dashboard filters survive the trip back; list-scoped params
// (bucket, status, q) are dropped so they can't leak into later drills.
export function BackButton() {
  const navigate = useNavigate();
  const [params] = useSearchParams();

  const next = new URLSearchParams();
  for (const key of ["repo", "priority"] as const) {
    const v = params.get(key);
    if (v) next.set(key, v);
  }
  const search = next.toString();

  return (
    <Button
      onClick={() => navigate(search ? `/?${search}` : "/")}
      sx={{
        display: "inline-flex",
        alignItems: "center",
        gap: 0.75,
        borderRadius: "9px",
        border: "1px solid var(--sla-border)",
        bgcolor: "var(--sla-card)",
        px: 1.5,
        py: 1,
        fontSize: 13,
        fontWeight: 500,
        color: "var(--sla-fg2)",
        textTransform: "none",
        "&:hover": { borderColor: "var(--sla-fg3)", color: "var(--sla-fg)", bgcolor: "var(--sla-card)" },
      }}
    >
      ← Back to dashboard
    </Button>
  );
}
