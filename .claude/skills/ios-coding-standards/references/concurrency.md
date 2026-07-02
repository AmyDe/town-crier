# Concurrency & State (reference)

Read when writing async code, wrapping a legacy callback API, or annotating ViewModel state with `@MainActor`. The core (`SKILL.md`) states the rule; this file is the full detail and rationale.

## Concurrency & State

Swift Concurrency provides structured, compiler-checked async code. Using `DispatchQueue` or `Combine` for simple async work bypasses these checks and makes data races harder to catch.

- **Pattern:** Swift Concurrency (`async`/`await`) exclusively.
- **No `DispatchQueue.main.async`** unless wrapping a legacy API that has no async alternative.
- **No completion handlers** for async logic — use `async throws` instead.
- **No `Combine`** for one-off async tasks. `Combine` is appropriate for reactive streams (e.g., observing a `@Published` property), not for request/response patterns.
- **`@MainActor`:** All UI-bound state in ViewModels must be annotated with `@MainActor`. This is enforced at compile time and eliminates a class of threading bugs.
