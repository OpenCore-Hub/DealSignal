# syntax=docker/dockerfile:1
# Multi-stage build for the DealSignal React frontend.
# The production image serves the static Vite build with Nginx and proxies
# /api requests to the backend service.

FROM node:22-alpine AS builder

WORKDIR /app

# Activate Corepack and pin pnpm to the version declared in package.json.
RUN corepack enable && corepack prepare pnpm@11.9.0 --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .
RUN pnpm build

FROM nginx:1.27-alpine

# Install envsubst so the Nginx config can be templated at runtime.
RUN apk add --no-cache gettext

COPY nginx.conf /etc/nginx/conf.d/default.conf.template
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

ENTRYPOINT ["/docker-entrypoint.sh"]
