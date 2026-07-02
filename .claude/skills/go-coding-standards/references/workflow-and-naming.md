# Workflow, Lint & Naming (reference)

Read when running build/test/lint/format commands, bootstrapping `.golangci.yml`, naming a package/file/interface/error, or assembling the pre-commit checklist. The core (`SKILL.md`) carries the forbidden list; this file is the operational detail.

## Workflow

### Build

```bash
cd api-go && go build ./...
```

### Test

```bash
cd api-go && go test ./...                  # all tests
cd api-go && go test -race ./...            # with race detector (required before merge)
cd api-go && go test -run TestName ./...    # single test
```

### Format

```bash
cd api-go && gofmt -w .                     # idiomatic formatting
cd api-go && go vet ./...                   # standard correctness checks
```

### Lint

```bash
cd api-go && golangci-lint run ./...
```

The lint config lives in `api-go/.golangci.yml`. Bootstrap it from the bundled asset:

```bash
cp .claude/skills/go-coding-standards/assets/.golangci.yml api-go/.golangci.yml
```

The baseline enables `errcheck`, `govet`, `staticcheck`, `gosec`, `sloglint`, `bodyclose`, `contextcheck`, `errorlint`, `noctx`, `rowserrcheck`, `sqlclosecheck`, `copyloopvar`, `intrange`, `misspell`, `unparam`, `unconvert`. Style-opinion linters (`funlen`, `cyclop`, `wsl`, `gofumpt`) are deliberately disabled — they fight AI agents without catching real bugs.

## Naming conventions

- **Packages**: short, lowercase, single word where possible (`notifications`, `planit`, `apns`). No underscores, no camelCase, no plurals where a singular reads naturally.
- **Files**: lowercase with underscores (`store_cosmos.go`, `handler_test.go`).
- **Exported identifiers**: PascalCase. **Unexported**: camelCase.
- **Interfaces**: noun or `-er` suffix (`Notifier`, `Validator`, `Store`). No `I` prefix.
- **Receivers**: short — one or two letters matching the type (`func (n *Notification) ...`). Never `this`/`self`.
- **Accessors**: no `Get` prefix — `n.Name()`, not `n.GetName()`.
- **Data access**: `Store`, not `Repository` (`CosmosStore`, `store_cosmos.go`).
- **Error variables**: `Err` prefix (`ErrNotFound`, `ErrAlreadyClaimed`).
- **Error strings**: lowercase, no trailing punctuation — `errors.New("authority is required")`, never `"Authority is required."` (staticcheck ST1005).
- **Test functions**: `TestSubject_Behaviour` (e.g. `TestNotification_RejectsEmptyAuthority`).
- **Constants**: PascalCase if exported, camelCase if not. No `SHOUTY_CASE`.

## Pre-commit checklist (run before every PR)

```bash
cd api-go && \
  gofmt -l . | tee /dev/stderr | wc -l | xargs -I{} test {} = 0 && \
  go vet ./... && \
  golangci-lint run ./... && \
  go test -race ./...
```

A single failing step blocks the PR.
