# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Town Crier is a mobile-first app for monitoring UK local authority planning applications, delivering push notifications to residents, community groups, and property professionals. It uses PlanIt (planit.org.uk) as its primary data provider, with a polling-based ingestion model (see ADR 0006).

## Monorepo Structure

```
/api          # .NET 10 backend (Hexagonal / Ports & Adapters)
/mobile/ios   # Native iOS app (Clean Architecture / MVVM-C)
/infra        # Pulumi IaC (.NET 10 / C#)
/docs/adr     # Architecture Decision Records
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend API | .NET 10 (ASP.NET Core), Native AOT, Azure Container Apps |
| Database | Azure Cosmos DB (Serverless) via Cosmos DB SDK (no ORM) |
| iOS App | Swift, SwiftUI, SwiftData, SPM |
| Infrastructure | Pulumi (C#/.NET 10) |
| CI/CD | GitHub Actions |
| Testing | TUnit (.NET), XCTest (iOS) |

## Development Commands

### .NET API (`/api`)

```bash
dotnet build                        # Build
dotnet test                         # Run all tests
dotnet test --filter "TestName"     # Run a single test
dotnet format --verify-no-changes   # Check formatting
dotnet format                       # Auto-fix formatting
```

### iOS (`/mobile/ios`)

```bash
swift build                         # Build
swift test                          # Run all tests
swiftlint lint --strict             # Lint
swift-format format --in-place --recursive .  # Auto-format
```

### Web (`/web`)

```bash
cd web && npm run dev               # Vite dev server with hot reload
cd web && npm run build             # Production build to /web/dist
cd web && npx tsc --noEmit          # Type check without emitting
cd web && npx vitest run            # Run tests (when Vitest is added)
```

## Coding Standards Skills

This repo has Claude Code skills that are **auto-triggered** when writing code:

- `dotnet-coding-standards` — Auto-invoked when writing, reviewing, or scaffolding any C# code in `/api`. Covers DDD, CQRS, TUnit testing, Cosmos DB SDK, and Native AOT patterns.
- `ios-coding-standards` — Auto-invoked when writing, reviewing, or scaffolding any Swift code in `/mobile/ios`. Covers MVVM-C, protocol-oriented design, XCTest, and Swift Concurrency.
- `react-coding-standards` — Auto-invoked when writing, reviewing, or scaffolding any React/TypeScript code in `/web`. Covers feature-sliced clean architecture, CSS Modules with design tokens, hooks-as-ViewModels, Vitest + Testing Library, and domain-layer purity.
- `design-language` — Auto-invoked when creating or modifying any UI code, colors, themes, or visual components. Defines the cross-platform design system (colors, typography, spacing, components) with light, dark, and OLED dark theme support.

Skills are defined in `.claude/skills/` as `SKILL.md` files.

## Key Architectural Constraints

### Native AOT (.NET)
All .NET code must be Native AOT-compatible:
- No reflection (`typeof(T).GetProperties()`, `Activator.CreateInstance`)
- System.Text.Json with `[JsonSerializable]` source generators — no Newtonsoft.Json
- No dynamic assembly loading or runtime code generation
- No EF Core or Dapper — use Cosmos DB SDK directly

### CQRS (Manual Dispatch)
Command/Query handlers are manually dispatched — no MediatR or similar libraries. Keep the dependency tree minimal.

### DDD (Rich Domain Models)
Business logic lives in domain entities and value objects, not in handlers. Handlers are lightweight orchestrators only.

### Testing
- TDD workflow: Red-Green-Refactor
- Handlers are the primary unit of test (.NET); ViewModels and Use Cases for iOS
- Builder pattern for test data (.NET); static extension fixtures for iOS
- Manual fakes/spies — no reflection-based mocking libraries

## Naming Conventions

- **Directory/project names:** `town-crier.*` (lowercase, hyphenated)
- **C# namespaces:** `TownCrier.*` (PascalCase)
- **Swift types:** PascalCase; no `I` prefix on protocols (use `...able`, `...Service`, `...Repository`)
- **C# classes:** `sealed` by default; Swift classes: `final` by default

## Issue Tracking — Beads (Exclusive)

Use **beads** (`bd`) for ALL task and issue tracking. Do NOT use TodoWrite, TaskCreate, or markdown files for tracking work.

- Run `bd prime` at the start of every session (or after compaction/clear) to load the workflow context
- Create a beads issue **before** writing code: `bd create --title="..." --description="..." --type=task|bug|feature --priority=2`
- Mark issues in-progress when starting: `bd update <id> --status=in_progress`
- Close issues when done: `bd close <id>`
- Check for available work: `bd ready`
- Do NOT use `bd edit` — it opens an interactive editor that blocks agents
- Beads data is stored in a Dolt database at `.beads/dolt/` with a remote pointing to this GitHub repo (stored on `refs/dolt/data`, invisible to normal Git operations)
- Run `bd dolt push` to sync beads data to GitHub — this is separate from `git push`

## Shell Commands — Non-Interactive Mode

Always use non-interactive flags with file operations to avoid hanging on confirmation prompts (some systems alias `cp`, `mv`, `rm` to include `-i`):

```bash
cp -f source dest               # NOT: cp source dest
mv -f source dest               # NOT: mv source dest
rm -f file                      # NOT: rm file
rm -rf directory                # NOT: rm -r directory
```

## Session Completion Checklist

When ending a work session where code was changed:

1. **File issues for remaining work** — `bd create` for anything needing follow-up
2. **Run quality gates** — tests, linters, builds
3. **Update issue status** — `bd close` finished work, update in-progress items
4. **Sync and push** —
   ```bash
   git pull --rebase
   bd dolt push
   git push
   ```
5. **Verify** — `git status` should show "up to date with origin"

## Memory Management

Proactively update Claude Code memory (`~/.claude/projects/.../memory/`) whenever you learn something noteworthy during a session. This includes:

- New architectural decisions or constraints
- User preferences or workflow patterns
- Project context that would be useful in future sessions
- Corrections or feedback from the user

Do not wait to be asked — if it seems worth remembering, save it.

## Architecture Decision Records (ADRs)

Record an ADR in `/docs/adr/` for any major architectural decision. Use the format `NNNN-title.md` (zero-padded sequence number). An ADR should be written whenever:

- A new technology, framework, or library is adopted (or explicitly rejected)
- A structural pattern is chosen (e.g., CQRS, event sourcing, specific auth flow)
- A significant trade-off is made (e.g., performance vs. simplicity, build vs. buy)
- An existing architectural decision is revisited or reversed

Use this template:

```markdown
# NNNN. Title

Date: YYYY-MM-DD

## Status

Proposed | Accepted | Deprecated | Superseded by [NNNN](NNNN-title.md)

## Context

What is the issue that we're seeing that is motivating this decision or change?

## Decision

What is the change that we're proposing and/or doing?

## Consequences

What becomes easier or more difficult to do because of this change?
```

## Style Enforcement Assets

Standard linting/formatting configs are bundled with their respective skills:
- `.claude/skills/dotnet-coding-standards/assets/.editorconfig` and `Directory.Build.props` (SonarAnalyzer, StyleCop, warnings-as-errors)
- `.claude/skills/ios-coding-standards/assets/.swiftlint.yml` (force cast/try/unwrap as errors)
