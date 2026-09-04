import type { NextConfig } from "next";

// Standalone output for Choreo's Dockerfile/BYOC deployment path
const nextConfig: NextConfig = {
  output: "standalone",
  // `loadConfig()` reads config/sla-config.yaml via fs at request time, not
  // via import — Next's file tracer only follows imports, so the yaml file
  // must be listed explicitly or it's dropped from the standalone bundle.
  outputFileTracingIncludes: {
    "/**": [
      "./config/sla-config.yaml",
      "./node_modules/.prisma/client/**",
    ],
  },
};

export default nextConfig;
