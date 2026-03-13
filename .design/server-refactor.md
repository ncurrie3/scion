# Server Command Refactoring

## Status: Proposal

## Problem Statement

The `cmd/server.go` file has grown to ~2300 lines and handles too many distinct responsibilities in a single file. While the server's _runtime logic_ is already well-factored into `pkg/hub` and `pkg/runtimebroker`, the **command layer** that wires everything together has become a maintenance burden. Key concerns:

- **Navigability**: Finding the relevant section for a specific concern (daemon lifecycle, broker registration, web server wiring, service install generation) requires scrolling through the entire file.
- **Test organization**: Tests are already split across 3 files (`server_test.go`, `server_bridge_test.go`, `server_workstation_test.go`), but the code they test is monolithic.
- **Coupling via package-level vars**: ~40 package-level flag variables create implicit state shared between the daemon launcher, the foreground runner, and the init function.
- **Duplicated defaults logic**: Workstation-mode defaults are applied in both `runServerStartOrDaemon` (for daemon arg construction) and `runServerStart` (for foreground execution), violating DRY.

## Current Structure Analysis

The file contains these logical sections:

| Lines (approx.) | Responsibility | Description |
|---|---|---|
| 62-103 | **Flag variables** | ~40 package-level `var` declarations for CLI flags |
| 105-260 | **Cobra command definitions** | `serverCmd`, `serverStartCmd`, `serverStopCmd`, `serverRestartCmd`, `serverStatusCmd`, `serverInstallCmd` |
| 262-300 | **Port checking** | `checkPort()` and `portStatus` type |
| 303-450 | **Daemon management** | `runServerStartOrDaemon()` - daemon launch, arg forwarding, quickstart |
| 452-554 | **Daemon stop/restart** | `runServerStop()`, `runServerRestart()` |
| 556-647 | **Server status** | `runServerStatus()`, `serverStatusInfo` type |
| 649-1688 | **Foreground server start** | `runServerStart()` - the core orchestration (~1040 lines) |
| 1690-1823 | **Global grove registration** | `registerGlobalGroveAndBroker()` |
| 1826-1994 | **Co-located dispatcher adapter** | `agentDispatcherAdapter` type and all its methods |
| 1996-2033 | **Broker profiles** | `buildStoreBrokerProfiles()` |
| 2035-2076 | **Debug/utility helpers** | `logOAuthDebug()`, `redactForDebug()` |
| 2078-2178 | **Service file generation** | `runServerInstall()`, `generateSystemdUnit()`, `generateLaunchdPlist()` |
| 2180-2203 | **Quickstart banner** | `printWorkstationQuickstart()` |
| 2206-2264 | **Flag registration** | `init()` function |
| 2266-2300 | **URL helpers** | `isLocalhostURL()`, `containerBridgeEndpoint()` |

### The core problem: `runServerStart()`

At ~1040 lines, `runServerStart()` is the primary offender. It sequentially:
1. Initializes logging (lines 650-736) - ~87 lines
2. Loads and reconciles configuration/flags (lines 738-868) - ~130 lines
3. Checks port availability (lines 890-922) - ~32 lines
4. Initializes the database/store (lines 954-994) - ~40 lines
5. Initializes dev auth (lines 1019-1050) - ~31 lines
6. Creates and configures the Hub server (lines 1052-1293) - ~241 lines
7. Creates and configures the Web server (lines 1296-1380) - ~84 lines
8. Creates and configures the Runtime Broker (lines 1382-1624) - ~242 lines
9. Wires up the dispatcher and heartbeat (lines 1627-1661) - ~34 lines
10. Prints startup banner and blocks on shutdown (lines 1663-1688) - ~25 lines

## Proposed Refactoring

### Strategy

Split `cmd/server.go` into focused files within the `cmd/` package. This keeps the refactoring contained to file organization without introducing new packages or changing any public API surface.

### Proposed File Layout

