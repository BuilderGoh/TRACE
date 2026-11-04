# TRACE agent architecture and defensive-transformation audit

**Status:** source audit of the checked-out `feat/trace-foundation` tree
**Baseline reviewed:** `dfd4451` (`feat: establish TRACE branding and isolated development build`)
**Purpose:** record the agent architecture and assess how far its current controls support TRACE's planned defensive DFIR scope.

## Executive summary

TRACE currently retains ARTEX's authorized-penetration-testing agent architecture and behavior. The development documentation says that TRACE's planned DFIR workflows and safety boundaries are not implemented yet (`docs/TRACE_DEVELOPMENT.md:47-55`), and the roadmap marks the Phase 1 defensive transformation tasks as incomplete (`docs/TRACE_ROADMAP.md:16-24`). The present system has useful policy and audit components, but they do not constitute a complete defensive-only enforcement boundary.

The main verified observations are:

- Planner, worker, and main-agent prompts retain offensive pentest framing and guidance (`agent/planner.go:298-340`, `agent/mainagent.go:84-101`; the worker prompt is defined in `agent/worker.go`). Prompts can be database-edited and have built-in fallbacks, but prompt wording is not equivalent to tool-level authorization.
- The Guard runs configured intercept rules on tool calls only where its hooks are attached. The Guard source explicitly says the RoE authorization-scope mechanism was removed and destructive/exfiltration checks now depend on ordinary database rules that users can disable or delete (`guard/guard.go:1-7`).
- Task constraints are rendered into planner/worker prompts as high-priority instructions (`agent/constraints.go:9-49`); the inspected implementation does not itself block a tool call based on those constraints.
- Asset allow/block rules gate selected asset-oriented paths, not every command, network request, or tool call. The asset insertion path skips denied assets, but ignores rule-loading errors and can proceed without those rules (`agent/tools_insert.go:230-250`).
- Finding storage has a comparatively strong transactional path: the evidence store prepares traffic references and records the finding under an evidence transaction; the DB validates task/exploration and intent relationships (`evidence/store.go:183-196`, `db/finding_traffic.go:342-407`). HTTP traffic references are optional and do not define whether the finding itself is persisted (`agent/tools.go:1224-1226`).
- Runtime enforcement in `github.com/Autumn-27/norma v0.4.3` has not been independently verified in this audit. Go is unavailable in the environment, and the dependency source was not found in the inspected module cache. Conclusions about hook behavior are therefore limited to the repository's wiring and call sites.

**Overall assessment:** treat the current branch as a development baseline, not as a defensive-only product. The roadmap's Phase 1 exit criterion—agent conversations and tool calls remaining within an authorized evidence-analysis environment—has not been demonstrated.

## Scope and method

This is a static source review of the checked-out repository, focused on agent prompts, tool registration, guard/intercept wiring, asset constraints, findings/evidence persistence, task execution, and the stated transformation roadmap. It is not a runtime test, dependency audit, deployment review, or authorization assessment. Repository paths and function names below identify the reviewed implementation; line numbers refer to the audited tree and may shift as code changes.

No application source or configuration was changed for this audit. The Go test suite and runtime behavior were not verified because the Go toolchain was unavailable. In particular, the Norma agent-core's semantics for `PermissionModeBypass`, hook execution, and tool dispatch remain outside the evidence available here.

## Architecture and verified behavior

### Agent roles and orchestration

The application has three principal task-agent roles:

1. **Main agent:** human-facing interface for observing progress and steering tasks. Its prompt allows operators to add hints and intents, steer work, set goals, and register task constraints (`agent/mainagent.go:87-101`). It builds domain tools together with `actool.DefaultTools()` (`agent/mainagent.go:125-130`).
2. **Planner:** evaluates the exploration graph, judges goals, and creates follow-up intents. The built-in planner prompt describes an authorized penetration-testing system, encourages deeper exploitation routes, and assigns quantitative coverage checks and task completion behavior (`agent/planner.go:298-340`). The `prove_goal` tool sets a goal to `met` after local goal/evidence-node validation; when all goals are met it sets the task's `GoalMet` flag (`agent/tools.go:1157-1202`). A separate `goal_met` tool directly sets that flag (`agent/tools.go:1205-1214`).
3. **Worker:** executes one claimed intent and is instructed to treat that intent as its only task (`agent/worker.go:296-318`). Worker graph context is framed as awareness rather than permission to widen the assigned work (`agent/worker.go:320-324` and following prompt text).

The engine constructs per-task planner, worker, and main-agent instances and injects the finding recorder into all three (`server/task_llm.go:552-600`). Task execution uses a shared `runIntent` path for pool workers and detached human-message handling; task-operation admission and final writes are part of the execution lifecycle (`server/engine.go`, `runIntent`).

