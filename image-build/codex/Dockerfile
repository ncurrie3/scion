# syntax=docker/dockerfile:1
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

ARG CODEX_CLI_VERSION=latest
ARG SANDBOX_NAME="codex-cli-sandbox"
ENV SANDBOX="$SANDBOX_NAME"

# install codex and clean up
RUN npm install -g @openai/codex@${CODEX_CLI_VERSION} \
  && npm cache clean --force

# default entrypoint when none specified
CMD ["codex"]
