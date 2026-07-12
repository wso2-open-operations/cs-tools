FROM node:20-alpine

WORKDIR /app

# Copy package files first for better caching
COPY package.json package-lock.json* ./

RUN npm ci --omit=dev

# Copy application source
COPY . .

# Create a non-root user (UID in Choreo's required 10000-20000 range)
RUN addgroup -g 10001 -S appgroup && adduser -u 10001 -S appuser -G appgroup \
    && chown -R appuser:appgroup /app

USER 10001

EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:3000/api/v1/health || exit 1

CMD ["node", "src/index.js"]
