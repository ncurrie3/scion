# Workstation Server Mode

## Overview

Evolve `scion server` to support a **workstation mode**: a single-user, local-first configuration optimized for developers running Scion on their own machine. This complements the existing multi-user Hub deployment use-case.

The workstation mode collapses the current multi-flag ceremony (`--enable-hub --enable-runtime-broker --enable-web --dev-auth`) into a single intent, adds daemon lifecycle management (start/stop/restart/status) directly on the `scion server` command (mirroring `scion broker`), and introduces a `--foreground` flag for integration with process managers like systemd and launchd.

## Motivation

Today, running Scion locally as a "personal server" requires:

```bash
scion server start --enable-hub --enable-runtime-broker --enable-web --dev-auth --auto-provide
```

This is verbose and leaks infrastructure concerns (Hub, Broker, dev-auth) that a single-user operator shouldn't need to think about. Meanwhile, the `scion broker` command already has polished daemon management (`start`/`stop`/`restart`/`status` with `--foreground`), but `scion server` has only `start` and always runs in the foreground.

**Goals:**
1. Make single-workstation usage the easy, default path.
2. Add daemon lifecycle to `scion server` (parity with `scion broker`).
3. Add `--foreground` for systemd/launchd integration.
4. Disable GCP-dependent features (secrets, storage, Cloud Logging) by default.
5. Keep the existing flag-based composition for production Hub deployments unchanged.

## Design

### 1. New `--workstation` Flag

A meta-flag on `scion server start` that implies:

| Implied Setting | Value | Notes |
|---|---|---|
| `--enable-hub` | `true` | |
| `--enable-runtime-broker` | `true` | |
| `--enable-web` | `true` | |
| `--dev-auth` | `true` | Auto-generates token |
| `--auto-provide` | `true` | |
| `secrets.backend` | `"local"` | SQLite-backed secrets |
| `storage.provider` | `"local"` | Local filesystem storage |
| GCP Cloud Logging | disabled | No `SCION_LOG_GCP` |

Explicit flags still override implied values, so `--workstation --no-web` or `--workstation --storage-bucket gs://...` would work.

**Implementation:** Early in `runServerStart()`, before the existing flag-changed checks, detect `--workstation` and set the default values for all implied flags. The existing `cmd.Flags().Changed()` guards ensure explicit overrides win.

```go
if workstationMode {
    if !cmd.Flags().Changed("enable-hub") {
        enableHub = true
    }
    if !cmd.Flags().Changed("enable-runtime-broker") {
        enableRuntimeBroker = true
        cfg.RuntimeBroker.Enabled = true
    }
    if !cmd.Flags().Changed("enable-web") {
        enableWeb = true
    }
    if !cmd.Flags().Changed("dev-auth") {
        enableDevAuth = true
        cfg.Auth.Enabled = true
    }
    if !cmd.Flags().Changed("auto-provide") {
        serverAutoProvide = true
    }
    // Force local backends unless explicitly overridden
    if !cmd.Flags().Changed("storage-bucket") {
        cfg.Storage.Provider = "local"
    }
    cfg.Secrets.Backend = "local"
}
```

### 2. Daemon Lifecycle for `scion server`

Add `stop`, `restart`, and `status` subcommands to `scion server`, mirroring the existing `scion broker` implementation.

#### Current State

| Command | `scion broker` | `scion server` |
|---|---|---|
| `start` | daemon (default) or `--foreground` | foreground only |
| `stop` | sends SIGTERM via PID file | does not exist |
| `restart` | stop + start | does not exist |
| `status` | daemon + health check | does not exist |

#### Proposed State

| Command | Behavior |
|---|---|
| `scion server start` | Daemon by default; `--foreground` for foreground |
| `scion server start --workstation` | All components enabled, daemon by default |
| `scion server stop` | SIGTERM via PID file |
| `scion server restart` | Stop + start with same args |
| `scion server status` | Daemon status + component health checks |

#### PID/Log File Naming

The `pkg/daemon` package currently hardcodes `broker.pid` / `broker.log`. This needs to be generalized:

**Option A (Recommended):** Add a `component` parameter to daemon functions:

```go
// Before:
const PIDFile = "broker.pid"
const LogFile = "broker.log"

// After:
func PIDFileName(component string) string { return component + ".pid" }
func LogFileName(component string) string { return component + ".log" }
```

The server would use `"server"` as the component, producing `server.pid` / `server.log` in `~/.scion/`. The broker keeps `"broker"` for backward compatibility.

**Option B:** Use a single PID file (`scion.pid`) since only one daemon process should manage components. This is simpler but prevents running a standalone broker alongside a hub server.

**Recommendation:** Option A, to maintain flexibility. The existing broker daemon logic and the new server daemon logic can coexist independently.

#### `scion broker` Delegation Change

Currently `scion broker start` delegates to `scion server start --enable-runtime-broker` (both foreground and daemon modes). This should continue unchanged — the broker command remains a convenient alias for broker-only operation.

The new `scion server start` (daemon mode) would delegate similarly to `scion server start` (foreground) under the hood, just as `scion broker start` does today.

### 3. `--foreground` Flag

Add `--foreground` to `scion server start`. When set:
- Run the server process in the current terminal (current behavior, now opt-in)
- Do not write a PID file
- Stdout/stderr go to the terminal
- Process exits on SIGINT/SIGTERM

When **not** set (new default):
- Fork a detached child process (via `pkg/daemon`)
- Redirect stdout/stderr to `~/.scion/server.log`
- Write PID to `~/.scion/server.pid`
- Parent exits after confirming child started

This matches the `scion broker start` behavior exactly.

