# tc — Town Crier admin CLI

A small Go CLI for Town Crier admin operations against the API's `/v1/admin/*`
endpoints. Authenticates with an `X-Admin-Key` header.

## Build

```bash
cd cli
go build -o tc ./cmd/tc
```

A static binary is produced with `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tc ./cmd/tc`.

## Configuration

Settings are read from `~/.config/tc/config.json`:

```json
{ "url": "https://api.towncrierapp.uk", "apiKey": "sk-..." }
```

`--url` and `--api-key` override the file. Either source must supply both values.

## Commands

```
tc generate-offer-codes --count <N> --tier <Personal|Pro> --duration-days <D> --label <label> [--max-redemptions <M>]
tc list-offer-codes     [--label <term>]
tc grant-subscription   --email <email> --tier <Free|Personal|Pro>
tc list-users           [--search <term>] [--page-size <n>]
tc help
tc version
```

`generate-offer-codes` mints a batch of codes that all share one admin-facing
`--label` (required) and one redemption cap `--max-redemptions` (1-10,000,
default 1 — a single-use code is just the default). `list-offer-codes` renders
the code listing (label, tier, duration, redeemed x/N, created, last redeemed)
from `GET /v1/admin/offer-codes`, optionally filtered by a case-insensitive
`--label` substring; there is no pagination beyond the API's own limit.

Run `tc help` for full usage. Exit codes: `0` success, `1` usage/validation/config
error, `2` API/runtime error.

## Develop

```bash
cd cli
gofmt -l .
go vet ./...
go test ./...
```
