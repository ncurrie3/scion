# Inline Agent Configuration: Late-Binding Config at Agent Creation Time

**Status:** Draft
**Created:** 2026-03-06
**Related:** [agent-config-flow.md](./agent-config-flow.md), [hosted-templates.md](./hosted/hosted-templates.md)

---

## 1. Overview

### Problem

Today, agent configuration is assembled from a multi-layered composition of templates, harness configs, settings profiles, and CLI flags. To customize an agent beyond the available CLI flags, users must:

1. Create a custom template directory with a `scion-agent.yaml`
2. Optionally create a custom harness-config directory
3. Reference these by name at agent creation time

This works well for reusable configurations but creates friction for one-off or exploratory use cases. Every new tunable that a user might want to set ad-hoc requires either a new CLI flag (leading to flag proliferation — e.g., `--enable-telemetry`, `--model`, `--max-turns`) or a new template.

The Hub web UI amplifies this problem: building a form that exposes all agent options requires either generating templates server-side or adding every field as a discrete API parameter.

### Goal

Allow agents to be started with an **inline configuration object** — a self-contained document that can express the full range of agent configuration without requiring pre-existing template or harness-config artifacts on disk.

Conceptually, this is "just-in-time" or "late-binding" configuration: the agent's settings are provided at creation time rather than being pre-staged as template artifacts. The implementation achieves this by evolving `ScionConfig` into a superset that absorbs fields currently scattered across harness-config entries, template content files, and CLI flags.

### Design Principles

1. **Additive** — Inline config is a new input path, not a replacement. Templates and harness configs continue to work as before.
2. **Superset via evolution** — `ScionConfig` itself is expanded to absorb fields that today live outside the config file (system prompt content, harness-config details). No new parallel config type is introduced.
3. **Explicit over composed** — When an inline config is provided, its values are authoritative. The multi-layer merge behavior is simplified: inline config is the "template equivalent," composed only with broker/runtime-level concerns.
4. **Backwards compatible** — Existing `scion start --type my-template` workflows are unaffected. Existing `scion-agent.yaml` files remain valid.

---

## 2. Current State

### Configuration Sources (Precedence, Low -> High)

```
Embedded defaults
  -> Global settings (~/.scion/settings.yaml)
    -> Grove settings (.scion/settings.yaml)
      -> Template chain (scion-agent.yaml, inherited)
        -> Harness-config (config.yaml + home/ files)
          -> Agent-persisted config (scion-agent.json)
            -> CLI flags (--image, --enable-telemetry, etc.)
```

### What Lives Where Today

| Concern | Where It's Defined | Format |
|---------|-------------------|--------|
| Harness type | Template `scion-agent.yaml` | `harness: claude` |
| Container image | Harness-config `config.yaml` or template | `image: ...` |
| Environment vars | Template, harness-config, settings, CLI | `env: {K: V}` |
| System prompt | Template directory file (`system-prompt.md`) | Markdown file |
| Agent instructions | Template directory file (`agents.md`) | Markdown file |
| Model selection | Template or harness-config | `model: claude-opus-4-6` |
| Auth method | Harness-config or settings profile | `auth_selected_type: api-key` |
| Container user | Harness-config `config.yaml` | `user: scion` |
| Volumes | Template, harness-config, settings | `volumes: [...]` |
| Resources | Template or settings profile | `resources: {requests: ...}` |
| Telemetry | Settings, template, or CLI flag | `telemetry: {enabled: true}` |
| Services (sidecars) | Template | `services: [...]` |
| Max turns/duration | Template | `max_turns: 100` |
| Home directory files | Harness-config `home/` directory | Filesystem artifacts |

### Key Observation

The current `ScionConfig` struct already captures most of these concerns. The gaps are:

1. **System prompt and agent instructions** — stored as file references in `ScionConfig` (`system_prompt: system-prompt.md`) but the actual content lives as files alongside the template. The config references a filename, not inline content.
2. **Harness-config details** — the container user, task flag, default CLI args, and auth method come from `HarnessConfigEntry`, not from `ScionConfig`.
3. **Home directory files** — harness-config `home/` directories provide files like `.claude.json`, `.bashrc`, etc. These are filesystem artifacts that can't be expressed inline. This is the trickiest gap to close, and we accept that templates and harness configs will remain the mechanism for home directory file provisioning. The goal is to capture as much of the common-denominator configuration as possible in the expanded `ScionConfig`.

### Existing Partial Precedent

