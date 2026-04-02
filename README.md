# Town Crier

Town Crier is a mobile-first app for monitoring UK local authority planning applications. It delivers push notifications to residents, community groups, and property professionals when new planning applications appear in their area.

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend API | .NET 10, ASP.NET Core, Native AOT |
| Web Frontend | React 19, TypeScript, Vite |
| Database | Azure Cosmos DB (Serverless) |
| iOS App | Swift, SwiftUI, SwiftData |
| Infrastructure | Pulumi (C# / .NET 10), Azure Container Apps |
| CI/CD | GitHub Actions |
| Testing | TUnit (.NET), Vitest (Web), XCTest (iOS) |

## Repository Structure

```plaintext
/api          — .NET backend (Hexagonal Architecture / Ports & Adapters)
/web          — React frontend (Vite, Leaflet maps, Auth0)
/mobile/ios   — Native iOS app (MVVM-C)
/infra        — Pulumi Infrastructure as Code
/docs/adr     — Architecture Decision Records
```

## Getting Started

### Prerequisites

- [.NET 10 SDK](https://dotnet.microsoft.com/download)
- [Node.js](https://nodejs.org/) (for web development)
- [Xcode](https://developer.apple.com/xcode/) (for iOS development)
- [Docker](https://www.docker.com/) (for running integration tests)

### API

```bash
cd api
dotnet build
dotnet test
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

Town Crier follows a **hexagonal architecture** (ports and adapters) on the backend with **CQRS** for command/query separation and **Domain-Driven Design** with rich domain models. The web frontend uses **React** with **Leaflet** for interactive maps, **Auth0** for authentication, and **React Query** for server state. The iOS app uses **MVVM-C** (Model-View-ViewModel-Coordinator) with Swift Concurrency.

Data is ingested from [PlanIt](https://www.planit.org.uk/) via a polling-based model. See the [Architecture Decision Records](docs/adr/) for detailed design rationale.

## License

See [LICENSE](LICENSE) for details.
