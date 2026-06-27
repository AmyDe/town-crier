# Town Crier

Town Crier is a mobile-first app for monitoring UK local authority planning applications. It delivers push notifications to residents, community groups, and property professionals when new planning applications appear in their area.

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend API | Go (`net/http`, `log/slog`), Azure Container Apps |
| Web Frontend | React 19, TypeScript, Vite |
| Database | Azure Database for PostgreSQL Flexible Server + PostGIS |
| iOS App | Swift, SwiftUI, SwiftData |
| Infrastructure | Pulumi (Go), Azure Container Apps |
| CI/CD | GitHub Actions |
| Testing | go test (Go), Vitest (Web), XCTest / Swift Testing (iOS) |

## Repository Structure

```plaintext
/api-go       — Go backend: HTTP API + background worker
/cli          — Go admin CLI (`tc`)
/web          — React frontend (Vite, Leaflet maps, Auth0)
/mobile/ios   — Native iOS app (MVVM-C)
/infra        — Pulumi Infrastructure as Code (Go)
/docs/adr     — Architecture Decision Records
```

## Getting Started

### Prerequisites

- [Go 1.26](https://go.dev/dl/) (backend, CLI, and infrastructure)
- [Node.js](https://nodejs.org/) (for web development)
- [Xcode](https://developer.apple.com/xcode/) (for iOS development)

### Backend API

```bash
cd api-go
go build ./...
go test ./...
```

### Admin CLI

```bash
cd cli
go build ./...
go test ./...
```

### Web

```bash
cd web
npm install
npm run dev       # Vite dev server with hot reload
npm run build     # Production build
npx vitest run    # Run tests
```

### iOS

```bash
cd mobile/ios
swift build
swift test
```

## Architecture

The backend is a **Go** service — an HTTP API plus a background worker — with a flat, feature-sliced layout under `internal/`, built on the standard library (`net/http`, `log/slog`), the `pgx` driver for Postgres + PostGIS, and the official Azure SDK for Service Bus. The web frontend uses **React** with **Leaflet** for interactive maps, **Auth0** for authentication, and **React Query** for server state. The iOS app uses **MVVM-C** (Model-View-ViewModel-Coordinator) with Swift Concurrency. Infrastructure is defined with **Pulumi** in Go.

Data is ingested from [PlanIt](https://www.planit.org.uk/) via a polling-based model. See the [Architecture Decision Records](docs/adr/) for detailed design rationale.

## License

See [LICENSE](LICENSE) for details.
</content>
</invoke>