The `AgentConfigOverride` struct in `pkg/hub/handlers.go` already provides a limited inline override:

```go
type AgentConfigOverride struct {
    Image    string            `json:"image,omitempty"`
    Env      map[string]string `json:"env,omitempty"`
    Detached *bool             `json:"detached,omitempty"`
    Model    string            `json:"model,omitempty"`
}
```

And `hubclient.AgentConfig` similarly has `Image`, `HarnessConfig`, `HarnessAuth`, `Env`, `Model`, `Task`. These are narrow override surfaces that exist because `ScionConfig` wasn't expressive enough to serve as the inline config document. By expanding `ScionConfig` to cover these cases, we can replace `AgentConfigOverride` and thread a full `ScionConfig` through more of the agent creation process — eliminating the need for ad-hoc override structs.

---

## 3. Proposed Design

### 3.1 Expanded ScionConfig Schema

Rather than introducing a new parallel type, we expand `ScionConfig` itself with the fields that today require separate artifacts. This keeps a single authoritative config type and ensures that `scion-agent.yaml` files in templates immediately gain inline content support.

New fields added to `ScionConfig`:

```go
// Added to the existing ScionConfig struct in pkg/api/types.go

// === Content fields (inline instead of file references) ===
// When set, these contain the actual content rather than a filename.
// The content resolution logic checks: if the value looks like a filename
// (no newlines, ends in a known extension), treat it as a file reference;
// otherwise treat it as inline content.
//
// Note: system_prompt and agent_instructions already exist on ScionConfig
// as file-reference fields. The change is in how the values are resolved,
// not in the schema itself.

// === Harness-config fields absorbed into ScionConfig ===
User             string   `json:"user,omitempty" yaml:"user,omitempty"`             // Container unix user
AuthSelectedType string   `json:"auth_selected_type,omitempty" yaml:"auth_selected_type,omitempty"`

// === Agent operational parameters ===
Task             string   `json:"task,omitempty" yaml:"task,omitempty"`
Branch           string   `json:"branch,omitempty" yaml:"branch,omitempty"`
```

#### Content Field Resolution

The `system_prompt` and `agent_instructions` fields already exist on `ScionConfig` today, but only accept filenames. The key change is extending the content resolution logic:

```go
func ResolveContent(value string, templateDir string) (string, error) {
    // If the value contains newlines, it's inline content
    if strings.Contains(value, "\n") {
        return value, nil
    }
    // If it looks like a file path, try to read it from the template dir
    if templateDir != "" && looksLikeFilePath(value) {
        content, err := os.ReadFile(filepath.Join(templateDir, value))
        if err == nil {
            return string(content), nil
        }
    }
    // Fall back to treating as inline content
    return value, nil
}
```

This is fully backwards compatible: existing `system_prompt: system-prompt.md` references continue to resolve as file paths; new inline content like `system_prompt: "You are a code reviewer."` works without any template directory.

#### Pressure Testing: Why Expand ScionConfig Instead of a New Type?

A separate type (e.g., `JITAgentConfig`) was considered and rejected. Here is the analysis:

**Concern: Bloating `scion-agent.json`**

`ScionConfig` is serialized to `scion-agent.json` in every agent directory. Adding fields like `user`, `task`, and potentially large inline content could bloat this file.

*Mitigation:* Fields like `task` and `branch` are operational parameters that are consumed at agent creation time and don't need to persist in `scion-agent.json`. We can use a `json:"-"` tag or a separate serialization path to exclude them from the persisted config. The `user` field is a small string. Inline content for `system_prompt` and `agent_instructions` replaces what would otherwise be a separate file on disk — the total data stored is the same.

**Concern: Conceptual muddling — input format vs. resolved config**

`ScionConfig` serves as both a user-facing input (`scion-agent.yaml`) and a resolved/persisted output (`scion-agent.json`). Adding operational parameters like `task` blurs this.

*Mitigation:* This is manageable with field-level documentation and selective serialization. The benefit of a single type — no conversion logic, no impedance mismatch, no two types drifting out of sync — outweighs the conceptual cost. Fields can be annotated with comments distinguishing "input-only" from "persisted" fields.

**Concern: Some fields only make sense in certain contexts**

`Branch` and `task` are meaningful at creation time but not in a template. `user` is meaningful in a template but is currently on `HarnessConfigEntry`.

*Mitigation:* Templates can simply omit fields that don't apply. YAML/JSON `omitempty` handles this naturally. A field being present on the type doesn't mean it must be set in every context.