**systemd integration example:**
```ini
[Unit]
Description=Scion Workstation Server
After=network.target docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/scion server start --workstation --foreground
ExecStop=/usr/local/bin/scion server stop
Restart=on-failure
User=developer

[Install]
WantedBy=multi-user.target
```

**launchd integration example:**
```xml
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>io.scion.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/scion</string>
        <string>server</string>
        <string>start</string>
        <string>--workstation</string>
        <string>--foreground</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

### 4. GCP Feature Gating

Several features default to GCP services and should be explicitly opt-in rather than silently failing or requiring credentials:

| Feature | Current Default | Workstation Default | Production Flag |
|---|---|---|---|
| Secrets backend | `"local"` | `"local"` | `--secrets-backend=gcpsm` |
| Storage provider | `"local"` | `"local"` | `--storage-bucket gs://...` |
| Cloud Logging | env-driven (`SCION_LOG_GCP`, `K_SERVICE`) | disabled | `SCION_LOG_GCP=true` |
| OAuth (Google/GitHub) | env-driven | disabled (dev-auth) | `SCION_SERVER_OAUTH_*` env vars |
| Telemetry GCP creds | hub-injected | not injected | configure via secrets |

The current defaults are already correct for local use. The `--workstation` flag simply guarantees the local path by:
- Forcing `cfg.Secrets.Backend = "local"`
- Forcing `cfg.Storage.Provider = "local"` (unless `--storage-bucket` is given)
- Not setting `SCION_LOG_GCP`

No code changes are needed to the secret or storage backends themselves — they already support `"local"` mode.

### 5. `scion server status` Command

Report composite status of all components:

```
Scion Server Status
  Mode:          workstation
  Daemon:        running (PID: 12345)
  Log file:      /home/user/.scion/server.log
  PID file:      /home/user/.scion/server.pid

Components:
  Hub API:       running (port 8080, mounted on web)
  Runtime Broker: running (port 9800)
  Web Frontend:  running (port 8080)

Broker:
  ID:            abc-123
  Name:          hostname
  Groves:        2 (global, my-project)
  Auto-provide:  true
```

This would probe the health endpoints (`/healthz`) on the known ports, and check daemon PID status.

## Implementation Plan

### Phase 1: Daemon Lifecycle (cmd/server.go, pkg/daemon)

1. **Generalize `pkg/daemon`**: Parameterize PID/log filenames by component name.
2. **Add `--foreground` flag** to `scion server start` (default: false, daemon mode).
3. **Add `scion server stop`**: Read `server.pid`, send SIGTERM.
4. **Add `scion server restart`**: Stop + start.
5. **Add `scion server status`**: Daemon status + health checks.
6. **Invert default**: `scion server start` runs as daemon unless `--foreground`.

### Phase 2: Workstation Mode (cmd/server.go)

1. **Add `--workstation` flag** with implied defaults.
2. **Force local backends** for secrets and storage in workstation mode.
3. **Update help text and examples** to feature workstation mode prominently.

### Phase 3: Configuration Support (pkg/config)

1. **Support `mode: workstation` in `settings.yaml`** so the flag doesn't need to be passed every time:
   ```yaml
   server:
     mode: workstation
   ```
2. **Persist daemon args** so `scion server restart` can re-launch with the same flags without requiring the user to re-specify them. Store in `~/.scion/server-args.json`.

### Phase 4: Polish

1. **First-run experience**: `scion server start --workstation` prints the dev token and a quickstart URL.
2. **`scion server install`**: (Optional) Generate systemd/launchd service files for the current platform.

## Files to Modify

| File | Changes |
|---|---|
| `cmd/server.go` | Add `--foreground`, `--workstation` flags; add `stop`, `restart`, `status` subcommands; invert daemon default |
| `pkg/daemon/daemon.go` | Parameterize PID/log filenames by component name |
| `cmd/broker.go` | Update daemon calls to use new parameterized API |
| `pkg/config/hub_config.go` | Add `Mode` field to `GlobalConfig` for `settings.yaml` support |

## Backward Compatibility

- `scion server start --enable-hub --enable-runtime-broker` continues to work unchanged (foreground behavior preserved when `--foreground` is passed).
- **Breaking change**: `scion server start` without `--foreground` will now daemonize instead of running in foreground. This is acceptable because:
  - The current foreground-only behavior has no daemon management (no stop/restart).
  - Production deployments use process managers (systemd, Cloud Run) that would pass `--foreground`.
  - The `scion broker start` command already established daemon-by-default as the convention.
- `scion broker start/stop/restart/status` continue to work unchanged. They manage a separate `broker.pid` and only start the runtime broker component.

## Open Questions

1. **Should `scion server` and `scion broker` share a PID file?** Currently, `broker start` delegates to `server start --enable-runtime-broker`. If the server also daemonizes, running both `scion server start` and `scion broker start` could conflict. Options:
   - **Separate PIDs**: Server uses `server.pid`, broker uses `broker.pid`. They could co-run on different ports.
   - **Shared PID with conflict detection**: Detect if a server daemon is already running the broker component and refuse to start a standalone broker.
   - **Recommendation**: Separate PIDs with port-conflict detection (already exists in `checkPort()`).

2. **Should workstation mode bind to `127.0.0.1` instead of `0.0.0.0`?** For single-user security, binding to localhost-only makes sense. Production deployments explicitly set `--host`.

3. **Should `--workstation` be the default when no `--enable-*` flags are given?** Instead of erroring with "no server components enabled", we could default to workstation mode. This would make `scion server start` "just work" but might surprise users expecting the current behavior.
