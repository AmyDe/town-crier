# Town Crier

Town Crier is a mobile-first app for monitoring UK local authority planning applications. It delivers push notifications to residents, community groups, and property professionals when new planning applications appear in their area.

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend API | .NET 10, ASP.NET Core, Native AOT |
| Database | Azure Cosmos DB (Serverless) |
| iOS App | Swift, SwiftUI, SwiftData |
| Infrastructure | Pulumi (C# / .NET 10), Azure Container Apps |
| CI/CD | GitHub Actions |
| Testing | TUnit (.NET), XCTest (iOS) |

## Repository Structure

```plaintext
/api          — .NET backend (Hexagonal Architecture / Ports & Adapters)
/mobile/ios   — Native iOS app (MVVM-C)
/infra        — Pulumi Infrastructure as Code
/docs/adr     — Architecture Decision Records
```

## Getting Started

### Prerequisites

- [.NET 10 SDK](https://dotnet.microsoft.com/download)
- [Xcode](https://developer.apple.com/xcode/) (for iOS development)
- [Docker](https://www.docker.com/) (for running integration tests)

### API

```bash
cd api
dotnet build
dotnet test
```

### iOS

```bash
cd mobile/ios
swift build
swift test
```

## Architecture

Town Crier follows a **hexagonal architecture** (ports and adapters) on the backend with **CQRS** for command/query separation and **Domain-Driven Design** with rich domain models. The iOS app uses **MVVM-C** (Model-View-ViewModel-Coordinator) with Swift Concurrency.

Data is ingested from [PlanIt](https://www.planit.org.uk/) via a polling-based model. See the [Architecture Decision Records](docs/adr/) for detailed design rationale.

## License

See [LICENSE](LICENSE) for details.