**Verdict:** Expanding `ScionConfig` is the right approach. The single-type model is simpler, avoids conversion logic, and means improvements to the schema automatically benefit both templates and inline configs.

### 3.2 Threading ScionConfig Through Agent Creation

Today, agent creation involves assembling config from multiple sources into a `ScionConfig`, plus separately extracting `HarnessConfigEntry` fields and content files. With the expanded `ScionConfig`:

1. **`ScionConfig` becomes the single carrier** for configuration data through the creation pipeline. The provisioning code receives one `ScionConfig` rather than a `ScionConfig` plus side-channel overrides.
2. **`AgentConfigOverride` is replaced.** The Hub API and `hubclient.AgentConfig` can accept a full `ScionConfig` instead of an ad-hoc subset of fields. This eliminates the need for `AgentConfigOverride` and its limited field set.
3. **`HarnessConfigEntry` fields are resolved from `ScionConfig`.** When `user` or `auth_selected_type` is set on `ScionConfig`, those values are used. When not set, the harness-config defaults still apply. The `HarnessConfigEntry` struct remains for harness-config files (`config.yaml`), but its values are lower-precedence than `ScionConfig`.

```
Precedence with inline config:

Embedded defaults
  -> Global/Grove settings
    -> Template scion-agent.yaml
      -> Harness-config config.yaml (for user, auth, home/ files)
        -> Inline config (--config file) merged over template
          -> CLI flags (--image, etc.) merged over inline config
            -> Runtime concerns (env expansion, auth injection)
```

Home directory files (`.claude.json`, `.bashrc`, etc.) remain the domain of harness-config `home/` directories. The `harness_config` field on `ScionConfig` can reference a harness-config by name to pick up these files, even when all other config is provided inline.

### 3.3 Merge Semantics When Inline Config Is Present

When an inline config is provided:

```
Base template (if --type also specified)
  -> Inline config merged over base (inline wins)
    -> CLI flags merged over inline (flags win)
      -> Runtime concerns (auth, env expansion) applied last
```

When an inline config is provided **without** `--type`:
- The `harness` field in the config determines the harness (required in this case)
- A harness-config name can still be specified to pick up `home/` directory files
- If no harness-config is specified, the harness's embedded defaults are used

This means inline config **replaces** the template layer but still composes with harness-config `home/` files and runtime-level concerns.

### 3.4 CLI Interface

```bash
# From a file
scion start my-agent --config agent-config.yaml

# From stdin (pipe from another tool)
cat config.yaml | scion start my-agent --config -

# Combined with a base template (inline overrides template)
scion start my-agent --type base-template --config overrides.yaml

# CLI flags still override everything
scion start my-agent --config config.yaml --image custom:latest
```

The `--config` flag accepts a path to a YAML or JSON file. A value of `-` reads from stdin.

### 3.5 Hub API Interface

The existing `CreateAgentRequest` is extended to accept a full `ScionConfig`:

```go
// In pkg/hub/handlers.go
type CreateAgentRequest struct {
    Name          string        `json:"name"`
    GroveID       string        `json:"groveId"`
    Template      string        `json:"template,omitempty"`
    // ... existing fields ...

    // Config provides a complete inline agent configuration.
    // When set, this replaces the template as the primary config source.
    // If Template is also set, Config is merged over the template config.
    // This replaces the previous AgentConfigOverride approach.
    Config        *ScionConfig  `json:"config,omitempty"`
}
```

The Hub handler treats `Config` as a template-equivalent: it extracts the relevant fields and passes them through to the broker in the `RemoteCreateAgentRequest`.

### 3.6 Web UI Integration

With inline config, the Hub web UI can present a form with all agent options:

```
+---------------------------------------------+
|  Create Agent                               |
+---------------------------------------------+
|  Name: [________________]                   |
|  Grove: [dropdown________]                  |
|                                             |
|  -- Configuration --                        |
|  Base Template: [optional dropdown]         |
|  Harness: [claude v]                        |
|  Model: [________________]                  |
|  Image: [________________]                  |
|                                             |
|  -- Limits --                               |
|  Max Turns: [____]  Max Duration: [___]     |
|                                             |
|  -- Environment --                          |
|  [KEY] = [VALUE]        [+ Add]             |
|                                             |
|  -- System Prompt --                        |
|  [multiline editor........................  |
|  .........................................  |
|                                             |
|  -- Task --                                 |
|  [multiline editor........................  |
|  .........................................  |
|                                             |
|  [Advanced: Resources, Telemetry, ...]      |
|                                             |
|  [Create Agent]                             |
+---------------------------------------------+
```

