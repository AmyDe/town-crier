---
name: dotnet-coding-standards
description: Enforce .NET coding standards for the /api backend: DDD with rich domain models, hexagonal architecture (ports & adapters), CQRS command/query handlers, TUnit testing with builder pattern, Cosmos DB SDK repository implementations, and Native AOT compatibility (System.Text.Json, JsonSerializerContext). Use this skill whenever writing, reviewing, refactoring, or scaffolding C# code in the API layers (domain, application, infrastructure, web) — including creating entities, value objects, handlers, repository adapters, or tests. Also use when applying .editorconfig or analyzer rules. Do NOT use for Pulumi infrastructure code, iOS/Swift, CI/CD pipelines, Dockerfiles, or general architecture questions.
---

# Dotnet Coding Standards

## Overview

This skill provides strict guidelines for C#/.NET development, prioritizing **Domain-Driven Design (DDD)**, **Test-Driven Development (TDD)**, and robust testing practices using **TUnit**. All code targets **.NET 10 with Native AOT** publishing.

## Project Structure

The API lives in `/api` following Hexagonal (Ports & Adapters) architecture:

```
/api
├── src/
│   ├── town-crier.domain/         # Domain Layer — pure business logic
│   ├── town-crier.application/    # Application Layer — Handlers, Ports
│   ├── town-crier.infrastructure/ # Infrastructure Layer — Adapters, CosmosClient
│   └── town-crier.web/            # Entry point — Program.cs, DI, config
└── tests/
    └── town-crier.application.tests/  # TUnit tests targeting Handlers
```

Directory names use `town-crier.*` (lowercase with hyphens). C# namespaces use PascalCase: `TownCrier.Domain`, `TownCrier.Application`, etc.

## Core Mandates

### 1. Domain-Driven Design (DDD)
- **Domain First:** Prioritize pure business logic in the Domain layer. Infrastructure (DB, API) depends on Domain, never the reverse.
- **Rich Models:** Encapsulate all business logic within entities and value objects. Avoid "anemic" models (pure data bags). Logic must never leak into Handlers.
- **Ubiquitous Language:** Code must mirror business terminology exactly.

**Example — Rich Entity:**
```csharp
public sealed class PlanningApplication
{
    public PlanningApplicationId Id { get; private set; }
    public string Reference { get; private set; }
    public ApplicationStatus Status { get; private set; }
    public DateOnly ReceivedDate { get; private set; }

    // Business logic lives HERE, not in handlers
    public void MarkAsDecided(Decision decision, DateOnly decisionDate)
    {
        if (Status != ApplicationStatus.UnderReview)
            throw new DomainException("Only applications under review can be decided.");

        Status = decision == Decision.Approved
            ? ApplicationStatus.Approved
            : ApplicationStatus.Refused;
    }
}
```

### 2. Architecture Style (Hexagonal / Ports & Adapters)
- **Structure:** Organize code into concentric layers.
    - **Domain (Core):** Pure business logic, Entities, Value Objects. No external dependencies.
    - **Application (Ports):** Use Cases (Handlers), Input/Output Ports (Interfaces). Defines *what* the system does. Handlers must be **lightweight orchestrators**; they coordinate dependencies but strictly delegate all business logic to Domain entities. Depends ONLY on Domain.
    - **Infrastructure (Adapters):** Implementations of Ports (Repositories, External APIs), Controllers, Cosmos DB access. Depends on Application.
- **Dependency Rule:** Dependencies point **inward**. Inner layers verify business rules; outer layers handle mechanics.
- **Ports:** Interfaces defined in the Application layer (e.g., `IPlanningApplicationRepository`).
- **Adapters:** Implementations in the Infrastructure layer (e.g., `CosmosPlanningApplicationRepository`).

**Example — Port (Application layer):**
```csharp
public interface IPlanningApplicationRepository
{
    Task<PlanningApplication?> GetByIdAsync(PlanningApplicationId id, CancellationToken ct);
    Task SaveAsync(PlanningApplication application, CancellationToken ct);
}
```

