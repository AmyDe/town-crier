# Security & Logging (reference)

Read when the bead touches credentials/secrets, tokens, HMAC, TLS config, random IDs/nonces, Auth0 JWT validation, or structured logging. The core (`SKILL.md`) states the rules; this file is the full detail.

## 8. Logging — `log/slog` only

- **`log/slog` is the only logger.** No `zap`, no `zerolog`, no `logrus`, no stdlib `log`.
- **JSON handler in production**, text handler permitted only in local dev:
  ```go
  logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
  ```
- **Pass the logger explicitly** through constructors. Do not use `slog.Default()` in library code; only `main()` may set it.
- **Key/value pairs, typed**: `logger.Info("notification dispatched", "id", n.ID, "user", n.UserID)`. Linted via `sloglint`.
- **Never log secrets or PII**: use the `SecretString` value type for credentials (see §10) and redact email/phone fields in structured logs.

## 10. Security primitives

- **`SecretString` value type** for any credential, redacting in `String()` and `MarshalJSON`:
  ```go
  type SecretString struct{ value string }
  func NewSecret(v string) SecretString             { return SecretString{value: v} }
  func (s SecretString) String() string             { return "[REDACTED]" }
  func (s SecretString) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
  func (s SecretString) LogValue() slog.Value       { return slog.StringValue("[REDACTED]") }
  func (s SecretString) Expose() string             { return s.value }
  ```
  The `slog.LogValuer` implementation matters: without it, `logger.Info("x", "key", cfg.CosmosKey)` may bypass `String()` redaction depending on the handler.
  Use for Auth0 client secrets, APNs auth keys, Service Bus connection strings, Cosmos primary keys. Pass `SecretString` through config; call `.Expose()` only at the boundary where the credential leaves the process.
- **`crypto/subtle.ConstantTimeCompare`** for HMAC/token equality. Never `==` on a secret.
- **`crypto/rand`** for IDs, nonces, tokens. Never `math/rand` for anything security-sensitive.
- **TLS 1.2 minimum** on any custom `tls.Config`: `MinVersion: tls.VersionTLS12`. Never `InsecureSkipVerify: true` in non-test code.
- **Auth0 JWT validation** via `github.com/auth0/go-jwt-middleware/v3`. Always validate `iss`, `aud`, `exp`. Cache JWKS with a TTL.
- **No CSRF** for native iOS clients on `Authorization: Bearer`. CSRF applies only to cookie-session browser POSTs. Don't add ceremony you don't need.