```
cmd/
  server.go                    # Cobra command definitions, init(), flag variables (~200 lines)
  server_daemon.go             # Daemon lifecycle: start/stop/restart/status (~280 lines)
  server_foreground.go         # runServerStart() orchestrator, refactored to call helpers (~300 lines)
  server_config.go             # Config loading, flag-to-config reconciliation, workstation defaults (~200 lines)
  server_logging.go            # Logging initialization (OTel, Cloud, request/message loggers) (~120 lines)
  server_hub.go                # Hub server creation, storage/secrets init, template bootstrap (~250 lines)
  server_web.go                # Web server creation and Hub-Web wiring (~100 lines)
  server_broker.go             # Broker server creation, identity resolution, co-located registration (~350 lines)
  server_dispatcher.go         # agentDispatcherAdapter type and all methods (~170 lines)
  server_install.go            # Service file generation (systemd, launchd) (~120 lines)
  server_helpers.go            # Small utility functions: port check, URL helpers, debug logging (~100 lines)
```

Estimated total: ~2190 lines across 11 files (vs 2300 in one file). Minor reduction from removing duplicated workstation defaults logic.

### File Responsibilities

#### `server.go` (~200 lines)
- Package-level flag variables
- All `cobra.Command` variable definitions (with `Use`, `Short`, `Long`, `RunE`)
- `init()` function for flag registration and command tree wiring
- This becomes the "table of contents" for the server command

#### `server_daemon.go` (~280 lines)
- `runServerStartOrDaemon()` - calls into `server_config.go` for defaults
- `runServerStop()`
- `runServerRestart()`
- `runServerStatus()` + `serverStatusInfo` type
- `printWorkstationQuickstart()`

#### `server_foreground.go` (~300 lines)
The refactored `runServerStart()` becomes a high-level orchestrator that delegates to helper functions in the other files:

```go
func runServerStart(cmd *cobra.Command, args []string) error {
    // 1. Init logging
    logCleanups, err := initServerLogging(cmd)
    // 2. Load & reconcile config
    cfg, resolved, err := resolveServerConfig(cmd)
    // 3. Check ports
    if err := checkServerPorts(cfg, resolved); err != nil { ... }
    // 4. Init store
    s, cleanup, err := initStore(ctx, cfg)
    // 5. Init dev auth
    devToken, err := initDevAuth(cfg, globalDir)
    // 6. Start Hub
    hubSrv, err := startHub(ctx, cfg, s, devToken, ...)
    // 7. Start Web
    webSrv, err := startWeb(ctx, cfg, hubSrv, devToken, ...)
    // 8. Start Broker
    err = startBroker(ctx, cfg, hubSrv, webSrv, s, ...)
    // 9. Block on shutdown
    return awaitShutdown(ctx, cancel, wg, errCh)
}
```

#### `server_config.go` (~200 lines)
- `resolveServerConfig(cmd) (*config.GlobalConfig, *resolvedConfig, error)` - loads config file, applies workstation defaults, reconciles CLI flag overrides
- `resolvedConfig` struct holding derived values (hubEndpoint, adminEmailList, adminMode, etc.)
- `applyWorkstationDefaults(cmd, cfg)` - single-source for workstation defaults (eliminates the duplication between daemon and foreground paths)

#### `server_logging.go` (~120 lines)
- `initServerLogging(cmd) (cleanups, requestLogger, messageLogger, error)` - all OTel, Cloud Logging, request logger, message logger initialization
- Returns cleanup functions and configured loggers

#### `server_hub.go` (~250 lines)
- `initHubServer(ctx, cfg, resolved, s, requestLogger, messageLogger) (*hub.Server, error)` - creates hub.ServerConfig, initializes storage, secrets, templates, notification channels, message broker
- Encapsulates the ~241 lines of Hub setup from `runServerStart()`

#### `server_web.go` (~100 lines)
- `initWebServer(ctx, cfg, resolved, hubSrv, requestLogger) (*hub.WebServer, error)` - creates web server config, wires Hub services, event publisher, health providers