**Example — Handler (Application layer):**
```csharp
public sealed class DecidePlanningApplicationCommandHandler
{
    private readonly IPlanningApplicationRepository _repository;

    public DecidePlanningApplicationCommandHandler(IPlanningApplicationRepository repository)
    {
        _repository = repository;
    }

    public async Task HandleAsync(DecidePlanningApplicationCommand command, CancellationToken ct)
    {
        var application = await _repository.GetByIdAsync(command.ApplicationId, ct)
            ?? throw new NotFoundException(command.ApplicationId);

        // Handler orchestrates, domain decides
        application.MarkAsDecided(command.Decision, command.DecisionDate);

        await _repository.SaveAsync(application, ct);
    }
}
```

### 3. Testing Strategy (TDD & TUnit)
- **Framework:** Use **TUnit** for all testing.
- **Unit of Work:** **Handlers** (Command/Query) are the primary testable unit in almost all cases.
- **Workflow:** Strict **Red-Green-Refactor**. Write the test *before* the implementation.
- **No Mocking Frameworks:** Never add dependencies on mocking libraries (Moq, NSubstitute, FakeItEasy, etc.). They rely on reflection/dynamic proxies which are incompatible with Native AOT, and they encourage testing implementation details rather than behavior.
- **Test Doubles — Preference Order:**
    1. **Real implementations** — Use the actual class when feasible (e.g., a real in-memory repository, a real domain service with no external dependencies).
    2. **Hand-written fakes** — When a real implementation has external dependencies (database, HTTP), create a simple in-memory fake that implements the same Port interface. Fakes live in the test project and use in-memory collections (e.g., `List<T>` or `Dictionary<TKey, TValue>`) to simulate persistence.
    3. **Hand-written spies** — When you need to verify that an interaction occurred (e.g., an event was published), create a spy that records calls. Keep spies minimal.
- **Methodology:**
    - **Setup:** Create test doubles for Port interfaces using the preference order above.
    - **Arrange:** Seed the fake repositories with domain entities built via the **Builder Pattern**.
    - **Act:** Execute the Handler.
    - **Assert:** Verify outcomes by inspecting handler return values, repository state, or domain entity state.
- **Focus:** Test **behavior** (public API/Business Value), never implementation details.
- **Data Construction:** Always use the **Builder Pattern** to seed test data. Never strictly couple tests to constructors.
- **No Reflection:** Reflection is forbidden in tests and production code (Native AOT incompatible).
- **Quality:**
    - **Rename** tests if the name doesn't clearly state the behavior (e.g., `Should_CalculateTotal_When_ItemsAdded`).
    - **Add Assertions** to existing tests if they are weak.
    - **Don't Over-Test:** Avoid testing framework features or trivial getters/setters.

**Example — Test with Builder & Fake Repository:**
```csharp
[Test]
public async Task Should_MarkApplicationAsApproved_When_UnderReview()
{
    // Arrange
    var application = new PlanningApplicationBuilder()
        .WithStatus(ApplicationStatus.UnderReview)
        .Build();
    var repository = new FakePlanningApplicationRepository(application);
    var handler = new DecidePlanningApplicationCommandHandler(repository);

    var command = new DecidePlanningApplicationCommand(
        application.Id, Decision.Approved, new DateOnly(2026, 3, 15));

    // Act
    await handler.HandleAsync(command, CancellationToken.None);

    // Assert
    var saved = await repository.GetByIdAsync(application.Id, CancellationToken.None);
    await Assert.That(saved!.Status).IsEqualTo(ApplicationStatus.Approved);
}
```

### 4. Architecture Patterns (CQRS)
- **Pattern:** Use strict **Command/Query Separation**.
    - **Commands:** `Command` / `CommandHandler` (Mutate state, return Result/void).
    - **Queries:** `Query` / `QueryHandler` (Read state, return Data).
    - **Events:** `Event` / `EventHandler` (React to changes).
- **Implementation:** **Manual dispatch only.** Do NOT use libraries like MediatR or Brighter. Implement simple, type-safe handlers to avoid dependency bloat.
- **Dependencies:** Keep the dependency tree minimal. Avoid "convenience" libraries that add unnecessary weight.

