# Legal Documents API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve privacy policy and terms of service as structured JSON from an anonymous API endpoint, providing a single source of truth across all clients.

**Architecture:** Follows existing CQRS pattern (static query handler, no DI). Content is hardcoded in the handler — no database. Anonymous endpoint at `GET /v1/legal/{documentType}`.

**Tech Stack:** .NET 10, ASP.NET Core Minimal APIs, TUnit, System.Text.Json source generators (Native AOT)

**Spec:** `docs/superpowers/specs/2026-04-08-legal-documents-api-design.md`

---

### File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `api/src/town-crier.application/Legal/GetLegalDocumentQuery.cs` | Query record |
| Create | `api/src/town-crier.application/Legal/GetLegalDocumentResult.cs` | Result DTOs |
| Create | `api/src/town-crier.application/Legal/GetLegalDocumentQueryHandler.cs` | Handler with hardcoded content |
| Create | `api/src/town-crier.web/Endpoints/LegalEndpoints.cs` | HTTP endpoint |
| Modify | `api/src/town-crier.web/Extensions/WebApplicationExtensions.cs:28` | Register endpoint |
| Modify | `api/src/town-crier.web/AppJsonSerializerContext.cs:48-51` | Register result type for Native AOT |
| Create | `api/tests/town-crier.application.tests/Legal/GetLegalDocumentQueryHandlerTests.cs` | Handler tests |

---

### Task 1: Application Layer Types

**Files:**
- Create: `api/src/town-crier.application/Legal/GetLegalDocumentQuery.cs`
- Create: `api/src/town-crier.application/Legal/GetLegalDocumentResult.cs`

- [ ] **Step 1: Create the query record**

```csharp
// api/src/town-crier.application/Legal/GetLegalDocumentQuery.cs
namespace TownCrier.Application.Legal;

public sealed record GetLegalDocumentQuery(string DocumentType);
```

- [ ] **Step 2: Create the result records**

```csharp
// api/src/town-crier.application/Legal/GetLegalDocumentResult.cs
namespace TownCrier.Application.Legal;

public sealed record GetLegalDocumentResult(
    string DocumentType,
    string Title,
    string LastUpdated,
    IReadOnlyList<LegalDocumentSectionResult> Sections);

public sealed record LegalDocumentSectionResult(
    string Heading,
    string Body);
```

- [ ] **Step 3: Verify build**

Run: `dotnet build api/src/town-crier.application/town-crier.application.csproj`
Expected: Build succeeded

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.application/Legal/
git commit -m "feat(api): add legal document query and result types"
```

---

### Task 2: Handler (TDD)

**Files:**
- Create: `api/tests/town-crier.application.tests/Legal/GetLegalDocumentQueryHandlerTests.cs`
- Create: `api/src/town-crier.application/Legal/GetLegalDocumentQueryHandler.cs`

- [ ] **Step 1: Write failing tests**

```csharp
// api/tests/town-crier.application.tests/Legal/GetLegalDocumentQueryHandlerTests.cs
using TownCrier.Application.Legal;

namespace TownCrier.Application.Tests.Legal;

