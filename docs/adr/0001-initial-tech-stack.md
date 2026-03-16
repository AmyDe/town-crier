# ADR 0001: Initial Tech Stack Decision

## Status
Accepted

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

## Rationale
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