These are agent orchestration and prompt-level work boundaries. They should not be represented as operating-system sandboxing or as a complete restriction on what an available command/network tool can do.

### Tool catalog and extension points

Built-in tool definitions are assembled in code. The database-backed catalog can filter catalogued domain tools by agent and override descriptions, schemas, and defaults, but does not replace their code-bound handlers. The SDK's `actool.DefaultTools()` are appended separately in the main-agent path, so they are not governed by those same catalog selections (`agent/toolcatalog.go`; `agent/mainagent.go:125-130`).

Agent assembly also supports skills and MCP augmentation and deferred/unlock metadata (`agent/assembly.go`; server visibility is integrated in `server/assembly.go`). This broadens the effective tool surface and means a defensive allowlist must account for built-ins, defaults, skills, MCP tools, and any other runtime-added handlers—not just the database catalog.

### Guard and intercept controls

`guard.Guard` registers `PreToolUse` and `PostToolUse` handlers and exposes them through `Hooks()` (`guard/guard.go:30-55`). On pre-tool use, it records an audit entry and then calls `applyIntercept` (`guard/guard.go:57-79`). The implementation documents two important limits directly: the RoE authorization-scope mechanism was removed, and destructive/exfiltration gating is no longer hard-coded; it relies on ordinary database intercept rules that users can disable or delete (`guard/guard.go:1-7`).

When an interceptor is present and the tool is enabled, `applyIntercept` evaluates rules, then optionally invokes an LLM fallback judge when none match. If no judge decision is available, it allows the call (`guard/guard.go:81-100`). A matching decision may allow, deny, or ask for human approval (`guard/guard.go:101-132`). The interceptor evaluates enabled rules in priority order against the tool name or serialized input (`intercept/intercept.go:518-561`).

The `ask` action creates a pending approval and blocks until a decision, cancellation, or configured timeout. A background task with conversation ID zero can still create an approval visible in the approvals workflow (`intercept/intercept.go:589-647`). The timeout action is configurable; the reviewed default ask timeout is 300 seconds with timeout action `deny`. The judge has a configured timeout (default 15 seconds), while model failure follows the configured fail action, whose reviewed default is `allow` (`intercept/intercept.go:312-359`, `439-505`). Therefore, these defaults do not provide a uniformly fail-closed policy.

Hook coverage depends on construction and attachment. The interactive `ChatAgent` attaches `c.guard.Hooks()` when a Guard is set (`agent/chat.go:73-75`, `157-165`), and server setup calls `SetGuard` for chat agents (`server/server.go:585`, `726`). In contrast, the inspected `MainAgent.Chat` builds session options with `permission.ModeBypass` and does not attach Guard hooks in that function (`agent/mainagent.go:112-166`). This is a source-level observation about that path, not a claim about behavior supplied by the external Norma dependency or about every MainAgent execution path.

### Task constraints and asset rules

`constraintBlock` loads task constraints and appends them to planner/worker system prompts as high-priority allow/deny language (`agent/constraints.go:9-49`). The reviewed function formats text for the model; it does not evaluate a proposed tool call or enforce a command/network boundary. The main-agent prompt likewise describes constraints as prompt injection for the next planning round (`agent/mainagent.go:94-97`).

Asset gating is more concrete but narrower. `db.EvaluateAssetGate` denies a matching enabled block rule, denies assets outside a configured enabled allowlist, and otherwise allows (`db/asset_intercept_match.go:178-195`). `CheckAssetsIntercept` combines global block rules with task block rules and task allow rules (`db/asset_intercept_match.go:214-249`).

The asset-insertion tool extracts domain/IP/URL candidates and applies the same gate before upserting an asset; denied entries skip the upsert and following side effects (`agent/tools_insert.go:241-250`). However, that path ignores errors when loading global and task rule sets (`agent/tools_insert.go:230-239`), so its own comments and control flow show a fail-open behavior for this insertion path. Separately, the `addOneIntent` path checks asset anchors and refuses dispatch when asset checks report hits (`agent/tools.go`, `addOneIntent`). These paths do not establish that all tool calls, Bash commands, WebFetch requests, or MCP calls are constrained to authorized evidence sources.

### Findings and evidence persistence

The finding workflow supports an independent finding record and an exploration-graph node. `RecordedFinding` returns both `FindingID` and `NodeID` (`db/finding_traffic.go:328-338`). `RecordFindingTx` locks task evidence, verifies that a task belongs to the expected exploration and that an intent belongs to that exploration, creates the finding node and finding record, writes an event, and adds prepared traffic bindings in a transaction (`db/finding_traffic.go:342-407`). The evidence store wraps preparation and recording in `WithEvidenceTx` (`evidence/store.go:183-196`).