public sealed class GetLegalDocumentQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnPrivacyPolicy_When_DocumentTypeIsPrivacy()
    {
        var query = new GetLegalDocumentQuery("privacy");

        var result = await GetLegalDocumentQueryHandler.HandleAsync(query, CancellationToken.None);

        await Assert.That(result).IsNotNull();
        await Assert.That(result!.DocumentType).IsEqualTo("privacy");
        await Assert.That(result.Title).IsEqualTo("Privacy Policy");
        await Assert.That(result.LastUpdated).IsEqualTo("2026-03-16");
        await Assert.That(result.Sections).HasCount().EqualTo(6);
        await Assert.That(result.Sections[0].Heading).IsEqualTo("What We Collect");
    }

    [Test]
    public async Task Should_ReturnTermsOfService_When_DocumentTypeIsTerms()
    {
        var query = new GetLegalDocumentQuery("terms");

        var result = await GetLegalDocumentQueryHandler.HandleAsync(query, CancellationToken.None);

        await Assert.That(result).IsNotNull();
        await Assert.That(result!.DocumentType).IsEqualTo("terms");
        await Assert.That(result.Title).IsEqualTo("Terms of Service");
        await Assert.That(result.LastUpdated).IsEqualTo("2026-03-16");
        await Assert.That(result.Sections).HasCount().EqualTo(6);
        await Assert.That(result.Sections[0].Heading).IsEqualTo("Acceptance of Terms");
    }

    [Test]
    public async Task Should_ReturnNull_When_DocumentTypeIsUnknown()
    {
        var query = new GetLegalDocumentQuery("unknown");

        var result = await GetLegalDocumentQueryHandler.HandleAsync(query, CancellationToken.None);

        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_BeCaseInsensitive_When_MatchingDocumentType()
    {
        var query = new GetLegalDocumentQuery("Privacy");

        var result = await GetLegalDocumentQueryHandler.HandleAsync(query, CancellationToken.None);

        await Assert.That(result).IsNotNull();
        await Assert.That(result!.DocumentType).IsEqualTo("privacy");
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "GetLegalDocumentQueryHandler"`
Expected: FAIL — `GetLegalDocumentQueryHandler` does not exist

- [ ] **Step 3: Implement the handler**

```csharp
// api/src/town-crier.application/Legal/GetLegalDocumentQueryHandler.cs
namespace TownCrier.Application.Legal;

public static class GetLegalDocumentQueryHandler
{
    public static Task<GetLegalDocumentResult?> HandleAsync(
        GetLegalDocumentQuery query, CancellationToken ct)
    {
        var result = query.DocumentType.ToLowerInvariant() switch
        {
            "privacy" => BuildPrivacyPolicy(),
            "terms" => BuildTermsOfService(),
            _ => null,
        };

        return Task.FromResult(result);
    }

    private static GetLegalDocumentResult BuildPrivacyPolicy() => new(
        DocumentType: "privacy",
        Title: "Privacy Policy",
        LastUpdated: "2026-03-16",
        Sections:
        [
            new("What We Collect",
                "Town Crier collects the minimum data needed to deliver planning " +
                "application alerts to you. This includes your postcode or saved " +
                "locations, notification preferences, and basic device information " +
                "required for push notifications. If you create an account, we also " +
                "store your email address for authentication and account recovery."),
            new("How We Process Your Data",
                "We process your location preferences to match relevant planning " +
                "applications from local authority data provided by PlanIt " +
                "(planit.org.uk). Your data is processed on secure servers hosted " +
                "in Microsoft Azure (UK region). We do not sell, rent, or share " +
                "your personal data with third parties for marketing purposes."),
            new("Data Storage and Retention",
                "Your data is stored securely in Azure Cosmos DB within the UK. " +
                "We retain your data for as long as your account is active. If you " +
                "stop using Town Crier, your data will be automatically deleted " +
                "after 12 months of inactivity."),
            new("Your Rights",
                "Under UK GDPR, you have the right to access, correct, and request " +
                "deletion of your personal data at any time. You can delete your " +
                "account and all associated data from within the app settings. You " +
                "also have the right to data portability and the right to withdraw " +
                "consent for optional data processing."),
            new("Push Notifications",
                "Town Crier uses Apple Push Notification Service to deliver planning " +
                "alerts. You can disable notifications at any time through your " +
                "device settings or within the app. Disabling notifications does not " +
                "delete your account or saved preferences."),
            new("Contact",
                "If you have questions about this privacy policy or wish to exercise " +
                "your data rights, please contact us at privacy@towncrier.app."),
        ]);

    private static GetLegalDocumentResult BuildTermsOfService() => new(
        DocumentType: "terms",
        Title: "Terms of Service",
        LastUpdated: "2026-03-16",
        Sections:
        [
            new("Acceptance of Terms",
                "By using Town Crier, you agree to these Terms of Service. If you " +
                "do not agree, please do not use the app. We may update these terms " +
                "from time to time and will notify you of material changes."),
            new("Service Description",
                "Town Crier provides notifications about UK local authority planning " +
                "applications based on your chosen locations. Planning data is sourced " +
                "from PlanIt (planit.org.uk) and local authority public registers. " +
                "While we strive for accuracy, we do not guarantee the completeness " +
                "or timeliness of planning data."),
            new("Subscriptions",
                "Town Crier offers both free and premium subscription tiers. Premium " +
                "subscriptions are billed through the Apple App Store. You can manage " +
                "or cancel your subscription at any time through your App Store " +
                "account settings. Refunds are handled by Apple in accordance with " +
                "their refund policy."),
            new("Acceptable Use",
                "You agree to use Town Crier for its intended purpose of monitoring " +
                "planning applications. You must not attempt to reverse-engineer the " +
                "app, scrape data at scale, or use the service to harass or spam " +
                "other users or planning authorities."),
            new("Limitation of Liability",
                "Town Crier is provided as-is. We are not liable for decisions made " +
                "based on planning data shown in the app. Always verify critical " +
                "planning information directly with your local authority. Our total " +
                "liability is limited to the amount you have paid for the service in " +
                "the preceding 12 months."),
            new("Governing Law",
                "These terms are governed by the laws of England and Wales. Any " +
                "disputes will be subject to the exclusive jurisdiction of the " +
                "courts of England and Wales."),
        ]);
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "GetLegalDocumentQueryHandler"`
Expected: 4 tests passed

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.application/Legal/GetLegalDocumentQueryHandler.cs api/tests/town-crier.application.tests/Legal/
git commit -m "feat(api): implement legal document query handler with TDD"
```

---

### Task 3: Endpoint, Wiring, and JSON Serialization

**Files:**
- Create: `api/src/town-crier.web/Endpoints/LegalEndpoints.cs`
- Modify: `api/src/town-crier.web/Extensions/WebApplicationExtensions.cs:28`
- Modify: `api/src/town-crier.web/AppJsonSerializerContext.cs:48-51`

- [ ] **Step 1: Create the endpoint**

```csharp
// api/src/town-crier.web/Endpoints/LegalEndpoints.cs
using TownCrier.Application.Legal;

namespace TownCrier.Web.Endpoints;

internal static class LegalEndpoints
{
    public static void MapLegalEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/legal/{documentType}", async (string documentType) =>
        {
            var result = await GetLegalDocumentQueryHandler.HandleAsync(
                new GetLegalDocumentQuery(documentType), CancellationToken.None);

            return result is not null ? Results.Ok(result) : Results.NotFound();
        })
        .AllowAnonymous();
    }
}
```

- [ ] **Step 2: Register the endpoint in WebApplicationExtensions.cs**

In `api/src/town-crier.web/Extensions/WebApplicationExtensions.cs`, add `v1.MapLegalEndpoints();` after the existing `v1.MapVersionConfigEndpoints();` line (around line 28).

- [ ] **Step 3: Register result type in AppJsonSerializerContext**

In `api/src/town-crier.web/AppJsonSerializerContext.cs`, add:
- `using TownCrier.Application.Legal;` to the imports
- `[JsonSerializable(typeof(GetLegalDocumentResult))]` before the class declaration (after the existing `GetVersionConfigResult` line)

- [ ] **Step 4: Build the full solution**

Run: `dotnet build api/`
Expected: Build succeeded with no errors

- [ ] **Step 5: Run all tests**

Run: `dotnet test api/`
Expected: All tests pass (including the 4 new handler tests)

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.web/Endpoints/LegalEndpoints.cs api/src/town-crier.web/Extensions/WebApplicationExtensions.cs api/src/town-crier.web/AppJsonSerializerContext.cs
git commit -m "feat(api): add anonymous legal documents endpoint at /v1/legal/{documentType}"
```