The form serializes to a `ScionConfig` JSON object and sends it as `config` in the create request. No template creation needed.

---

## 4. Implementation Approach

### Phase 1: Expand ScionConfig and Add CLI `--config` Flag

**Scope:** Add the new fields to `ScionConfig`, implement content resolution for inline values, and add `--config <path>` to `scion start` and `scion create`.

**Changes:**
- `pkg/api/types.go` — Add `user`, `auth_selected_type`, `task`, `branch` fields to `ScionConfig`. Annotate `task` and `branch` with `json:"-"` to exclude from persisted config.
- `pkg/agent/provision.go` — Extend content resolution to support inline `system_prompt` and `agent_instructions` values (detect inline content vs. file references). When `user` or `auth_selected_type` is set on `ScionConfig`, apply these during harness-config resolution.
- `cmd/start.go` / `cmd/common.go` — Add `--config` flag, load file, parse as `ScionConfig`, merge into the provisioning flow.

**Key detail:** When `--config` is provided without `--type`, the provisioning path skips template loading and uses the inline config as the base. The harness-config `home/` directory is still applied (based on the `harness_config` field or the harness default).

**Validation:**
- If neither `--type` nor `--config` is provided, existing behavior (default template)
- If `--config` is provided, `harness` must be specified either in the config or via a base template
- If both `--type` and `--config`, merge config over template

### Phase 2: Hub API Support (Replace AgentConfigOverride)

**Scope:** Extend Hub create-agent API to accept a full `ScionConfig`. Deprecate `AgentConfigOverride`.

**Changes:**
- `pkg/hub/handlers.go` — Accept `config` (`*ScionConfig`) in `CreateAgentRequest`; merge with template if both provided; pass through to dispatcher. Deprecate the existing `AgentConfigOverride` field with a compatibility shim that converts it to a partial `ScionConfig`.
- `pkg/hub/httpdispatcher.go` — Include `ScionConfig` fields in `RemoteCreateAgentRequest`
- `pkg/runtimebroker/handlers.go` — Accept and apply the full `ScionConfig` during agent provisioning
- `pkg/runtimebroker/types.go` — Replace per-field overrides in `CreateAgentConfig` with a `ScionConfig` field

**Design decision:** The Hub resolves the `ScionConfig` into the existing `RemoteAgentConfig` fields, keeping the broker interface stable. The broker doesn't need to know whether config came from a template or inline. Option (B) from the original design — centralize conversion in the Hub.

### Phase 3: Web UI Form

**Scope:** Add agent creation form to the Hub web UI that generates a `ScionConfig`.

**Changes:**
- `web/src/client/` — Agent creation form component
- `web/src/server/` — API pass-through (already handled by Hub API)

This phase is purely frontend work once Phase 2 is complete.

### Phase 4: Config Export and Sharing

**Scope:** Allow exporting an existing agent's resolved config as a `ScionConfig` file, enabling config sharing and reproduction.

```bash
# Export current agent config as a reusable config file
scion config export my-agent > agent-config.yaml

# Start a new agent with the same config
scion start new-agent --config agent-config.yaml
```

**Changes:**
- `cmd/config.go` — Add `config export` subcommand
- `pkg/agent/` — Read agent's `scion-agent.json` + content files, produce a complete `ScionConfig`

---

## 5. Alternative Approaches Considered

### A: Separate JITAgentConfig Type (Original Draft Approach)

Introduce a new `JITAgentConfig` type that is a superset of `ScionConfig`, with conversion methods (`ToScionConfig()`, `ToHarnessConfigEntry()`).

**Pros:**
- Clean separation between user input format and resolved config
- No risk of bloating `scion-agent.json` with input-only fields
- Clear boundary: `ScionConfig` is the resolved config, `JITAgentConfig` is the input

**Cons:**
- Two types that must stay in sync — every new field requires updates in both places
- Conversion logic adds complexity and is a source of bugs
- The Hub, CLI, and broker all need to understand both types
- `AgentConfigOverride` would still exist as a third type, or need its own migration

**Verdict:** Rejected. The maintenance cost of two parallel config types outweighs the conceptual cleanliness. Selective serialization (`json:"-"` tags, separate marshal methods) handles the input-vs-persisted distinction within a single type.

### B: Templates-as-JSON via API (Ephemeral Templates)

