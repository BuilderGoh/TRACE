<div align="center">

# TRACE
### Threat Reconstruction & Analysis Coordination Engine

**An experimental, open-source agentic DFIR and threat-hunting platform.**

*Built on [ARTEX Global Edition](https://github.com/jiwoochris/artex-global); currently in early development.*

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

</div>

## Project status

> **Foundation stage — not yet a functional DFIR product.** The codebase currently retains ARTEX's original penetration-testing logic, tools, and UI. Do **not** treat a renamed fork as a defensive-only tool. TRACE's investigation workflows and safety boundaries are planned work, not implemented functionality.

TRACE aims to help analysts investigate security incidents using specialized AI agents, deterministic forensic parsers, evidence-backed findings, and reproducible workflows. The aim is to build useful DFIR capabilities **without reinventing** an agent orchestration platform.

## Planned first demonstration

Given a public or synthetic Windows incident dataset containing EVTX/Sysmon telemetry, TRACE should eventually:

1. Ingest case evidence and preserve its provenance and cryptographic hashes.
2. Dispatch a forensic-analysis agent to an isolated, allowlisted tool environment.
3. Analyze security telemetry with established tools (starting with Hayabusa).
4. Correlate observations into a timestamped attack narrative.
5. Produce an analyst-reviewable report where findings cite the original artifact and event.

**All five steps are planned, not yet implemented.**

## Architecture strategy

**Reuse from ARTEX:** Go backend, Next.js frontend, PostgreSQL, task orchestration, multi-agent roles, human-in-the-loop chat, task history, findings, graph infrastructure, LLM providers, and MCP interfaces.

**Adapt for TRACE:** agent prompts, case-centric workflows, forensic tool access, timelines, evidence provenance, hunting/detection outputs, and defensive safety controls.

**Out of scope for now:** rebuilding the agent framework, writing forensic parsers from scratch, commercial forensic product integration, autonomous containment, and active offensive operations.

See [the phased roadmap](docs/TRACE_ROADMAP.md).

For a source-built development environment isolated from the upstream baseline deployment, see [the development guide](docs/TRACE_DEVELOPMENT.md).

## Getting started: original ARTEX baseline

At this stage, the repository is an upstream-derived application, **not yet a configured DFIR release**.

### Prerequisites

- Docker Engine and Docker Compose
- An LLM endpoint, configured as described by upstream ARTEX
- The original environment variables and deployment prerequisites from the [upstream setup guide](https://github.com/jiwoochris/artex-global#quick-start-docker-compose)

```bash
git clone https://github.com/BuilderGoh/TRACE.git
cd TRACE
git checkout feat/trace-foundation
```

Review the upstream configuration, create the required `.env` file **locally**, then use the existing compose stack:

```bash
docker compose up -d
docker compose ps
docker compose logs --tail=100
```

**Do not commit API keys, case evidence, credentials, or client data.** Run baseline tests only in an isolated lab. The inherited application may provide offensive capabilities that are not appropriate for production networks.

The original Go build path remains `./cmd/artex` and the original Go module name remains `github.com/Autumn-27/artex` until migration is tested.

## Portfolio goals

This project is primarily a **digital forensics and incident response portfolio project**, rather than a novel software-engineering framework. Quality will be demonstrated through realistic case walkthroughs, reproducible artifacts, evidence-backed conclusions, and measurable investigation outcomes.

## Credits, license, and provenance

TRACE is a derivative of **[ARTEX Global Edition](https://github.com/jiwoochris/artex-global)**, itself based on **[Autumn-27/ARTEX](https://github.com/Autumn-27/ARTEX)**. The initial implementation, including its orchestration engine and frontend, originates with those projects. TRACE-specific modifications will be identified in subsequent commits. Review the upstream README for original documentation and safety notices.

The repository retains its **GNU Affero General Public License v3.0 (AGPL-3.0)**. See [LICENSE](LICENSE). Preserve upstream notices and comply with the license when modifying or deploying TRACE.

## Responsible use

TRACE development is limited to data and infrastructure the developer owns or is explicitly authorized to analyze. Prefer public and synthetic datasets. Never upload confidential investigations or sensitive evidence to external LLM services without authorization. Any future state-changing response actions will require explicit, enforced approval and appropriate safeguards.