Traffic capture is a useful enrichment, not a prerequisite for retaining a finding: tool guidance makes `traffic_refs` optional where no exact record is available and explicitly says not to guess IDs or repeat probes merely to create a binding (`agent/tools.go:1224-1226`). The workflow encourages traceable evidence while permitting non-HTTP evidence or an explanation when traffic was not captured. It does not, by itself, establish forensic acquisition provenance, chain of custody, or immutability of source evidence.

### Goal and completion semantics

`prove_goal` validates that the goal is local to the current task and that the evidence node is a fact or finding, then links evidence and marks the goal met (`agent/tools.go:1157-1184`). It checks whether all task goals are met and can set `GoalMet` (`agent/tools.go:1185-1202`). The separate `goal_met` tool sets `GoalMet` directly with a reason and does not perform the same per-goal evidence linkage (`agent/tools.go:1205-1214`). The planner prompt describes quantitative checks before `prove_goal` and says the final goal triggers completion (`agent/planner.go:315-316`), but the existence of a direct completion tool means the prompt is not the only route represented in code.

## Defensive-transformation gap assessment

| Area | Verified current state | Defensive implication |
| --- | --- | --- |
| Product purpose and prompts | Development docs say planned DFIR workflows/safety boundaries are not implemented; planner/main-agent prompt strings retain authorized pentest and exploration/exploitation framing. | Do not describe the current agent experience as DFIR-only. Replace editable defaults and built-in fallback prompts, then test effective rendered prompts. |
| Tool authorization | Guard relies on user-editable intercept rules; no inspected universal forensic tool allowlist. Some agent sessions explicitly use bypass permission mode. | A default-deny capability policy must be enforced at a common dispatch boundary and cover dynamically augmented tools. |
| Approval/failure behavior | Ask exists; judge and ask timeout/failure outcomes are configurable, with reviewed defaults including allow-on-judge-failure. | Define one explicit, auditable failure policy for each operation class; verify it cannot be weakened by ordinary rule editing. |
| Scope enforcement | Task constraints are prompt text. Asset gating is limited to selected asset-aware writes/intent paths, and one insertion path ignores rule-load errors. | Enforce authorization on actual actions and destinations, not only on descriptions, asset records, or model instructions. Fail closed on policy lookup errors. |
| Execution isolation | Agent work directories and output spill directories are configured (`agent/worker.go:257-286`), but those paths alone do not demonstrate process/network isolation. | Specify and test an execution boundary for parsers and external forensic utilities, including filesystem, network, and resource limits. |
| Evidence handling | Findings and optional traffic references are transactionally linked; evidence workflow contains integrity-aware transactional steps. | Extend this model with case/evidence manifests, acquisition provenance, tool/version/run lineage, and preservation controls identified in the roadmap. |
| Extensions | Skills, MCP augmentation, and SDK defaults exist in addition to code-bound tools. | Inventory and explicitly authorize every source of tools at runtime; ensure disabling a catalog entry cannot leave an equivalent capability reachable elsewhere. |
| Completion claims | `prove_goal` links a finding/fact, while `goal_met` can directly complete the task. | For forensic workflows, define evidence-backed completion criteria and test all completion entry points against them. |
| Dependency/runtime semantics | Norma v0.4.3 behavior was not verified in this environment. | Validate hook coverage, permission bypass semantics, cancellation, and tool registration in a controlled test harness before relying on source-level assumptions. |

## Recommendations

These are recommendations, not statements of already implemented behavior.

### P0 — before advertising a defensive-only mode

1. **Inventory all callable capabilities.** Build a complete role-by-role inventory covering domain tools, SDK defaults, Bash/shell, WebFetch/search, skills, MCP, deferred tools, and trigger/background execution. Treat dynamically added tools as part of the same authorization surface.
2. **Create a code-enforced default-deny policy boundary.** Apply an explicit forensic allowlist at the last common point before side effects. Bind decisions to the active case, evidence roots, operation, and destination. Do not make the safety property depend on editable prompt text or deletable ordinary intercept rules.
3. **Resolve bypass and hook coverage.** Verify the exact Norma v0.4.3 behavior in a controlled test and ensure every agent construction path—including MainAgent, workers, planners, scheduled/triggered runs, and chat—has the intended policy hook or is deliberately incapable of side effects.
4. **Make policy errors fail closed.** In asset and tool paths, distinguish “no rule configured” from “rule could not be loaded.” Do not silently continue after a failed authorization/policy lookup.
5. **Define and test approval semantics.** Set explicit behavior for judge failure, missing reviewer, timeout, cancellation, and background approvals. Prevent configuration from silently turning a required approval into allow.
6. **Replace offensive agent defaults.** Rewrite both DB-seeded and built-in fallback prompts, tool descriptions, templates, and completion language to match a clearly bounded analyst-first workflow. Prompt replacement is necessary for product coherence but is not a substitute for P0 enforcement.