Instead of a new config format, the Hub could create ephemeral/anonymous templates from the web UI form, then reference them normally.

**Pros:**
- No new config path — reuses existing template machinery
- Broker doesn't change at all

**Cons:**
- Creates invisible template artifacts that need lifecycle management
- Ephemeral templates need garbage collection
- Adds latency (create template -> start agent, two-step)
- Doesn't solve the CLI use case

**Verdict:** Rejected. Adds complexity without solving the core problem.

### C: Flag Proliferation (Status Quo Extended)

Continue adding CLI flags for each new option (`--model`, `--max-turns`, `--system-prompt`, etc.).

**Pros:**
- Simple, incremental
- No new concepts

**Cons:**
- Doesn't scale — `ScionConfig` has 20+ fields, many with nested structure
- Each new field requires changes to `cmd/`, `StartOptions`, and all the plumbing
- Can't express complex structures (telemetry config, services) via flags
- Web UI still needs a different solution

**Verdict:** Rejected as a strategy. Individual high-use flags (`--model`) may still be added for convenience alongside `--config`.

### D: Inline Config as Complete Override (No Template Merge)

When `--config` is provided, completely ignore templates — no merge, no composition.

**Pros:**
- Simpler mental model: "config file = everything"
- No ambiguity about precedence

**Cons:**
- Users can't use a template as a base and override a few fields
- Forces duplication if you want "template X but with a different model"
- Loses the composability that makes the current system flexible

**Verdict:** Rejected as default behavior, but could be offered as an opt-in mode (`--config-only` or a field in the config itself: `standalone: true`).

---

## 6. Open Questions

### Q1: Should inline config support inline home directory files?

Today, harness-config `home/` directories provide files like `.claude.json` and `.bashrc`. Should inline config support declaring these inline?

```yaml
home_files:
  ".claude.json": |
    {"permissions": {"allow": ["Bash", "Read"]}}
  ".bashrc": |
    export PS1="$ "
```

**Considerations:**
- Powerful but complex — files can be binary (images, compiled configs)
- Significantly increases the config surface area
- Home directory files are the trickiest part of the config to capture inline, since they are filesystem artifacts rather than structured configuration
- Alternative: reference a harness-config by name for `home/` files, only override config values inline

**Recommendation:** Defer to a later phase. For Phase 1, the `harness_config` field can reference an existing harness-config for `home/` files. Inline file support can be added later if there is demonstrated demand for it.

### Q2: How should content field resolution distinguish filenames from inline content?

The `system_prompt` and `agent_instructions` fields need to support both file references (`system_prompt: system-prompt.md`) and inline content (`system_prompt: "You are a code reviewer."`). The resolution heuristic matters.

**Options:**
- **Newline detection:** If the value contains `\n`, it's inline content; otherwise try as a filename first. Simple but could misclassify single-line inline content if a file with that name exists.
- **Explicit prefix/suffix:** Use a convention like `file:system-prompt.md` for file references, bare strings for inline. Breaking change for existing templates.
- **Separate fields:** `system_prompt` for filenames, `system_prompt_content` for inline. Avoids ambiguity but adds field proliferation.
- **Try file first, fall back to inline:** Attempt to read the value as a file path relative to the template directory. If no template directory or file doesn't exist, treat as inline content.

**Recommendation:** "Try file first, fall back to inline." When a template directory exists, attempt file resolution; if the file is not found (or no template directory), treat the value as inline content. This is fully backwards compatible and handles the common cases naturally. Edge case: a user provides inline content that happens to match a filename in the template directory. This is unlikely in practice and can be documented.

### Q3: How should `--config` interact with `--type` for content fields?

If a template has `agents.md` and the inline config also has `agent_instructions`, the inline config wins (standard merge). But what about partial specification?

```yaml
# Inline config sets system_prompt but not agent_instructions
system_prompt: "You are a careful code reviewer."
# Should agent_instructions come from the base template?
```

**Recommendation:** Yes — standard merge semantics. Inline config fields override template fields when set; template fields are preserved when the inline config field is empty. This matches existing `MergeScionConfig` behavior.

### Q4: Should the config schema version independently?

Templates have `schema_version: "1"`. The expanded `ScionConfig` used with `--config` should also have a schema version.

**Recommendation:** Use the same `schema_version` field that already exists on `ScionConfig`. Since we're expanding `ScionConfig` rather than creating a new type, the version tracks the same schema. Start at `"1"` (or the current version). Inline configs and template configs evolve together.