### 5. Data Access (Cosmos DB SDK)
- **SDK:** Use the **Azure Cosmos DB SDK** (`Microsoft.Azure.Cosmos`) directly. No ORM — no EF Core, no Dapper.
- **Serialization:** Configure `CosmosClient` with **System.Text.Json** via `CosmosSystemTextJsonSerializer`. Newtonsoft.Json is not Native AOT-compatible.
- **Database:** Target **Azure Cosmos DB (Serverless)**.
- **Repository Pattern:** Implement Port interfaces in the Infrastructure layer. Repositories handle all Cosmos DB concerns (partition keys, container references, document mapping) — the Application layer never sees `CosmosClient` or container details.
- **Partition Keys:** Design partition keys around query access patterns. Document this choice per container.
- **Document Mapping:** Keep Cosmos DB documents (DTOs) separate from Domain entities. The repository maps between them, so the Domain layer stays persistence-ignorant.

**Example — Adapter (Infrastructure layer):**
```csharp
public sealed class CosmosPlanningApplicationRepository : IPlanningApplicationRepository
{
    private readonly Container _container;

    public CosmosPlanningApplicationRepository(CosmosClient client)
    {
        _container = client.GetContainer("town-crier", "planning-applications");
    }

    public async Task<PlanningApplication?> GetByIdAsync(
        PlanningApplicationId id, CancellationToken ct)
    {
        try
        {
            var response = await _container.ReadItemAsync<PlanningApplicationDocument>(
                id.Value.ToString(),
                new PartitionKey(id.Value.ToString()),
                cancellationToken: ct);

            return response.Resource.ToDomain();
        }
        catch (CosmosException ex) when (ex.StatusCode == HttpStatusCode.NotFound)
        {
            return null;
        }
    }

    public async Task SaveAsync(PlanningApplication application, CancellationToken ct)
    {
        var document = PlanningApplicationDocument.FromDomain(application);
        await _container.UpsertItemAsync(document,
            new PartitionKey(document.Id),
            cancellationToken: ct);
    }
}
```

### 6. Native AOT Compatibility
All code must be Native AOT-compatible. This means:
- **No reflection.** Avoid `typeof(T).GetProperties()`, `Activator.CreateInstance`, or any `System.Reflection` usage.
- **System.Text.Json source generators.** Use `[JsonSerializable]` attributes on a `JsonSerializerContext` for all serialized types — the JSON serializer cannot discover types at runtime under AOT.
- **No dynamic assembly loading.** No `Assembly.Load`, no MEF, no runtime code generation.
- **Trim-safe code.** Avoid patterns that break when the linker removes unused code paths.
- **DI registration:** Use concrete factory registrations where possible. Avoid `services.AddScoped(typeof(IGeneric<>), typeof(Generic<>))` open-generic registrations unless verified AOT-safe.

**Example — JSON Source Generator:**
```csharp
[JsonSerializable(typeof(PlanningApplicationDocument))]
[JsonSerializable(typeof(List<PlanningApplicationDocument>))]
internal partial class AppJsonSerializerContext : JsonSerializerContext;
```

## Workflow

### 1. Verification
To check the codebase for style and formatting issues:

```bash
dotnet format --verify-no-changes
dotnet build
```

### 2. Auto-Formatting
To automatically fix formatting issues:

```bash
dotnet format
```

### 3. Setup Enforcements
To enforce standards in a project, use the bundled assets.

#### Apply .editorconfig
Copy the standard `.editorconfig` to the solution root.

```bash
cp .claude/skills/dotnet-coding-standards/assets/.editorconfig .
```

#### Apply Directory.Build.props
Copy `Directory.Build.props` to the solution root to enable strict mode and add analyzers.

```bash
cp .claude/skills/dotnet-coding-standards/assets/Directory.Build.props .
```

## Guidelines

### Naming Conventions
- **Classes/Methods/Properties:** PascalCase
- **Private Fields:** _camelCase
- **Interfaces:** IPascalCase
- **Async Methods:** End with `Async` suffix
- **Directories/Projects:** `town-crier.*` (lowercase, hyphenated)
- **Namespaces:** `TownCrier.*` (PascalCase)

### Best Practices
- Use `var` when the type is apparent from the right-hand side.
- Prefer pattern matching (`is`) over `as` + null check.
- Use `async`/`await` for I/O bound operations; avoid `.Result` or `.Wait()`.
- Properties should be used instead of public fields.
- Use `sealed` on classes by default unless inheritance is explicitly designed.
- Pass `CancellationToken` through all async call chains.