### P1 — evidence integrity and reproducibility

1. Define a case identity and evidence manifest with source digest, original name, acquisition context/time, custodian, and permitted analysis roots.
2. Record tool identity/version, command/configuration, input/output digests, timestamps, and parent-child relationships for every analysis run.
3. Make evidence sources read-only to analysis workers and direct derived output to an explicit per-case workspace. Verify isolation at the process/container and network layers rather than inferring it from working-directory configuration.
4. Preserve observation, interpretation, and hypothesis as distinct record types and require citations to exact evidence locations for report claims.
5. Review whether the mutable evidence DB, traffic store, and output files meet the intended retention, access-control, and integrity requirements.

### P2 — validation and migration

1. Add negative tests showing prohibited capabilities cannot be invoked through any role, SDK default, MCP/skill extension, trigger, or alternate agent path.
2. Add tests for allowlist boundaries, policy database failure, approval timeout/failure, cancellation races, and cross-task asset/evidence references.
3. Test packaged/development deployments, clean installation, and effective runtime tool inventories; document differences between them.
4. Gate the defensive-only product claim on the roadmap's Phase 1 exit criterion and repeatable evidence, not on branding or prompt updates alone.

## Unresolved questions

1. **Norma hook semantics:** Does Norma v0.4.3 invoke the registered hook before every tool side effect, and can `PermissionModeBypass` skip or alter those checks? This requires dependency-source inspection and/or a controlled executable test.
2. **Main-agent policy coverage:** Is a Guard attached through any path outside the inspected `MainAgent.Chat` function? The inspected function itself does not attach one; server setup evidence covers ChatAgent guard wiring, not this specific MainAgent path.
3. **Effective tool universe:** What tools are registered for every role after DB configuration, SDK defaults, skills, MCP, and deferred-tool unlocks are combined? The reviewed code identifies multiple sources but does not provide a single runtime inventory.
4. **Policy lifecycle:** Which users/roles can disable or delete built-in intercept rules, alter judge settings, or change task allow/block rules in a deployed instance? The Guard comments establish that the rules are user-disableable/deletable, but the full administrative authorization model was not audited here.
5. **Execution boundary:** What filesystem and network access do Bash, external tools, MCP servers, and analysis workers actually receive in each deployment? Working directories and proxy configuration are not sufficient evidence of isolation.
6. **Forensic provenance:** Which existing evidence records are immutable or cryptographically verifiable across acquisition, processing, export, and report generation? The reviewed finding/traffic transaction establishes relational persistence, not complete chain of custody.
7. **Completion enforcement:** Can callers invoke `goal_met` without satisfying per-goal evidence checks in every execution mode, and what user-facing consequences follow? The tool implementation directly marks the task complete; runtime authorization and UX effects need testing.
8. **Test status:** Which existing automated tests currently exercise these policy and evidence paths in a configured environment? This audit did not run Go tests because Go was unavailable.

## Source index

- `docs/TRACE_DEVELOPMENT.md:47-55` — current TRACE development-scope note.
- `docs/TRACE_ROADMAP.md:16-24` — incomplete Phase 1 tasks and exit criterion.
- `agent/mainagent.go:84-101,112-166` — main-agent prompt and session construction.
- `agent/planner.go:295-345` — built-in planner prompt and its rendering.
- `agent/worker.go:257-293,296-324` — artifact directory rules and worker task framing.
- `agent/constraints.go:9-49` — task-constraint prompt rendering.
- `agent/chat.go:73-75,157-165` — ChatAgent Guard setter and hook attachment.
- `guard/guard.go:1-7,30-55,57-132` — Guard scope, hook registry, and intercept decision flow.
- `intercept/intercept.go:312-359,439-505,518-561,589-674` — judge configuration, fallback, matching, and approval wait behavior.
- `db/asset_intercept_match.go:178-249` — asset gate and asset checks.
- `agent/tools_insert.go:230-250` — asset-rule loading and insertion gate.
- `agent/tools.go:addOneIntent` — intent anchor validation and asset-intercept check.
- `agent/tools.go:1157-1214` — goal proof and direct task completion tools.
- `agent/tools.go:1219-1227` — finding-report schema and traffic-reference guidance.
- `agent/toolcatalog.go` and `agent/assembly.go` — tool catalog and runtime augmentation.
- `server/assembly.go` — server agent-visibility integration.
- `server/task_llm.go:552-600` — construction of per-task agents and finding-recorder injection.
- `server/server.go:585,726` — server chat Guard wiring.
- `evidence/store.go:183-196` — transactional finding preparation and recording.
- `db/finding_traffic.go:328-407` — finding identity and transactional persistence/validation.
- `go.mod` — Norma dependency version (`github.com/Autumn-27/norma v0.4.3`).
