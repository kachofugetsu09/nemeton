FROM node:22.17.1-bookworm AS node

FROM golang:1.25.0-trixie

COPY --from=node /usr/local/bin/ /usr/local/bin/
COPY --from=node /usr/local/lib/node_modules/ /usr/local/lib/node_modules/

RUN apt-get update \
    && apt-get install -y --no-install-recommends git \
    && corepack enable pnpm \
    && corepack prepare pnpm@11.3.0 --activate \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY webui/package.json webui/pnpm-lock.yaml ./webui/
RUN pnpm --dir webui install --frozen-lockfile \
    && pnpm --dir webui exec playwright install --with-deps chromium

COPY . .
RUN scripts/container-certification

CMD ["scripts/container-certification"]
