# Base image note: node:22-slim (Debian/glibc) is deliberate. Switching to
# alpine (musl) breaks the default Prisma query engine unless you also add
# binaryTargets = ["native", "linux-musl-openssl-3.0.x"] to schema.prisma.
FROM node:22-slim AS base

# ---- deps ---------------------------------------------------------------
FROM base AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

# ---- build --------------------------------------------------------------
FROM base AS builder
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1

COPY --from=deps /app/node_modules ./node_modules
COPY . .

# Next.js inlines NEXT_PUBLIC_* env vars at build time, pass them as Docker build args 
# Choreo: Build Configurations editor.
ARG NEXT_PUBLIC_AUTH_MODE=stub
ARG NEXT_PUBLIC_ASGARDEO_BASE_URL=
ARG NEXT_PUBLIC_ASGARDEO_CLIENT_ID=
ARG NEXT_PUBLIC_ASGARDEO_SCOPES="openid groups"
ENV NEXT_PUBLIC_AUTH_MODE=$NEXT_PUBLIC_AUTH_MODE \
    NEXT_PUBLIC_ASGARDEO_BASE_URL=$NEXT_PUBLIC_ASGARDEO_BASE_URL \
    NEXT_PUBLIC_ASGARDEO_CLIENT_ID=$NEXT_PUBLIC_ASGARDEO_CLIENT_ID \
    NEXT_PUBLIC_ASGARDEO_SCOPES=$NEXT_PUBLIC_ASGARDEO_SCOPES

RUN npx prisma generate
RUN npm run build

# ---- runtime ------------------------------------------------------------
FROM base AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN groupadd --system --gid 10001 nodejs \
    && useradd --system --uid 10001 --gid 10001 --create-home nextjs

# Standalone output (next.config.ts: output: 'standalone') bundles only the
# production deps actually needed at runtime.
COPY --from=builder --chown=10001:10001 /app/.next/standalone ./
COPY --from=builder --chown=10001:10001 /app/.next/static ./.next/static
COPY --from=builder --chown=10001:10001 /app/prisma ./prisma

# Next writes its fetch/ISR cache here at runtime.
RUN mkdir -p .next/cache && chown -R 10001:10001 .next

USER 10001

EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
