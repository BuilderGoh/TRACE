# TRACE defensive-execution controls and coverage

**Status:** source review, pinned-dependency dispatch inspection, isolated prototype harness
**Baseline:** `dfd4451` (`feat: establish TRACE branding and isolated development build`)
**Purpose:** document which execution controls are present, where they are wired, and which reachable paths are outside the verified Guard boundary.

## Summary

TRACE does not currently have a universal, fail-closed defensive-execution boundary. `guard.Guard` provides audit hooks and applies configured intercept rules when a session attaches those hooks. Coverage is not universal: selected chat and worker sessions use Guard hooks, while Planner and MainAgent sessions use permission bypass without verified Guard hook attachment. Some other paths call tools or start processes directly, outside agent-session dispatch.

The pinned `github.com/Autumn-27/norma v0.4.3` dispatch was inspected in disposable `golang:1.26.3` containers with this repository mounted read-only. The observed dispatch evaluates ordinary permission checks before `PreToolUse`; a blocking hook prevents the tool call. `ModeBypass` skips ordinary permission checks, but explicit denials and hooks that are actually wired may still block. These findings describe the inspected dependency version and code paths, not an OS sandbox or a universal authorization policy.

This is a Phase 1B.2 inventory plus an isolated prototype harness, not a claim that defensive-only mode is implemented or safe to enable in the running app. The harness is not wired into any baseline execution path.

## Isolated synthetic-data prototype

Package `restricteddfir` provides a small, standalone dispatch harness. Its only accepted tool name is `analyze_synthetic_events`; all other names are denied. The accepted operation strictly decodes a bounded JSON input, requires `synthetic: true`, rejects unknown fields and malformed event types, and returns deterministic sorted counts by event type. It performs no filesystem, process, network, agent, MCP, or external-tool operations. Positive and negative unit tests are in `restricteddfir/dispatch_test.go`.

The `synthetic` flag is a caller-provided label, not provenance verification. It cannot establish that data is genuinely synthetic, prevent a caller from mislabelling real evidence, or protect any other TRACE code path. The package is deliberately not registered with Norma, an agent, an API route, an MCP server, or any ordinary app configuration. Treat it only as a unit-testable policy/analyzer prototype. It must remain unavailable in the running app while alternate entry points and side-effect boundaries remain uncontrolled and untested.

Prototype verification status: `go test ./restricteddfir -v` passed in `golang:1.26.3` (all 5 top-level tests and their subtests), and `go vet ./restricteddfir` completed successfully. Validation used a disposable `docker run --rm --network none --read-only` container, mounted the repository read-only, and redirected Go build/module caches and temporary build files to a temporary host directory. An initial test attempt failed because Go's default `/tmp` was mounted `noexec`; setting `GOTMPDIR` to a separate executable container tmpfs resolved that harness configuration issue, with no source changes needed. A repository search found no package/tool references outside `restricteddfir` and this documentation. Docker reported the pre-existing `trace-artex-1` and `trace-postgres-1` containers running; validation did not start, stop, or modify them. These results validate only this package's unit behavior and vet checks, not a complete defensive execution boundary.

## Control behavior

`guard.Guard` registers `PreToolUse` and `PostToolUse` handlers. The pre-hook records an audit entry and, when an interceptor exists and the tool is enabled, evaluates configured rules against the tool name or serialized input. A matching rule can allow, deny, or request approval. If no rule matches, the configured fallback judge may be consulted; when it is unavailable or makes no decision, the hook allows the call (`guard/guard.go:30-55,57-132`; `intercept/intercept.go:518-561`).

Important limitations of this control:

- Guard source explicitly records that RoE authorization-scope enforcement was removed and destructive/exfiltration gating is no longer hard-coded. Such gating relies on ordinary database intercept rules that users can disable or delete (`guard/guard.go:1-7`).
- Intercept matching treats a rules-load error or empty rule list as no match (`intercept/intercept.go:521-525`). The Guard then may consult its fallback judge; without a judge decision the hook allows (`guard/guard.go:91-99`).
- An approval request blocks until a decision, cancellation, or configured timeout. The decision on timeout is configurable (`intercept/intercept.go:589-669`).
- Guard checks only apply when the execution path attaches its hook registry. The existence of a Guard, an audit record, a prompt instruction, a configured proxy, or a working directory does not by itself establish universal authorization or process/network isolation.

### Pinned Norma dispatch observations

Inspection of the exact dependency version in `go.mod` yielded these dispatch semantics:

1. Ordinary permission evaluation occurs before the `PreToolUse` hook.
2. A hook result that blocks prevents the underlying tool call.
3. `ModeBypass` skips ordinary permission evaluation. It does not by itself prove that hooks are skipped: explicitly wired hooks and explicit denials may still take effect.

Therefore `ModeBypass` is not equivalent to a proven bypass of every explicit Guard hook, and hook behavior cannot be assumed on sessions that do not attach Guard. This inspection did not test every dynamic tool-registration or alternate invocation path.

## Execution-path coverage

| Path | Verified wiring / execution | Defensive-execution assessment |
| --- | --- | --- |
| Interactive ChatAgent conversations, including profile-specific ChatAgents | Server setup calls `SetGuard(s.chatGuard())` for the global and profile agents (`server/server.go:582-586,722-729`). ChatAgent attaches the Guard hooks when non-nil (`agent/chat.go:73-75,157-165`). | Guard/intercept checks apply to calls dispatched through these sessions, subject to rule configuration, fallback behavior, and the dependency semantics above. Not a universal boundary for other callers. |
| Scheduled/triggered conversation turns | Triggered runs create a conversation and call `runConversationSync`; that resolves a ChatAgent and invokes `ca.Chat` (`server/conversations.go:525-532,603-624,870-894`). | Uses the ChatAgent path and its configured Guard when available. Does not extend coverage to direct calls elsewhere. |
| Worker intent execution | Engine builds worker hooks using `t.Guard.Hooks()` and passes them to worker execution (`server/engine.go:1041-1059`). | Guard applies where the task has a Guard and the worker tool call is made through this session. The inspected wiring alone does not prove every other task role shares this control. |
| Planner goal decomposition | Goal decomposition constructs an agent session with `permission.ModeBypass` (`agent/goals.go:168-178`). | No Guard hook is attached in this construction. Ordinary permission checks are bypassed; no Guard enforcement is verified for this path. |
| Planner rounds | Planner execution uses a bypass permission mode; no Guard hook wiring was identified in the inspected planner construction (`agent/planner.go`, `server/task_llm.go`). | No Guard enforcement is verified for this path. |
| MainAgent conversations | MainAgent builds domain and SDK default tools, augments them, and sets `permission.ModeBypass`; its session options do not attach Guard hooks (`agent/mainagent.go:112-166`). | Ordinary permission checks are bypassed and no Guard hook is verified here. Includes Bash, WebFetch, optional WebSearch, and dynamically augmented tools in the configured tool set. |
| Direct delegated/orchestration tool calls | Orchestration selects a tool and calls `pick(tsx).Call(ctx, inner, nil)` directly (`server/orchestration.go:150-171`). | This invocation does not go through an agent session's permission/hook dispatch. Any checks inside the selected tool are path-specific. |
| Direct MCP client calls | The MCP HTTP client implements a `CoreTool`-shaped remote MCP client (`mcphttp/client.go`); direct MCP call sites include ScopeSentry synchronization (`server/sync_scopesentry.go`). | Direct calls can bypass agent dispatch and its Guard hooks. MCP server behavior and remote effects have separate trust boundaries. |
| Custom command tools | Command tools invoke `actool.NewBash().Call` directly (`server/customtool.go:345-360`). | Uses Bash's execution implementation, but this direct handler call does not itself establish that the agent-session Guard hook ran. |
| Custom script tools | Python code is written to a temporary file and started with `exec.CommandContext` (`server/customtool.go:363-429`). | Separate process execution path; no universal Guard check or OS isolation is established by the inspected code. |
| Custom HTTP tools | The handler builds an HTTP request and calls `client.Do` (`server/customtool.go:432-470`). | Direct network path; proxy selection is configurable and is not an authorization boundary. |
| Tool usage recording | The metering wrapper records a row and then calls the wrapped tool (`server/tool_usage.go:18-48`). | Usage telemetry only; it does not authorize or block the call. |
| Skills and MCP augmentation | Agent assembly adds dynamically visible skill/MCP tools and resolves catalogued built-ins; MCP/skill/host tools pass through the built-in resolver (`agent/assembly.go:26-72`). | The effective tool set is assembled from several sources. Visibility, deferred unlock, panic recovery, and catalog selection are not substitutes for a common policy enforcement point. |

