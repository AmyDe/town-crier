# Testing & Integration (reference)

Read when writing any test, hand-writing a fake, or building real-DB integration tests (Postgres + PostGIS / the `pgtest` harness). The core (`SKILL.md`) states the test-double conventions; this file is the full examples and the harness API.

## 5. Testing — stdlib `testing` first, `testify/require` allowed

- **Framework**: stdlib `testing` is the default. `github.com/stretchr/testify/require` and `.../assert` are permitted for assertion ergonomics (`require.NoError(t, err)` is genuinely better than the four-line `if err != nil` form repeated 50 times).
- **Forbidden**: `testify/suite` (fights `t.Cleanup`), `gomock`, `mockery`, any reflection-based mocking library. Hand-written fakes only.
- **Table-driven tests** with subtests are the default shape:
  ```go
  func TestNotification_Validation(t *testing.T) {
      t.Parallel()
      tests := []struct {
          name      string
          authority string
          reference string
          wantErr   bool
      }{
          {"valid", "GLA", "24/0001", false},
          {"missing authority", "", "24/0001", true},
          {"missing reference", "GLA", "", true},
      }
      for _, tc := range tests {
          t.Run(tc.name, func(t *testing.T) {
              t.Parallel()
              _, err := NewNotification(UserID("u1"), tc.authority, tc.reference, time.Now())
              if (err != nil) != tc.wantErr {
                  t.Fatalf("got err=%v, wantErr=%v", err, tc.wantErr)
              }
          })
      }
  }
  ```
- **Hand-written fakes** live in `_test.go` files in the same package. No public test fixtures package.
  ```go
  type fakeNotificationStore struct {
      saved   map[NotificationID]Notification
      saveErr error
  }

  func newFakeNotificationStore() *fakeNotificationStore {
      return &fakeNotificationStore{saved: map[NotificationID]Notification{}}
  }

  func (f *fakeNotificationStore) Save(ctx context.Context, n Notification) error {
      if f.saveErr != nil {
          return f.saveErr
      }
      f.saved[n.ID] = n
      return nil
  }

  func (f *fakeNotificationStore) Get(ctx context.Context, id NotificationID) (Notification, error) {
      n, ok := f.saved[id]
      if !ok {
          return Notification{}, ErrNotFound
      }
      return n, nil
  }
  ```
- **HTTP integration tests** use `httptest.NewServer` with `http.HandlerFunc`. Outbound client tests assert against a captured `*http.Request`.
- **Integration tests live in the package they exercise**, behind a build tag (`//go:build integration` at the top of `integration_test.go`), run with `go test -tags=integration ./...`. The top-level `tests/e2e/` directory is reserved for black-box docker-compose tests that drive the compiled binary over HTTP — nothing else goes there.
- **Real-database store tests (Postgres + PostGIS).** The Cosmos → Postgres migration (memo 0010, epic #645) adds a real-DB test layer for store ports. Local Postgres runs in Docker (`api-go/docker-compose.yml`). The `internal/platform/postgres/pgtest` package (itself `//go:build integration`) exposes `New(t) *pgxpool.Pool` — connects via `TEST_DATABASE_URL` or the compose default, applies the embedded goose migrations, and **`t.Skip`s with a hint when no DB is reachable** — plus `Truncate(t, pool, tables...)` for per-test isolation. Put store tests behind `//go:build integration` in the package they exercise, seed deterministic fixtures, and assert exact results. Run with `make -C api-go test-integration` (boots the DB) or `go test -tags=integration ./...` against a running DB. **Additive, not a replacement:** handler/logic tests keep using hand-written fakes; the real-DB layer exists to cover spatial/SQL behaviour fakes cannot honestly model (`ST_DWithin`, KNN `ORDER BY location <-> point`, accurate `COUNT`). A Postgres store port that only passes the untagged `go test ./...` suite is not done.
- **No builder pattern.** Go has struct literals and small helper constructors. `notif := Notification{ID: "n1", ...}` or `notif := newTestNotification(t)`. Builders add ceremony Go does not need.
- **`t.Parallel()`** on every test that doesn't share global state. Catches data races and keeps the suite fast.
- **`t.Helper()`** in helper functions so failures point at the caller.
- **`t.Cleanup()`** for teardown instead of `defer` when the cleanup is a fixture concern.
