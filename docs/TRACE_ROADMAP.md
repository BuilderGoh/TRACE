# TRACE development roadmap

This roadmap describes intended work, not implemented features.

## Phase 0: Baseline and provenance

- [x] Fork ARTEX Global Edition into `BuilderGoh/TRACE`
- [x] Create `feat/trace-foundation` branch
- [x] Document scope, upstream credits, and current limitations
- [ ] Confirm a clean Docker Compose startup and authentication workflow in a lab
- [ ] Inventory upstream agent prompts, task templates, offensive tools, and permissions
- [ ] Capture a baseline screenshot and test notes

**Exit criterion:** reproducible startup of unchanged ARTEX on the TRACE branch.

## Phase 1: Defensive rebranding and guardrails

- [ ] Update visible application titles and project descriptions without changing import paths
- [ ] Replace default pentesting task templates and prompts with analyst-first DFIR roles
- [ ] Explicitly disable offensive execution paths before advertising defensive-only mode
- [ ] Implement a forensic tool allowlist, isolation boundary, and human approval gates
- [ ] Add tests verifying disabled offensive paths cannot be invoked through agents or MCP

**Exit criterion:** agent conversations and tool calls stay within an authorized evidence-analysis environment.

## Phase 2: Windows DFIR MVP

- [ ] Add a case identity and evidence manifest
- [ ] Record SHA-256 digest, original filename, acquisition context, and tool-run lineage
- [ ] Integrate Hayabusa through a constrained execution adapter
- [ ] Correlate EVTX/Sysmon observations into a timeline with timezone information
- [ ] Generate a report citing exact source events and separating observation from inference

**Exit criterion:** reproducible analysis of a public/synthetic Windows attack dataset.

## Phase 3: Hunting and validation

- [ ] Add Sigma hunting workflow and rule suggestions
- [ ] Add analyst validation for generated queries and detections
- [ ] Measure missed events, incorrect claims, and investigation time against ground truth
- [ ] Publish a redacted end-to-end walkthrough with repeatable commands

## Deferred

Memory and network forensic agents, cloud DFIR, case-system connectors, automatic incident response actions, and commercial forensic tools.

## Engineering constraints

- Prefer existing ARTEX components over replacements.
- Prefer established deterministic forensic utilities over custom parsers.
- Keep the original AGPL-3.0 license and attribution.
- Never store secrets or real client evidence in the repository.
- Do not claim features exist until they have tests or a demonstrated workflow.