The table is a static wiring inventory, not an exhaustive runtime enumeration of every tool that an operator may configure or expose. Direct tool calls may have local validation or logging, but that is not evidence that the Guard ran.

## What the current controls do not establish

The inspected implementation does not establish:

- A code-enforced default-deny allowlist for all commands, network destinations, MCP tools, scripts, or dynamically added capabilities.
- Uniform Guard coverage across Planner, MainAgent, workers, scheduled/triggered conversations, direct MCP calls, and direct `CoreTool.Call` paths.
- That user-editable intercept rules cannot be disabled, deleted, weakened, or rendered ineffective by rule-load errors or fallback configuration.
- Filesystem, network, privilege, or resource isolation for Bash, Python/custom tools, MCP servers, or external utilities.
- That proxy configuration restricts destinations; it routes traffic and may record it, but does not by itself define an allowlist.
- Chain of custody or immutability for original evidence. Transactional finding persistence and optional traffic references are useful provenance features but do not prove an end-to-end forensic evidence boundary (`evidence/store.go:183-196`; `db/finding_traffic.go:342-407`).

## Phase 1B.2 conclusion

The checked-in baseline has useful, configurable interception and audit functionality on selected agent-session paths, but no verified universal enforcement choke point. The isolated package prototype does not change this conclusion. Phase 1's defensive-only exit criterion remains unmet: the roadmap requires agent conversations and tool calls to stay within an authorized evidence-analysis environment (`docs/TRACE_ROADMAP.md:16-24`).

Before claiming a defensive-only mode, the implementation needs a shared enforcement boundary immediately before side effects, explicit fail-closed behavior, coverage of direct and dynamically added tool paths, and tests showing prohibited calls are denied through each entry point. Process and network isolation must be designed and tested separately from agent hooks.

## Source index

- `go.mod` — pinned Go and Norma versions.
- `guard/guard.go:1-7,30-55,57-132` — Guard scope, hooks, audit, and intercept decision flow.
- `intercept/intercept.go:518-561,589-669` — rule evaluation and approval wait/timeout behavior.
- `agent/chat.go:73-75,157-165` — ChatAgent Guard attachment.
- `agent/goals.go:168-178` — goal decomposition permission mode.
- `agent/mainagent.go:112-166` — MainAgent tools and session options.
- `agent/planner.go` and `server/task_llm.go` — planner execution and construction.
- `server/server.go:582-586,722-729` — global/profile ChatAgent Guard wiring.
- `server/engine.go:1041-1059` — worker Guard hook wiring and execution.
- `server/conversations.go:525-532,603-624,870-894` — chat and triggered conversation execution.
- `server/orchestration.go:150-171` — direct selected-tool invocation.
- `mcphttp/client.go` and `server/sync_scopesentry.go` — MCP client and direct call path.
- `server/customtool.go:345-429,432-470` — custom command, script, and HTTP execution.
- `server/tool_usage.go:18-48` — metering wrapper.
- `agent/assembly.go:26-80` — tool augmentation, resolution, aliases, and panic wrapper.
- `docs/TRACE_ROADMAP.md:16-24` — Phase 1 tasks and exit criterion.