#### `server_broker.go` (~350 lines)
- `initRuntimeBroker(ctx, cfg, resolved, hubSrv, s, ...) error` - broker identity resolution, credential generation, co-located registration, broker server creation
- `registerGlobalGroveAndBroker()` - moved here from `server.go`
- `buildStoreBrokerProfiles()` - moved here

#### `server_dispatcher.go` (~170 lines)
- `agentDispatcherAdapter` type definition
- All dispatcher methods: `DispatchAgentCreate`, `DispatchAgentStart`, `DispatchAgentStop`, `DispatchAgentRestart`, `DispatchAgentDelete`, `DispatchAgentMessage`
- `newAgentDispatcherAdapter()` constructor

#### `server_install.go` (~120 lines)
- `runServerInstall()`
- `generateSystemdUnit()`
- `generateLaunchdPlist()`

#### `server_helpers.go` (~100 lines)
- `checkPort()` + `portStatus` type
- `isLocalhostURL()`
- `containerBridgeEndpoint()`
- `logOAuthDebug()` + `redactForDebug()`

### Test File Mapping

Existing test files map naturally to the new source files:

| Current Test File | Tests | Maps To Source |
|---|---|---|
| `server_test.go` | `TestRegisterGlobalGroveAndBroker_*` | `server_broker.go` |
| `server_bridge_test.go` | `TestContainerBridgeEndpoint`, `TestIsLocalhostURL` | `server_helpers.go` |
| `server_workstation_test.go` | `TestWorkstationMode*`, `TestProductionMode*`, `TestPrint*`, `TestGenerate*` | `server_config.go`, `server_daemon.go`, `server_install.go` |

Tests can remain in their current files or be renamed to match new source files. Since they're all in `package cmd`, no imports change.

## What This Refactoring Does NOT Do

- **No new packages**: Everything stays in `cmd/`. This avoids creating `pkg/serversetup` or similar packages that would add import complexity for what is fundamentally command-layer wiring.
- **No interface extraction**: The helper functions return concrete types. We're splitting a file, not building an abstraction layer.
- **No behavioral changes**: Every code path remains identical. This is a pure file-organization refactoring.
- **No flag variable consolidation**: While the 40 package-level vars are a code smell, consolidating them into a struct would touch every flag reference and test. That's a separable follow-up if desired.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Merge conflicts with concurrent work on `server.go` | Do this as a single commit, rebase immediately |
| Test breakage from moved unexported functions | All files share `package cmd`, unexported symbols remain accessible |
| Increased file count in `cmd/` | The `server_` prefix groups files clearly; `cmd/` already has ~30+ files |

## Execution Plan

1. Create all new files and move code (no logic changes)
2. Extract helper functions from `runServerStart()` to reduce it to an orchestrator
3. Consolidate the duplicated workstation defaults logic into `applyWorkstationDefaults()`
4. Run `go build -buildvcs=false ./...` and `go test -tags no_sqlite ./...` to verify
5. Single commit: `refactor: split cmd/server.go into focused files by responsibility`

## Open Questions

1. **Should `agentDispatcherAdapter` move to a `pkg/` package?** It implements `hub.AgentDispatcher` and is tightly coupled to both `agent.Manager` and `store.Store`. Moving it to `pkg/hub` or `pkg/runtimebroker` would break the dependency direction (those packages shouldn't import each other). Keeping it in `cmd/` as glue code is correct for now.

2. **Should the flag variables be grouped into a `serverFlags` struct?** This would improve testability (pass struct instead of relying on globals) but touches every flag reference. Could be done as a follow-up.

3. **Should `resolvedConfig` capture all derived state?** Currently, several intermediate values (hubEndpoint, devAuthToken, brokerID, etc.) are local variables threaded through `runServerStart()`. A "resolved config" struct could capture these, but over-structuring a linear startup sequence can reduce readability. The proposal uses targeted helper functions with explicit return values instead.
