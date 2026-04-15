# tc — Town Crier Admin CLI

Date: 2026-04-15

## Purpose

A lightweight CLI tool for interacting with Town Crier admin API endpoints. Provides a quick, scriptable way to perform admin operations (subscription grants, one-time codes, etc.) without needing curl commands or a web UI.

Expected scope: 10-20 flat commands covering admin writes and operational read queries.

## Constraints

- .NET 10, Native AOT compiled. No reflection, no dynamic code generation.
- All JSON serialization via `System.Text.Json` with `[JsonSerializable]` source generators.
- Lives at `/cli` as a peer to `/api`. Own solution, own types. No shared project references.
- No dependency injection container, no `IHost` builder.

## Repository Layout

```
/cli
  tc.sln
  src/
    tc/
      tc.csproj
      Program.cs              # Entry point, arg parsing, command dispatch
      Config.cs               # Config file loading + CLI arg merging
      ApiClient.cs            # HttpClient wrapper (base URL, X-Admin-Key header)
      Commands/
        GrantSubscriptionCommand.cs
      Json/
        TcJsonContext.cs      # System.Text.Json source generator context
  .editorconfig
```

## Configuration

**File location:** `~/.config/tc/config.json`

```json
{
  "url": "https://api.towncrierapp.uk",
  "apiKey": "sk-..."
}
```

**Precedence:** CLI args (`--url`, `--api-key`) override config file values. These are global args available on every command.

**Behaviour:**
- Config file is loaded if it exists. Missing file is not an error as long as CLI args supply the values.
- If neither source provides `url` or `apiKey`, print an error with setup instructions and exit with code 1.
- Deserialized with source-generated `System.Text.Json` — no reflection.
- No `tc config` management subcommand — users create the file manually.

## Command Parsing & Dispatch

Manual arg parsing — no external library.

- `args[0]` is the command name.
- Remaining args parsed as `--key value` pairs into a dictionary.
- Each command validates its required args and prints usage if missing.
- Unknown command prints the help summary.

**Built-in commands:**
- `tc help` (also `--help`, `-h`, no args) — lists all commands with one-line descriptions.
- `tc version` — prints version string.

**Exit codes:**
- `0` — success
- `1` — user error (bad args, missing config)
- `2` — API error (non-success HTTP response)

**Output:**
- Success: human-readable result to stdout (e.g. `Subscription granted: foo@bar.com -> Pro`).
- Errors: human-readable message to stderr, including HTTP status when applicable.
- No JSON output mode.

## API Client

Thin `HttpClient` wrapper.

- Sets `X-Admin-Key` header on every request.
- Base URL from resolved config.
- Methods return raw `HttpResponseMessage` — commands handle their own deserialization.
- No retry logic or resilience policies — human-operated tool, re-run on failure.
- Default 30-second timeout.

## Initial Command: grant-subscription

```
tc grant-subscription --email foo@bar.com --tier Pro
```

**Required args:**
- `--email` — user's email address.
- `--tier` — subscription tier: `Free`, `Personal`, or `Pro` (case-insensitive).

**Behaviour:**
- Validates tier against known values.
- Sends `PUT /v1/admin/subscriptions` with body `{ "email": "...", "subscriptionTier": "..." }`.
- On success: `Subscription granted: foo@bar.com -> Pro`
- On 404: `User not found: foo@bar.com`
- On other errors: prints status code and response body to stderr.

## Adding Future Commands

Each new command requires:
1. A new file in `Commands/` with a static `RunAsync` method.
2. A new case in the `switch` in `Program.cs`.
3. Any new DTOs added to `TcJsonContext`.

No registration mechanism, interfaces, or base classes.

## Out of Scope

- Shared project references with `/api`.
- Dependency injection or host builder.
- JSON output mode.
- Config management subcommand.
- Retry/resilience logic.
- Authentication via user JWT — admin key only.
