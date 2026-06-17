#!/usr/bin/env bash
set -euo pipefail

CONTAINER_NAME="dealsignal-postgres"
IMAGE="postgres:16-alpine"

if docker ps -q --filter "name=^${CONTAINER_NAME}$" | grep -q .; then
  echo "PostgreSQL container '${CONTAINER_NAME}' is already running."
  exit 0
fi

if docker ps -aq --filter "name=^${CONTAINER_NAME}$" | grep -q .; then
  echo "Starting existing PostgreSQL container '${CONTAINER_NAME}'..."
  docker start "${CONTAINER_NAME}"
else
  echo "Creating PostgreSQL container '${CONTAINER_NAME}'..."
  docker run -d \
    --name "${CONTAINER_NAME}" \
    -e POSTGRES_USER=dealsignal \
    -e POSTGRES_PASSWORD=dealsignal \
    -e POSTGRES_DB=dealsignal \
    -p 5432:5432 \
    -v dealsignal-postgres-data:/var/lib/postgresql/data \
    "${IMAGE}"
fi
