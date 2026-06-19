# 0001. Initial Tech Stack Decision

Date: 2026-03-16

## Status
Accepted — the backend (item 2) and IaC (item 7) technology choices have since been reversed by [ADR 0028](0028-migrate-backend-from-dotnet-to-go.md) and [ADR 0029](0029-migrate-infrastructure-from-dotnet-to-go.md), and the admin CLI (added after this ADR, originally .NET) was migrated by [ADR 0030](0030-migrate-admin-cli-from-dotnet-to-go.md); **.NET is no longer used anywhere in the repository.** All non-.NET choices stand: iOS (Swift), Azure hosting, Cosmos DB (Serverless), the Cosmos-SDK / no-ORM data-access pattern, and GitHub Actions.

## Context
The project "town-crier" requires a mobile application and a supporting backend API. The primary mobile platform is iOS, and the backend needs to be hosted in a cloud environment with strong enterprise support and scalability.

## Decision
We will use the following technology stack for the initial prototype and development:

1.  **Frontend:** Native iOS development using **Swift**.
2.  **Backend:** **.NET 10** (ASP.NET Core) for the API, hosted on **Azure Container Apps (Consumption Plan)** using **Native AOT**.
3.  **Hosting & Infrastructure:** **Microsoft Azure**.
4.  **Database:** **Azure Cosmos DB (Serverless)** for cost-effective, scalable data storage.
5.  **Data Access:** **Azure Cosmos DB SDK** (`Microsoft.Azure.Cosmos`) with **System.Text.Json** serialization. No ORM (EF Core, Dapper, etc.).
6.  **CI/CD:** **GitHub Actions** for automated building, testing, and deployment.
7.  **IaC:** **Pulumi** using **.NET 10 (C#)** for infrastructure as code.

### Rationale
- **Native Swift (iOS):** Provides the best performance, access to the latest platform APIs, and the most refined user experience for iPhone users.
- **.NET 10:** Released in November 2025, .NET 10 provides the latest performance enhancements and stability features, making it the ideal choice for building efficient and scalable APIs.
- **Azure Container Apps (Consumption):** Offers a "scale-to-zero" model with a superior free tier (2M requests/month), ensuring near-zero costs when the application is idle and lower operational costs than Functions.
- **Native AOT:** Using **.NET 10** Native AOT with Azure Container Apps eliminates the platform overhead of a separate host process, providing the fastest possible "cold start" and minimal memory footprint.
- **Cosmos DB (Serverless):** Eliminates hourly compute costs, charging only for storage and actual database operations, which is ideal for low-usage or prototype phases.
- **Cosmos DB SDK (No ORM):** EF Core's Native AOT support remains experimental (as of .NET 10) with significant limitations around runtime code generation in the query pipeline. Dapper is a relational micro-ORM and incompatible with Cosmos DB's document model. The Cosmos DB SDK is the thinnest, most natural data access layer — it is Native AOT-compatible when configured with System.Text.Json (replacing the default Newtonsoft.Json serializer), avoids ORM mapping overhead, and aligns with the hexagonal architecture by keeping repository implementations explicit behind port interfaces. Cosmos DB is schemaless, so Code First migrations offer little value.
- **Azure:** Offers seamless integration with .NET 10, robust DevOps capabilities, and a wide range of services for scaling and managing the backend.
- **GitHub Actions:** Provides a unified CI/CD platform directly integrated with our source control, allowing for fast iteration and automated quality checks.
- **Pulumi with .NET 10:** Enables infrastructure definitions using the same language and tooling (C#/.NET) as our backend, promoting code reuse, type safety, and a unified developer experience.

## Consequences
- Development will focus on the iOS platform first.
- Developers will need proficiency in Swift and C#/.NET.
- Azure costs will remain extremely low (near $0) for low-volume usage.
- Native AOT requires specific coding patterns (e.g., avoiding reflection, using source generators, System.Text.Json) to ensure compatibility.
- Data access uses the Cosmos DB SDK directly — repository implementations handle document serialization and partition key strategies behind application-layer port interfaces.
- Containerization (Docker) will be required for the backend deployment.
- Pulumi state management and GitHub Actions secrets will need to be securely configured.

## Amendments

### 2026-03-31
- Updated: Data access changed from the **Microsoft.Azure.Cosmos SDK** to a **custom Cosmos DB REST client** (`CosmosRestClient`). The custom client talks directly to the Cosmos DB REST API (version `2018-12-31`) using `Azure.Identity` (`DefaultAzureCredential`) for authentication and `System.Text.Json` source-generated serialization. The `Microsoft.Azure.Cosmos` NuGet package is no longer referenced — the only Azure package is `Azure.Identity 1.19.0`. HTTP resilience (retry with exponential backoff for 429/408/503/449) is handled via `Microsoft.Extensions.Http.Resilience`. This gives full Native AOT compatibility without depending on the SDK's internal serialization or reflection paths, and keeps the dependency tree minimal.

### 2026-06-15
- Reversed (item 2, backend): the backend API and worker were **migrated from .NET 10 / Native AOT to Go** and the .NET source was deleted. See [ADR 0028](0028-migrate-backend-from-dotnet-to-go.md). The Native AOT rationale (System.Text.Json source generators, reflection avoidance, the custom `CosmosRestClient` from the 2026-03-31 amendment) no longer applies to the backend — Go uses stdlib `net/http`/`encoding/json`/`log/slog` and the official `azcosmos`/`azservicebus` SDKs.
- Unchanged: every other item in this ADR still holds. iOS (Swift), Azure hosting, Cosmos DB (Serverless), GitHub Actions, and **Pulumi in .NET 10 (C#)** for IaC are all retained. The migration was backend-language-only; `/infra` (Pulumi) and `/cli` remain on .NET.

### 2026-06-18
- Reversed (item 7, IaC): the **Pulumi infrastructure program was ported from C#/.NET to Go** (zero-diff, no resource changes). See [ADR 0029](0029-migrate-infrastructure-from-dotnet-to-go.md). The 2026-06-15 amendment above said `/infra` "remains on .NET" — that is no longer true. Pulumi is still the IaC tool, with the same stacks and Azure-native provider version (3.16.0); only the program language changed (`runtime: go`, `infra/go.mod`, no `infra/global.json`).
- Unchanged: **`/cli` remains on .NET** and is now the only .NET component in the repo. iOS (Swift), Azure hosting, Cosmos DB (Serverless), and GitHub Actions still stand.

### 2026-06-19
- Reversed (CLI): the admin CLI (`/cli`, originally C#/.NET 10 Native AOT — added after this ADR) was **rebuilt in Go**, feature-identical. See [ADR 0030](0030-migrate-admin-cli-from-dotnet-to-go.md). The 2026-06-18 amendment's closing line — "`/cli` remains on .NET and is now the only .NET component in the repo" — no longer holds.
- Current state: **.NET is fully removed from the repository.** The server-side stack is Go (backend, infrastructure, and CLI), with iOS (Swift) and web (TypeScript) as the only other ecosystems. No `setup-dotnet`, NuGet cache, or .NET SDK pin remains anywhere in CI or the repo. This closes the language consolidation begun by [ADR 0028](0028-migrate-backend-from-dotnet-to-go.md) and [ADR 0029](0029-migrate-infrastructure-from-dotnet-to-go.md).
