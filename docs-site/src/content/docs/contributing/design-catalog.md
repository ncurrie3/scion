---
title: Design Catalog
description: Historical and future design specifications for the Scion project.
---

The Scion project maintains detailed design documents in the `.design/` directory of the repository. These documents capture the architectural decisions, feature specifications, and implementation plans that have shaped the platform.

This catalog provides a categorized map of the key design documents for contributors and maintainers.

## Core System Design

- **[Scion Overview](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/scion.md)**: The original high-level design and value proposition.
- **[Taskless Refactor](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/taskless-refactor.md)**: Design for moving towards a more flexible, long-running agent model.
- **[Versioned Settings Refactor](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/versioned-settings-refactor.md)**: The migration to a schema-versioned `settings.yaml` and `scion-agent.yaml` system.
- **[Agnostic Template Design](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/agnostic-template-design.md)**: Decoupling role definitions from specific LLM tool mechanics.

## Hosted & Distributed Architecture

- **[Hosted Design](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/hosted/README.md)**: Overview of the Hub and Runtime Broker architecture.
- **[Web Frontend Design](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/hosted/web-frontend-design.md)**: Architecture and technology stack for the Scion Dashboard.
- **[Dev Auth Design](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/hosted/dev-auth.md)**: Specification for the zero-config development authentication mode.
- **[Auth Dialog Design](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/hosted/auth-dialog.md)**: UX and implementation of the CLI-to-Hub authentication flow.

## Runtimes & Infrastructure

- **[Podman Runtime](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/podman-runtime.md)**: Design for daemonless/rootless agent execution.
- **[Apple Virtualization Framework](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/apple-container.md)**: Native macOS container execution.
- **[Kubernetes Design](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/kubernetes/README.md)**: Specification for running agents as Kubernetes Pods and managing remote workspace sync.

## Feature Specifications

- **[Message Command](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/message-cmd.md)**: Enqueuing input into running agent sessions.
- **[Tmux Integration](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/tmux-footer.md)**: Persistent session management and status visualization.
- **[Agent Status Reporting](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/sciontool-overview.md)**: Detailed design of the `sciontool` utility and agent lifecycle signaling.
- **[GCS Volume Support](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/initial-gcs-volume-support.md)**: Design for remote storage persistence and template distribution.

## Walkthroughs & Guides

- **[Hosted Mode Walkthrough](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/walkthroughs/hosted-mode.md)**: Step-by-step setup of the distributed architecture.
- **[K8s Local Development](https://github.com/GoogleCloudPlatform/scion/blob/main/.design/walkthroughs/k8s-local.md)**: Setting up a local development cluster with KinD/Minikube.

---

*Note: For the most up-to-date specifications, always refer to the source `.md` files in the `.design/` directory of the repository.*
