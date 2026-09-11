# syntax=docker/dockerfile:1

FROM node:24-alpine AS build

WORKDIR /workspace
ENV NEXT_TELEMETRY_DISABLED=1

# Install the exact pnpm version declared by the workspace.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN npm install --global "$(node -p 'require("./package.json").packageManager')"
COPY apps/web/package.json ./apps/web/package.json
RUN pnpm install --frozen-lockfile

COPY apps/web ./apps/web

RUN mkdir -p apps/web/public \
    && pnpm --filter @keepup/web build

FROM node:24-alpine AS runtime

WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    HOSTNAME=0.0.0.0 \
    PORT=3000

COPY --from=build --chown=node:node /workspace/apps/web/.next/standalone ./
COPY --from=build --chown=node:node /workspace/apps/web/.next/static ./apps/web/.next/static
COPY --from=build --chown=node:node /workspace/apps/web/public ./apps/web/public

USER node
EXPOSE 3000
CMD ["node", "apps/web/server.js"]