### Q5: Validation strictness

Should inline config be validated more strictly than template config? For example:
- Require `harness` to be set (templates can inherit it)
- Require `image` to be set (harness-configs provide defaults)

**Recommendation:** When used standalone (no `--type`), require `harness`. Other fields fall back to harness-config or embedded defaults, same as templates today. The point is to be explicit, not necessarily exhaustive.

### Q6: How should `task` and `branch` persist (or not)?

These fields are operational parameters — they're consumed at agent creation time but are not part of the agent's durable configuration. Options:

- **`json:"-"` tag:** Exclude from `scion-agent.json` serialization entirely. Simple but means exported configs lose this information.
- **Separate serialization:** Use a custom `MarshalJSON` that excludes input-only fields. More control but adds complexity.
- **Just include them:** Let `task` and `branch` persist in `scion-agent.json`. They're small strings and having them in the persisted config aids debugging and config export (Phase 4).

**Recommendation:** Include them in `scion-agent.json`. The storage cost is negligible, and having the original task and branch in the persisted config is useful for auditability and config export.

### Q7: Deprecation path for AgentConfigOverride and hubclient.AgentConfig

Once `ScionConfig` is threaded through the Hub API, `AgentConfigOverride` and the ad-hoc fields on `hubclient.AgentConfig` become redundant.

**Options:**
- **Immediate removal:** Clean but may break existing Hub API clients.
- **Compatibility shim:** Accept both old-style overrides and new-style `ScionConfig`. If both are present, convert old-style to a partial `ScionConfig` and merge (new-style takes precedence). Deprecation warning in logs.
- **Parallel support:** Keep both indefinitely, document that `config` takes precedence.

**Recommendation:** Compatibility shim in Phase 2. The Hub handler converts any `AgentConfigOverride` into a partial `ScionConfig` internally, so the rest of the pipeline only deals with `ScionConfig`. Mark `AgentConfigOverride` as deprecated in the API docs. Remove in a subsequent release.

### Q8: How does inline config interact with the env-gather flow?

The env-gather flow (`GatherEnv: true`) evaluates whether required environment variables are present and prompts the user to supply missing ones. Inline config might declare `secrets` that trigger this flow.

**Recommendation:** The `secrets` field feeds into the same env-gather pipeline. No special handling needed — the broker evaluates completeness the same way regardless of config source.

---

## 7. Migration and Compatibility

### No Breaking Changes

- Existing `scion start --type <template>` continues to work identically
- Existing Hub API `CreateAgentRequest` without `config` is unchanged
- Existing `scion-agent.yaml` template format is unchanged and gains inline content support for free
- `AgentConfigOverride` continues to work via compatibility shim (deprecated)

### Migration Path

1. **Phase 1:** `ScionConfig` gains new fields. `--config` flag added to CLI. No API changes.
2. **Phase 2:** Hub API gains `config` field. `AgentConfigOverride` deprecated with compatibility shim. `hubclient.AgentConfig` ad-hoc fields deprecated.
3. **Future:** Remove `AgentConfigOverride` and `hubclient.AgentConfig` ad-hoc fields once all clients have migrated to sending a full `ScionConfig`.

---

## 8. Example Config Files

### Minimal: Just override model and add telemetry

```yaml
schema_version: "1"
harness: claude
model: claude-sonnet-4-6
telemetry:
  enabled: true
```

### Full-featured: Code reviewer agent

```yaml
schema_version: "1"
harness: claude
model: claude-opus-4-6
image: us-central1-docker.pkg.dev/my-project/scion/scion-claude:latest

system_prompt: |
  You are a meticulous code reviewer. Focus on:
  - Security vulnerabilities
  - Performance issues
  - API contract violations
  Review only the files that changed. Be concise.

agent_instructions: |
  Review the current branch against main.
  Use `git diff main...HEAD` to see changes.
  Write your review as comments in a new file: REVIEW.md

env:
  REVIEW_STRICTNESS: high
  MAX_FILE_SIZE: "10000"

max_turns: 50
max_duration: 30m

resources:
  requests:
    cpu: "2"
    memory: 4Gi

task: "Review the latest changes on this branch"
```

### Template-based with overrides

```yaml
# Used with: scion start reviewer --type code-review --config this-file.yaml
schema_version: "1"
model: claude-sonnet-4-6  # Override the template's default model
env:
  REVIEW_STRICTNESS: low  # Override one env var, template's others preserved
max_turns: 20             # Shorter review
```
