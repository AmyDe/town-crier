# Offer Codes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Auto-triggered coding standards skills:** `dotnet-coding-standards` (Phases A–G), `ios-coding-standards` (Phase H), `react-coding-standards` (Phase I), `design-language` (Phases H + I for any UI work). Invoke them at the start of each relevant task.

**Goal:** Ship a platform-neutral single-use offer-code system — admin bulk generation via `tc`, user redemption on iOS and web Settings, time-bounded Personal/Pro grants.

**Spec:** `docs/specs/offer-codes.md`

**Architecture:** New `OfferCode` aggregate in the domain layer with single-use redemption semantics; CQRS command handlers in the application layer (`GenerateOfferCodesCommandHandler`, `RedeemOfferCodeCommandHandler`); `CosmosOfferCodeRepository` using the existing `ICosmosRestClient` (last-writer-wins, ETag concurrency deferred per spec); admin generate endpoint + user redeem endpoint in the web layer; iOS + web Settings UI + token-refresh after redeem.

**Tech Stack:** .NET 10 (Native AOT), Cosmos DB (via `ICosmosRestClient` wrapper), TUnit, SwiftUI / SwiftData, React 18 + TypeScript + Vitest, Pulumi (.NET).

---

## File Structure

### API — Domain (`api/src/town-crier.domain/OfferCodes/`)
| File | Responsibility |
|------|----------------|
| `OfferCode.cs` | Aggregate root; `Redeem(userId, now)` mutator; invariants |
| `OfferCodeAlreadyRedeemedException.cs` | Thrown from `Redeem` when already consumed |

### API — Application (`api/src/town-crier.application/OfferCodes/`)
| File | Responsibility |
|------|----------------|
| `IOfferCodeRepository.cs` | Port — `CreateAsync`, `GetAsync` |
| `IOfferCodeGenerator.cs` | Port — `Generate()` returns one canonical 12-char code |
| `OfferCodeGenerator.cs` | Default impl using `RandomNumberGenerator` + Crockford base32 |
| `OfferCodeFormat.cs` | Static helpers: `Normalize(input)`, `Format(canonical)`, `IsValidCanonical(s)` |
| `OfferCodeNotFoundException.cs` | Thrown by redeem when code doesn't exist |
| `AlreadySubscribedException.cs` | Thrown by redeem when `profile.Tier != Free` |
| `InvalidOfferCodeFormatException.cs` | Thrown by redeem when input can't be normalized |
| `GenerateOfferCodesCommand.cs` | `record(int Count, SubscriptionTier Tier, int DurationDays)` |
| `GenerateOfferCodesCommandHandler.cs` | Validates, loops Count times, persists |
| `GenerateOfferCodesResult.cs` | `record(IReadOnlyList<string> Codes)` — canonical form |
| `RedeemOfferCodeCommand.cs` | `record(string UserId, string Code)` |
| `RedeemOfferCodeCommandHandler.cs` | Normalize, fetch code, validate state, activate subscription, sync Auth0 |
| `RedeemOfferCodeResult.cs` | `record(SubscriptionTier Tier, DateTimeOffset ExpiresAt)` |

### API — Application Tests (`api/tests/town-crier.application.tests/OfferCodes/`)
| File | Responsibility |
|------|----------------|
| `FakeOfferCodeRepository.cs` | Hand-written in-memory fake |
| `FakeOfferCodeGenerator.cs` | Fake generator returning a pre-seeded queue |
| `OfferCodeFormatTests.cs` | Normalize/format/validate tests |
| `GenerateOfferCodesCommandHandlerTests.cs` | Validation + happy path |
| `RedeemOfferCodeCommandHandlerTests.cs` | All error paths + happy path |

### API — Domain Tests (`api/tests/town-crier.domain.tests/OfferCodes/`)
| File | Responsibility |
|------|----------------|
| `OfferCodeTests.cs` | Constructor invariants + `Redeem` behaviour |

### API — Infrastructure (`api/src/town-crier.infrastructure/OfferCodes/`)
| File | Responsibility |
|------|----------------|
| `OfferCodeDocument.cs` | Cosmos JSON shape; `FromDomain`, `ToDomain` |
| `CosmosOfferCodeRepository.cs` | `IOfferCodeRepository` impl |
| `InMemoryOfferCodeRepository.cs` | Used for local-dev (Testcontainers-free) |

Modified: `api/src/town-crier.infrastructure/Cosmos/CosmosContainerNames.cs` — add `OfferCodes` constant.
Modified: `api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs` — register `OfferCodeDocument`.

### API — Infrastructure Tests (`api/tests/town-crier.infrastructure.tests/OfferCodes/`)
| File | Responsibility |
|------|----------------|
| `CosmosOfferCodeRepositoryTests.cs` | Round-trip create → get → save-as-redeemed → get |

### API — Web (`api/src/town-crier.web/`)
| File | Responsibility |
|------|----------------|
| `Endpoints/OfferCodeEndpoints.cs` | `POST /v1/offer-codes/redeem` (user JWT) |
| `Endpoints/OfferCodeRequests.cs` | `GenerateOfferCodesRequest`, `RedeemOfferCodeRequest`, `RedeemOfferCodeResponse` DTOs |

Modified:
- `Endpoints/AdminEndpoints.cs` — add `POST /v1/admin/offer-codes`
- `AppJsonSerializerContext.cs` — register new DTOs
- `Extensions/WebApplicationExtensions.cs` — `v1.MapOfferCodeEndpoints()`
- `Extensions/ServiceCollectionExtensions.cs` — register handlers + generator + repository

### API — Web Tests (`api/tests/town-crier.web.tests/OfferCodes/`)
| File | Responsibility |
|------|----------------|
| `GenerateOfferCodesEndpointTests.cs` | Admin auth, validation, `text/plain` output |
| `RedeemOfferCodeEndpointTests.cs` | Each error mapping, happy path |

### CLI (`cli/src/tc/`)
| File | Responsibility |
|------|----------------|
| `Commands/GenerateOfferCodesCommand.cs` | Parse args → POST → stream response |

Modified:
- `Json/TcJsonContext.cs` — register `GenerateOfferCodesRequest`
- `Program.cs` — dispatch `generate-offer-codes` + help text

### CLI Tests (`cli/tests/tc.tests/`)
| File | Responsibility |
|------|----------------|
| `GenerateOfferCodesCommandTests.cs` | Arg validation, exit codes, output routing |

### Infra (`infra/`)
Modified: `EnvironmentStack.cs` — add `OfferCodes` container to `containerDefinitions`.

### iOS (`mobile/ios/packages/`)
| File | Responsibility |
|------|----------------|
| `town-crier-data/Sources/OfferCodes/OfferCodeService.swift` | Protocol + HTTP impl |
| `town-crier-data/Sources/OfferCodes/OfferCodeError.swift` | Mapped server error codes |
| `town-crier-presentation/Sources/Features/RedeemOfferCode/RedeemOfferCodeViewModel.swift` | MVVM-C ViewModel |
| `town-crier-presentation/Sources/Features/RedeemOfferCode/RedeemOfferCodeView.swift` | SwiftUI view |
| `town-crier-presentation/Sources/Features/RedeemOfferCode/RedeemOfferCodeCoordinator.swift` | Presents sheet + callbacks |

Modified:
- `town-crier-presentation/Sources/Features/Settings/SettingsView.swift` — new row
- `town-crier-presentation/Sources/Coordinators/SettingsCoordinator.swift` — wires the redeem coordinator

### iOS Tests (`mobile/ios/town-crier-tests/` or package tests)
| File | Responsibility |
|------|----------------|
| `FakeOfferCodeService.swift` | Protocol fake |
| `RedeemOfferCodeViewModelTests.swift` | All state transitions |

### Web (`web/src/features/offerCode/`)
| File | Responsibility |
|------|----------------|
| `api/redeemOfferCode.ts` | Typed fetch wrapper |
| `api/types.ts` | `RedeemResult`, `RedeemErrorCode` types |
| `hooks/useRedeemOfferCode.ts` | Hook-as-ViewModel |
| `components/RedeemOfferCode.tsx` | Form component |
| `components/RedeemOfferCode.module.css` | Styles via design tokens |
| `format/formatOfferCode.ts` | Live-format `XXXX-XXXX-XXXX` as user types |

Modified:
- `web/src/features/Settings/SettingsPage.tsx` — mount `RedeemOfferCode`
- `web/src/features/Settings/ConnectedSettingsPage.tsx` — pass needed deps

### Web Tests (`web/src/features/offerCode/__tests__/`)
| File | Responsibility |
|------|----------------|
| `fixtures/offerCodeFixtures.ts` | Builder for server responses |
| `spies/SpyOfferCodeClient.ts` | Hand-written spy |
| `useRedeemOfferCode.test.ts` | Hook state transitions |
| `RedeemOfferCode.test.tsx` | Component behaviour |
| `formatOfferCode.test.ts` | Formatter tests |

---

## Phase A — API Domain

### Task A1: `OfferCode` construction invariants

**Files:**
- Create: `api/src/town-crier.domain/OfferCodes/OfferCode.cs`
- Create: `api/src/town-crier.domain/OfferCodes/OfferCodeAlreadyRedeemedException.cs`
- Create: `api/tests/town-crier.domain.tests/OfferCodes/OfferCodeTests.cs`

- [ ] **Step 1: Create the failing test file**

```csharp
// api/tests/town-crier.domain.tests/OfferCodes/OfferCodeTests.cs
using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Domain.Tests.OfferCodes;

public sealed class OfferCodeTests
{
    [Test]
    public async Task Should_Construct_When_AllInputsValid()
    {
        var created = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var code = new OfferCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30, created);

        await Assert.That(code.Code).IsEqualTo("A7KMZQR3FNXP");
        await Assert.That(code.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(code.DurationDays).IsEqualTo(30);
        await Assert.That(code.CreatedAt).IsEqualTo(created);
        await Assert.That(code.RedeemedByUserId).IsNull();
        await Assert.That(code.RedeemedAt).IsNull();
        await Assert.That(code.IsRedeemed).IsFalse();
    }

    [Test]
    public async Task Should_Throw_When_TierIsFree()
    {
        var act = () => new OfferCode("A7KMZQR3FNXP", SubscriptionTier.Free, 30, DateTimeOffset.UtcNow);
        await Assert.That(act).Throws<ArgumentException>();
    }

    [Test]
    [Arguments(0)]
    [Arguments(-1)]
    [Arguments(366)]
    public async Task Should_Throw_When_DurationOutOfRange(int duration)
    {
        var act = () => new OfferCode("A7KMZQR3FNXP", SubscriptionTier.Pro, duration, DateTimeOffset.UtcNow);
        await Assert.That(act).Throws<ArgumentOutOfRangeException>();
    }

    [Test]
    [Arguments("SHORT")]                  // too short
    [Arguments("A7KMZQR3FNXPTOOLONG")]    // too long
    [Arguments("a7kmzqr3fnxp")]           // lowercase
    [Arguments("A7KM-ZQR3-FNXP")]         // has separators
    [Arguments("A7KMZQR3FNXI")]           // contains excluded letter I
    public async Task Should_Throw_When_CodeMalformed(string code)
    {
        var act = () => new OfferCode(code, SubscriptionTier.Pro, 30, DateTimeOffset.UtcNow);
        await Assert.That(act).Throws<ArgumentException>();
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd api && dotnet test --filter "FullyQualifiedName~OfferCodeTests" --no-restore 2>&1 | tail -20
```

Expected: build errors referencing missing `OfferCode` type.

- [ ] **Step 3: Create the exception**

```csharp
// api/src/town-crier.domain/OfferCodes/OfferCodeAlreadyRedeemedException.cs
namespace TownCrier.Domain.OfferCodes;

public sealed class OfferCodeAlreadyRedeemedException : Exception
{
    public OfferCodeAlreadyRedeemedException(string code)
        : base($"Offer code '{code}' has already been redeemed.")
    {
    }
}
```

- [ ] **Step 4: Create the aggregate**

```csharp
// api/src/town-crier.domain/OfferCodes/OfferCode.cs
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Domain.OfferCodes;

public sealed class OfferCode
{
    private const string CrockfordBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ";

    public OfferCode(string code, SubscriptionTier tier, int durationDays, DateTimeOffset createdAt)
    {
        ArgumentException.ThrowIfNullOrEmpty(code);

        if (!IsValidCanonicalCode(code))
        {
            throw new ArgumentException(
                $"Code '{code}' is not a valid canonical offer code (12 chars, Crockford base32).",
                nameof(code));
        }

        if (tier == SubscriptionTier.Free)
        {
            throw new ArgumentException("Offer codes cannot grant the Free tier.", nameof(tier));
        }

        if (durationDays < 1 || durationDays > 365)
        {
            throw new ArgumentOutOfRangeException(
                nameof(durationDays),
                durationDays,
                "Duration must be between 1 and 365 days.");
        }

        this.Code = code;
        this.Tier = tier;
        this.DurationDays = durationDays;
        this.CreatedAt = createdAt;
    }

    // Rehydration ctor for repository
    public OfferCode(
        string code,
        SubscriptionTier tier,
        int durationDays,
        DateTimeOffset createdAt,
        string? redeemedByUserId,
        DateTimeOffset? redeemedAt)
        : this(code, tier, durationDays, createdAt)
    {
        this.RedeemedByUserId = redeemedByUserId;
        this.RedeemedAt = redeemedAt;
    }

    public string Code { get; }

    public SubscriptionTier Tier { get; }

    public int DurationDays { get; }

    public DateTimeOffset CreatedAt { get; }

    public string? RedeemedByUserId { get; private set; }

    public DateTimeOffset? RedeemedAt { get; private set; }

    public bool IsRedeemed => this.RedeemedByUserId is not null;

    public void Redeem(string userId, DateTimeOffset now)
    {
        ArgumentException.ThrowIfNullOrEmpty(userId);

        if (this.IsRedeemed)
        {
            throw new OfferCodeAlreadyRedeemedException(this.Code);
        }

        this.RedeemedByUserId = userId;
        this.RedeemedAt = now;
    }

    private static bool IsValidCanonicalCode(string code)
    {
        if (code.Length != 12)
        {
            return false;
        }

        foreach (var c in code)
        {
            if (CrockfordBase32.IndexOf(c) < 0)
            {
                return false;
            }
        }

        return true;
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd api && dotnet test --filter "FullyQualifiedName~OfferCodeTests" --no-restore 2>&1 | tail -10
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.domain/OfferCodes/ api/tests/town-crier.domain.tests/OfferCodes/
git commit -m "feat(domain): add OfferCode aggregate with construction invariants"
```

---

### Task A2: `OfferCode.Redeem` behaviour

**Files:**
- Modify: `api/tests/town-crier.domain.tests/OfferCodes/OfferCodeTests.cs`

- [ ] **Step 1: Add failing redemption tests**

Append to `OfferCodeTests` class:

```csharp
[Test]
public async Task Should_RecordRedemption_When_RedeemCalled()
{
    var now = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
    var code = new OfferCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30, now.AddDays(-1));

    code.Redeem("auth0|user-1", now);

    await Assert.That(code.RedeemedByUserId).IsEqualTo("auth0|user-1");
    await Assert.That(code.RedeemedAt).IsEqualTo(now);
    await Assert.That(code.IsRedeemed).IsTrue();
}

[Test]
public async Task Should_Throw_When_RedeemCalledTwice()
{
    var now = DateTimeOffset.UtcNow;
    var code = new OfferCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30, now);
    code.Redeem("auth0|user-1", now);

    var act = () => code.Redeem("auth0|user-2", now);

    await Assert.That(act).Throws<OfferCodeAlreadyRedeemedException>();
}
```

- [ ] **Step 2: Run tests to verify they pass**

```bash
cd api && dotnet test --filter "FullyQualifiedName~OfferCodeTests" --no-restore 2>&1 | tail -10
```

Expected: pass (the aggregate from A1 already supports this — these are confirming tests).

- [ ] **Step 3: Commit**

```bash
git add api/tests/town-crier.domain.tests/OfferCodes/OfferCodeTests.cs
git commit -m "test(domain): cover OfferCode.Redeem behaviour"
```

---

## Phase B — Application: Generation

### Task B1: Code format helpers + tests

**Files:**
- Create: `api/src/town-crier.application/OfferCodes/OfferCodeFormat.cs`
- Create: `api/tests/town-crier.application.tests/OfferCodes/OfferCodeFormatTests.cs`

- [ ] **Step 1: Write failing tests**

```csharp
// api/tests/town-crier.application.tests/OfferCodes/OfferCodeFormatTests.cs
using TownCrier.Application.OfferCodes;

namespace TownCrier.Application.Tests.OfferCodes;

public sealed class OfferCodeFormatTests
{
    [Test]
    [Arguments("A7KMZQR3FNXP",      "A7KMZQR3FNXP")]   // already canonical
    [Arguments("a7kmzqr3fnxp",      "A7KMZQR3FNXP")]   // lowercase
    [Arguments("A7KM-ZQR3-FNXP",    "A7KMZQR3FNXP")]   // hyphens
    [Arguments("  A7KM ZQR3 FNXP ", "A7KMZQR3FNXP")]   // whitespace + spaces
    public async Task Normalize_Should_StripSeparatorsAndUppercase(string input, string expected)
    {
        var result = OfferCodeFormat.Normalize(input);
        await Assert.That(result).IsEqualTo(expected);
    }

    [Test]
    [Arguments("")]
    [Arguments("SHORT")]
    [Arguments("A7KMZQR3FNXPEXTRA")]
    [Arguments("A7KMZQR3FNXI")]  // excluded letter I
    public async Task Normalize_Should_Throw_When_InputInvalid(string input)
    {
        var act = () => OfferCodeFormat.Normalize(input);
        await Assert.That(act).Throws<InvalidOfferCodeFormatException>();
    }

    [Test]
    public async Task Format_Should_InsertHyphensEveryFourChars()
    {
        var display = OfferCodeFormat.Format("A7KMZQR3FNXP");
        await Assert.That(display).IsEqualTo("A7KM-ZQR3-FNXP");
    }

    [Test]
    public async Task IsValidCanonical_Should_ReturnTrue_For12AlphabetChars()
    {
        await Assert.That(OfferCodeFormat.IsValidCanonical("A7KMZQR3FNXP")).IsTrue();
    }

    [Test]
    [Arguments("a7kmzqr3fnxp")]
    [Arguments("A7KM-ZQR3-FNXP")]
    [Arguments("A7KMZQR3FNXI")]
    public async Task IsValidCanonical_Should_ReturnFalse_ForNonCanonical(string input)
    {
        await Assert.That(OfferCodeFormat.IsValidCanonical(input)).IsFalse();
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd api && dotnet test --filter "FullyQualifiedName~OfferCodeFormatTests" --no-restore 2>&1 | tail -15
```

Expected: build failure — `OfferCodeFormat` and `InvalidOfferCodeFormatException` don't exist.

- [ ] **Step 3: Create exception**

```csharp
// api/src/town-crier.application/OfferCodes/InvalidOfferCodeFormatException.cs
namespace TownCrier.Application.OfferCodes;

public sealed class InvalidOfferCodeFormatException : Exception
{
    public InvalidOfferCodeFormatException(string reason) : base(reason)
    {
    }
}
```

- [ ] **Step 4: Create format helper**

```csharp
// api/src/town-crier.application/OfferCodes/OfferCodeFormat.cs
namespace TownCrier.Application.OfferCodes;

public static class OfferCodeFormat
{
    public const int CanonicalLength = 12;
    public const string Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ";

    public static string Normalize(string input)
    {
        if (string.IsNullOrWhiteSpace(input))
        {
            throw new InvalidOfferCodeFormatException("Offer code is required.");
        }

        Span<char> buffer = stackalloc char[input.Length];
        var length = 0;

        foreach (var c in input)
        {
            if (c == '-' || char.IsWhiteSpace(c))
            {
                continue;
            }

            var upper = char.ToUpperInvariant(c);
            if (Alphabet.IndexOf(upper) < 0)
            {
                throw new InvalidOfferCodeFormatException(
                    $"Offer code contains invalid character '{c}'.");
            }

            buffer[length++] = upper;
        }

        if (length != CanonicalLength)
        {
            throw new InvalidOfferCodeFormatException(
                $"Offer code must be {CanonicalLength} characters (got {length}).");
        }

        return new string(buffer[..length]);
    }

    public static string Format(string canonical)
    {
        if (!IsValidCanonical(canonical))
        {
            throw new ArgumentException("Expected canonical 12-char code.", nameof(canonical));
        }

        return $"{canonical[..4]}-{canonical.Substring(4, 4)}-{canonical[8..]}";
    }

    public static bool IsValidCanonical(string? value)
    {
        if (value is null || value.Length != CanonicalLength)
        {
            return false;
        }

        foreach (var c in value)
        {
            if (Alphabet.IndexOf(c) < 0)
            {
                return false;
            }
        }

        return true;
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd api && dotnet test --filter "FullyQualifiedName~OfferCodeFormatTests" --no-restore 2>&1 | tail -10
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.application/OfferCodes/ api/tests/town-crier.application.tests/OfferCodes/OfferCodeFormatTests.cs
git commit -m "feat(application): add OfferCodeFormat normalize/format/validate helpers"
```

---

### Task B2: Repository port + fake

**Files:**
- Create: `api/src/town-crier.application/OfferCodes/IOfferCodeRepository.cs`
- Create: `api/src/town-crier.application/OfferCodes/OfferCodeNotFoundException.cs`
- Create: `api/tests/town-crier.application.tests/OfferCodes/FakeOfferCodeRepository.cs`

- [ ] **Step 1: Create the port**

```csharp
// api/src/town-crier.application/OfferCodes/IOfferCodeRepository.cs
using TownCrier.Domain.OfferCodes;

namespace TownCrier.Application.OfferCodes;

public interface IOfferCodeRepository
{
    Task<OfferCode?> GetAsync(string canonicalCode, CancellationToken ct);

    /// <summary>Inserts a new code. Throws if the code already exists (used to detect generator collisions).</summary>
    Task CreateAsync(OfferCode code, CancellationToken ct);

    Task SaveAsync(OfferCode code, CancellationToken ct);
}
```

- [ ] **Step 2: Create exception**

```csharp
// api/src/town-crier.application/OfferCodes/OfferCodeNotFoundException.cs
namespace TownCrier.Application.OfferCodes;

public sealed class OfferCodeNotFoundException : Exception
{
    public OfferCodeNotFoundException(string code)
        : base($"Offer code '{code}' was not found.")
    {
    }
}
```

- [ ] **Step 3: Create the fake**

```csharp
// api/tests/town-crier.application.tests/OfferCodes/FakeOfferCodeRepository.cs
using System.Collections.Concurrent;
using TownCrier.Application.OfferCodes;
using TownCrier.Domain.OfferCodes;

namespace TownCrier.Application.Tests.OfferCodes;

public sealed class FakeOfferCodeRepository : IOfferCodeRepository
{
    private readonly ConcurrentDictionary<string, OfferCode> store = new(StringComparer.Ordinal);

    public Task<OfferCode?> GetAsync(string canonicalCode, CancellationToken ct)
    {
        this.store.TryGetValue(canonicalCode, out var code);
        return Task.FromResult(code);
    }

    public Task CreateAsync(OfferCode code, CancellationToken ct)
    {
        if (!this.store.TryAdd(code.Code, code))
        {
            throw new InvalidOperationException($"Offer code '{code.Code}' already exists.");
        }

        return Task.CompletedTask;
    }

    public Task SaveAsync(OfferCode code, CancellationToken ct)
    {
        this.store[code.Code] = code;
        return Task.CompletedTask;
    }

    public int Count => this.store.Count;

    public IReadOnlyCollection<OfferCode> All => this.store.Values.ToArray();
}
```

- [ ] **Step 4: Build to confirm compilation**

```bash
cd api && dotnet build --no-restore 2>&1 | tail -10
```

Expected: build succeeds.

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.application/OfferCodes/IOfferCodeRepository.cs \
        api/src/town-crier.application/OfferCodes/OfferCodeNotFoundException.cs \
        api/tests/town-crier.application.tests/OfferCodes/FakeOfferCodeRepository.cs
git commit -m "feat(application): add IOfferCodeRepository port and fake"
```

---

### Task B3: Code generator port, default impl, and fake

**Files:**
- Create: `api/src/town-crier.application/OfferCodes/IOfferCodeGenerator.cs`
- Create: `api/src/town-crier.application/OfferCodes/OfferCodeGenerator.cs`
- Create: `api/tests/town-crier.application.tests/OfferCodes/OfferCodeGeneratorTests.cs`
- Create: `api/tests/town-crier.application.tests/OfferCodes/FakeOfferCodeGenerator.cs`

- [ ] **Step 1: Write failing test for `OfferCodeGenerator`**

```csharp
// api/tests/town-crier.application.tests/OfferCodes/OfferCodeGeneratorTests.cs
using TownCrier.Application.OfferCodes;

namespace TownCrier.Application.Tests.OfferCodes;

public sealed class OfferCodeGeneratorTests
{
    [Test]
    public async Task Generate_Should_Return12CharCanonicalCode()
    {
        var generator = new OfferCodeGenerator();

        var code = generator.Generate();

        await Assert.That(code).HasLength().EqualTo(OfferCodeFormat.CanonicalLength);
        await Assert.That(OfferCodeFormat.IsValidCanonical(code)).IsTrue();
    }

    [Test]
    public async Task Generate_Should_ReturnDifferentCodesEachCall()
    {
        var generator = new OfferCodeGenerator();

        var codes = Enumerable.Range(0, 100).Select(_ => generator.Generate()).ToHashSet();

        await Assert.That(codes).HasCount().EqualTo(100);
    }
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd api && dotnet test --filter "FullyQualifiedName~OfferCodeGeneratorTests" --no-restore 2>&1 | tail -15
```

Expected: missing types.

- [ ] **Step 3: Create port**

```csharp
// api/src/town-crier.application/OfferCodes/IOfferCodeGenerator.cs
namespace TownCrier.Application.OfferCodes;

public interface IOfferCodeGenerator
{
    string Generate();
}
```

- [ ] **Step 4: Create default impl**

```csharp
// api/src/town-crier.application/OfferCodes/OfferCodeGenerator.cs
using System.Security.Cryptography;

namespace TownCrier.Application.OfferCodes;

public sealed class OfferCodeGenerator : IOfferCodeGenerator
{
    public string Generate()
    {
        // 12 characters × 5 bits = 60 bits. Use 8 bytes (64 bits) and discard the top 4.
        Span<byte> randomBytes = stackalloc byte[8];
        RandomNumberGenerator.Fill(randomBytes);

        var value = BitConverter.ToUInt64(randomBytes);

        Span<char> buffer = stackalloc char[OfferCodeFormat.CanonicalLength];
        for (var i = OfferCodeFormat.CanonicalLength - 1; i >= 0; i--)
        {
            buffer[i] = OfferCodeFormat.Alphabet[(int)(value & 0x1F)];
            value >>= 5;
        }

        return new string(buffer);
    }
}
```

- [ ] **Step 5: Create fake generator**

```csharp
// api/tests/town-crier.application.tests/OfferCodes/FakeOfferCodeGenerator.cs
using TownCrier.Application.OfferCodes;

namespace TownCrier.Application.Tests.OfferCodes;

public sealed class FakeOfferCodeGenerator : IOfferCodeGenerator
{
    private readonly Queue<string> codes;

    public FakeOfferCodeGenerator(params string[] codes)
    {
        this.codes = new Queue<string>(codes);
    }

    public string Generate()
    {
        if (this.codes.Count == 0)
        {
            throw new InvalidOperationException("FakeOfferCodeGenerator exhausted.");
        }

        return this.codes.Dequeue();
    }
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd api && dotnet test --filter "FullyQualifiedName~OfferCodeGeneratorTests" --no-restore 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.application/OfferCodes/IOfferCodeGenerator.cs \
        api/src/town-crier.application/OfferCodes/OfferCodeGenerator.cs \
        api/tests/town-crier.application.tests/OfferCodes/OfferCodeGeneratorTests.cs \
        api/tests/town-crier.application.tests/OfferCodes/FakeOfferCodeGenerator.cs
git commit -m "feat(application): add OfferCodeGenerator with Crockford base32 encoding"
```

---

### Task B4: `GenerateOfferCodesCommandHandler`

**Files:**
- Create: `api/src/town-crier.application/OfferCodes/GenerateOfferCodesCommand.cs`
- Create: `api/src/town-crier.application/OfferCodes/GenerateOfferCodesResult.cs`
- Create: `api/src/town-crier.application/OfferCodes/GenerateOfferCodesCommandHandler.cs`
- Create: `api/tests/town-crier.application.tests/OfferCodes/GenerateOfferCodesCommandHandlerTests.cs`

- [ ] **Step 1: Write failing tests**

```csharp
// api/tests/town-crier.application.tests/OfferCodes/GenerateOfferCodesCommandHandlerTests.cs
using TownCrier.Application.OfferCodes;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.OfferCodes;

public sealed class GenerateOfferCodesCommandHandlerTests
{
    [Test]
    public async Task Should_GenerateAndPersist_RequestedCount()
    {
        var repository = new FakeOfferCodeRepository();
        var generator = new FakeOfferCodeGenerator(
            "AAAAAAAAAAAA", "BBBBBBBBBBBB", "CCCCCCCCCCCC");
        var clock = new FakeClock(new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));
        var handler = new GenerateOfferCodesCommandHandler(repository, generator, clock);

        var result = await handler.HandleAsync(
            new GenerateOfferCodesCommand(3, SubscriptionTier.Pro, 30),
            CancellationToken.None);

        await Assert.That(result.Codes).HasCount().EqualTo(3);
        await Assert.That(repository.Count).IsEqualTo(3);
        await Assert.That(repository.All.All(c => c.Tier == SubscriptionTier.Pro)).IsTrue();
        await Assert.That(repository.All.All(c => c.DurationDays == 30)).IsTrue();
    }

    [Test]
    [Arguments(0)]
    [Arguments(-5)]
    [Arguments(1001)]
    public async Task Should_Throw_When_CountOutOfRange(int count)
    {
        var handler = new GenerateOfferCodesCommandHandler(
            new FakeOfferCodeRepository(),
            new FakeOfferCodeGenerator(),
            new FakeClock(DateTimeOffset.UtcNow));

        var act = () => handler.HandleAsync(
            new GenerateOfferCodesCommand(count, SubscriptionTier.Pro, 30),
            CancellationToken.None);

        await Assert.That(act).ThrowsAsync<ArgumentOutOfRangeException>();
    }

    [Test]
    public async Task Should_Throw_When_TierIsFree()
    {
        var handler = new GenerateOfferCodesCommandHandler(
            new FakeOfferCodeRepository(),
            new FakeOfferCodeGenerator(),
            new FakeClock(DateTimeOffset.UtcNow));

        var act = () => handler.HandleAsync(
            new GenerateOfferCodesCommand(1, SubscriptionTier.Free, 30),
            CancellationToken.None);

        await Assert.That(act).ThrowsAsync<ArgumentException>();
    }

    [Test]
    [Arguments(0)]
    [Arguments(366)]
    public async Task Should_Throw_When_DurationOutOfRange(int days)
    {
        var handler = new GenerateOfferCodesCommandHandler(
            new FakeOfferCodeRepository(),
            new FakeOfferCodeGenerator(),
            new FakeClock(DateTimeOffset.UtcNow));

        var act = () => handler.HandleAsync(
            new GenerateOfferCodesCommand(1, SubscriptionTier.Pro, days),
            CancellationToken.None);

        await Assert.That(act).ThrowsAsync<ArgumentOutOfRangeException>();
    }
}
```

**Note:** This test file references `FakeClock`. Check for an existing `FakeClock` fake in the application tests project. If present, reuse it. If not, add a minimal one in the same folder before step 4:

```csharp
// api/tests/town-crier.application.tests/OfferCodes/FakeClock.cs (only if not already present elsewhere)
namespace TownCrier.Application.Tests.OfferCodes;

internal sealed class FakeClock(DateTimeOffset now) : TimeProvider
{
    public override DateTimeOffset GetUtcNow() => now;
}
```

(If a repo-wide `FakeClock` exists, import it instead.)

- [ ] **Step 2: Run tests to verify failure**

```bash
cd api && dotnet test --filter "FullyQualifiedName~GenerateOfferCodesCommandHandlerTests" --no-restore 2>&1 | tail -20
```

Expected: missing types.

- [ ] **Step 3: Create command + result**

```csharp
// api/src/town-crier.application/OfferCodes/GenerateOfferCodesCommand.cs
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.OfferCodes;

public sealed record GenerateOfferCodesCommand(int Count, SubscriptionTier Tier, int DurationDays);
```

```csharp
// api/src/town-crier.application/OfferCodes/GenerateOfferCodesResult.cs
namespace TownCrier.Application.OfferCodes;

public sealed record GenerateOfferCodesResult(IReadOnlyList<string> Codes);
```

- [ ] **Step 4: Create handler**

```csharp
// api/src/town-crier.application/OfferCodes/GenerateOfferCodesCommandHandler.cs
using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.OfferCodes;

public sealed class GenerateOfferCodesCommandHandler
{
    private const int MaxCount = 1000;
    private const int MaxGenerationAttempts = 5;

    private readonly IOfferCodeRepository repository;
    private readonly IOfferCodeGenerator generator;
    private readonly TimeProvider timeProvider;

    public GenerateOfferCodesCommandHandler(
        IOfferCodeRepository repository,
        IOfferCodeGenerator generator,
        TimeProvider timeProvider)
    {
        this.repository = repository;
        this.generator = generator;
        this.timeProvider = timeProvider;
    }

    public async Task<GenerateOfferCodesResult> HandleAsync(
        GenerateOfferCodesCommand command,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        if (command.Count < 1 || command.Count > MaxCount)
        {
            throw new ArgumentOutOfRangeException(
                nameof(command),
                command.Count,
                $"Count must be between 1 and {MaxCount}.");
        }

        if (command.Tier == SubscriptionTier.Free)
        {
            throw new ArgumentException("Offer codes cannot grant the Free tier.", nameof(command));
        }

        if (command.DurationDays < 1 || command.DurationDays > 365)
        {
            throw new ArgumentOutOfRangeException(
                nameof(command),
                command.DurationDays,
                "DurationDays must be between 1 and 365.");
        }

        var createdAt = this.timeProvider.GetUtcNow();
        var codes = new List<string>(command.Count);

        for (var i = 0; i < command.Count; i++)
        {
            var code = await this.CreateUniqueCodeAsync(command.Tier, command.DurationDays, createdAt, ct)
                .ConfigureAwait(false);
            codes.Add(code.Code);
        }

        return new GenerateOfferCodesResult(codes);
    }

    private async Task<OfferCode> CreateUniqueCodeAsync(
        SubscriptionTier tier,
        int durationDays,
        DateTimeOffset createdAt,
        CancellationToken ct)
    {
        for (var attempt = 0; attempt < MaxGenerationAttempts; attempt++)
        {
            var canonical = this.generator.Generate();
            var offerCode = new OfferCode(canonical, tier, durationDays, createdAt);

            try
            {
                await this.repository.CreateAsync(offerCode, ct).ConfigureAwait(false);
                return offerCode;
            }
            catch (InvalidOperationException)
            {
                // Collision — try again.
            }
        }

        throw new InvalidOperationException(
            $"Could not generate a unique offer code after {MaxGenerationAttempts} attempts.");
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd api && dotnet test --filter "FullyQualifiedName~GenerateOfferCodesCommandHandlerTests" --no-restore 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.application/OfferCodes/GenerateOfferCodesCommand.cs \
        api/src/town-crier.application/OfferCodes/GenerateOfferCodesResult.cs \
        api/src/town-crier.application/OfferCodes/GenerateOfferCodesCommandHandler.cs \
        api/tests/town-crier.application.tests/OfferCodes/GenerateOfferCodesCommandHandlerTests.cs \
        api/tests/town-crier.application.tests/OfferCodes/FakeClock.cs
git commit -m "feat(application): add GenerateOfferCodesCommandHandler"
```

---

## Phase C — Application: Redemption

### Task C1: `RedeemOfferCodeCommandHandler`

**Files:**
- Create: `api/src/town-crier.application/OfferCodes/AlreadySubscribedException.cs`
- Create: `api/src/town-crier.application/OfferCodes/RedeemOfferCodeCommand.cs`
- Create: `api/src/town-crier.application/OfferCodes/RedeemOfferCodeResult.cs`
- Create: `api/src/town-crier.application/OfferCodes/RedeemOfferCodeCommandHandler.cs`
- Create: `api/tests/town-crier.application.tests/OfferCodes/RedeemOfferCodeCommandHandlerTests.cs`

- [ ] **Step 1: Write failing tests covering all error paths + happy path**

```csharp
// api/tests/town-crier.application.tests/OfferCodes/RedeemOfferCodeCommandHandlerTests.cs
using TownCrier.Application.Auth;
using TownCrier.Application.OfferCodes;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.OfferCodes;

public sealed class RedeemOfferCodeCommandHandlerTests
{
    private const string UserId = "auth0|user-1";

    [Test]
    public async Task Should_ActivateSubscription_When_CodeValidAndUserFree()
    {
        var (handler, codeRepo, profileRepo, auth0) = BuildHandlerWithCode(
            code: "A7KMZQR3FNXP",
            tier: SubscriptionTier.Pro,
            durationDays: 30);
        await profileRepo.SaveAsync(
            UserProfile.Register(UserId, "user@example.com"),
            CancellationToken.None);

        var result = await handler.HandleAsync(
            new RedeemOfferCodeCommand(UserId, "A7KM-ZQR3-FNXP"),
            CancellationToken.None);

        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(result.ExpiresAt).IsEqualTo(new DateTimeOffset(2026, 5, 18, 12, 0, 0, TimeSpan.Zero));
        await Assert.That(profileRepo.GetByUserId(UserId)!.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(codeRepo.All.Single().IsRedeemed).IsTrue();
        await Assert.That(auth0.Updates).HasCount().EqualTo(1);
        await Assert.That(auth0.Updates[0].Tier).IsEqualTo("Pro");
    }

    [Test]
    public async Task Should_Throw_When_CodeFormatInvalid()
    {
        var (handler, _, _, _) = BuildHandlerWithCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30);

        var act = () => handler.HandleAsync(
            new RedeemOfferCodeCommand(UserId, "not-a-code"),
            CancellationToken.None);

        await Assert.That(act).ThrowsAsync<InvalidOfferCodeFormatException>();
    }

    [Test]
    public async Task Should_Throw_When_CodeNotFound()
    {
        var (handler, _, profileRepo, _) = BuildHandlerWithCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30);
        await profileRepo.SaveAsync(
            UserProfile.Register(UserId, "user@example.com"),
            CancellationToken.None);

        var act = () => handler.HandleAsync(
            new RedeemOfferCodeCommand(UserId, "BBBBBBBBBBBB"),
            CancellationToken.None);

        await Assert.That(act).ThrowsAsync<OfferCodeNotFoundException>();
    }

    [Test]
    public async Task Should_Throw_When_CodeAlreadyRedeemed()
    {
        var (handler, codeRepo, profileRepo, _) = BuildHandlerWithCode(
            "A7KMZQR3FNXP", SubscriptionTier.Pro, 30);
        await profileRepo.SaveAsync(UserProfile.Register(UserId, "user@example.com"), CancellationToken.None);

        var existing = codeRepo.All.Single();
        existing.Redeem("auth0|other-user", DateTimeOffset.UtcNow);
        await codeRepo.SaveAsync(existing, CancellationToken.None);

        var act = () => handler.HandleAsync(
            new RedeemOfferCodeCommand(UserId, "A7KMZQR3FNXP"),
            CancellationToken.None);

        await Assert.That(act).ThrowsAsync<OfferCodeAlreadyRedeemedException>();
    }

    [Test]
    public async Task Should_Throw_When_UserAlreadySubscribed()
    {
        var (handler, _, profileRepo, _) = BuildHandlerWithCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30);

        var profile = UserProfile.Register(UserId, "user@example.com");
        profile.ActivateSubscription(SubscriptionTier.Personal, DateTimeOffset.UtcNow.AddDays(30));
        await profileRepo.SaveAsync(profile, CancellationToken.None);

        var act = () => handler.HandleAsync(
            new RedeemOfferCodeCommand(UserId, "A7KMZQR3FNXP"),
            CancellationToken.None);

        await Assert.That(act).ThrowsAsync<AlreadySubscribedException>();
    }

    [Test]
    public async Task Should_Throw_When_UserNotFound()
    {
        var (handler, _, _, _) = BuildHandlerWithCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30);

        var act = () => handler.HandleAsync(
            new RedeemOfferCodeCommand("auth0|missing", "A7KMZQR3FNXP"),
            CancellationToken.None);

        await Assert.That(act).ThrowsAsync<UserProfileNotFoundException>();
    }

    private static (
        RedeemOfferCodeCommandHandler Handler,
        FakeOfferCodeRepository CodeRepo,
        FakeUserProfileRepository ProfileRepo,
        FakeAuth0ManagementClient Auth0) BuildHandlerWithCode(
            string code,
            SubscriptionTier tier,
            int durationDays)
    {
        var codeRepo = new FakeOfferCodeRepository();
        var offerCode = new OfferCode(code, tier, durationDays,
            new DateTimeOffset(2026, 4, 1, 0, 0, 0, TimeSpan.Zero));
        codeRepo.CreateAsync(offerCode, CancellationToken.None).GetAwaiter().GetResult();

        var profileRepo = new FakeUserProfileRepository();
        var auth0 = new FakeAuth0ManagementClient();
        var clock = new FakeClock(new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));

        var handler = new RedeemOfferCodeCommandHandler(codeRepo, profileRepo, auth0, clock);
        return (handler, codeRepo, profileRepo, auth0);
    }
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd api && dotnet test --filter "FullyQualifiedName~RedeemOfferCodeCommandHandlerTests" --no-restore 2>&1 | tail -20
```

- [ ] **Step 3: Create exception + command + result**

```csharp
// api/src/town-crier.application/OfferCodes/AlreadySubscribedException.cs
namespace TownCrier.Application.OfferCodes;

public sealed class AlreadySubscribedException : Exception
{
    public AlreadySubscribedException()
        : base("User already has an active subscription; offer codes are only available to free-tier users.")
    {
    }
}
```

```csharp
// api/src/town-crier.application/OfferCodes/RedeemOfferCodeCommand.cs
namespace TownCrier.Application.OfferCodes;

public sealed record RedeemOfferCodeCommand(string UserId, string Code);
```

```csharp
// api/src/town-crier.application/OfferCodes/RedeemOfferCodeResult.cs
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.OfferCodes;

public sealed record RedeemOfferCodeResult(SubscriptionTier Tier, DateTimeOffset ExpiresAt);
```

- [ ] **Step 4: Create handler**

```csharp
// api/src/town-crier.application/OfferCodes/RedeemOfferCodeCommandHandler.cs
using TownCrier.Application.Auth;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.OfferCodes;

public sealed class RedeemOfferCodeCommandHandler
{
    private readonly IOfferCodeRepository codeRepository;
    private readonly IUserProfileRepository profileRepository;
    private readonly IAuth0ManagementClient auth0Client;
    private readonly TimeProvider timeProvider;

    public RedeemOfferCodeCommandHandler(
        IOfferCodeRepository codeRepository,
        IUserProfileRepository profileRepository,
        IAuth0ManagementClient auth0Client,
        TimeProvider timeProvider)
    {
        this.codeRepository = codeRepository;
        this.profileRepository = profileRepository;
        this.auth0Client = auth0Client;
        this.timeProvider = timeProvider;
    }

    public async Task<RedeemOfferCodeResult> HandleAsync(
        RedeemOfferCodeCommand command,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var canonical = OfferCodeFormat.Normalize(command.Code);

        var code = await this.codeRepository.GetAsync(canonical, ct).ConfigureAwait(false)
            ?? throw new OfferCodeNotFoundException(canonical);

        if (code.IsRedeemed)
        {
            throw new OfferCodeAlreadyRedeemedException(canonical);
        }

        var profile = await this.profileRepository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false)
            ?? throw new UserProfileNotFoundException($"No user profile found for userId '{command.UserId}'.");

        if (profile.Tier != SubscriptionTier.Free)
        {
            throw new AlreadySubscribedException();
        }

        var now = this.timeProvider.GetUtcNow();
        code.Redeem(command.UserId, now);
        profile.ActivateSubscription(code.Tier, now.AddDays(code.DurationDays));

        await this.codeRepository.SaveAsync(code, ct).ConfigureAwait(false);
        await this.profileRepository.SaveAsync(profile, ct).ConfigureAwait(false);
        await this.auth0Client.UpdateSubscriptionTierAsync(profile.UserId, profile.Tier.ToString(), ct)
            .ConfigureAwait(false);

        return new RedeemOfferCodeResult(profile.Tier, profile.SubscriptionExpiry!.Value);
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd api && dotnet test --filter "FullyQualifiedName~RedeemOfferCodeCommandHandlerTests" --no-restore 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.application/OfferCodes/AlreadySubscribedException.cs \
        api/src/town-crier.application/OfferCodes/RedeemOfferCodeCommand.cs \
        api/src/town-crier.application/OfferCodes/RedeemOfferCodeResult.cs \
        api/src/town-crier.application/OfferCodes/RedeemOfferCodeCommandHandler.cs \
        api/tests/town-crier.application.tests/OfferCodes/RedeemOfferCodeCommandHandlerTests.cs
git commit -m "feat(application): add RedeemOfferCodeCommandHandler"
```

---

## Phase D — Infrastructure

### Task D1: `CosmosOfferCodeRepository`

**Files:**
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosContainerNames.cs`
- Create: `api/src/town-crier.infrastructure/OfferCodes/OfferCodeDocument.cs`
- Create: `api/src/town-crier.infrastructure/OfferCodes/CosmosOfferCodeRepository.cs`
- Create: `api/src/town-crier.infrastructure/OfferCodes/InMemoryOfferCodeRepository.cs`
- Modify: `api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs`
- Create: `api/tests/town-crier.infrastructure.tests/OfferCodes/CosmosOfferCodeRepositoryTests.cs`

- [ ] **Step 1: Add container name constant**

Edit `api/src/town-crier.infrastructure/Cosmos/CosmosContainerNames.cs` — add after the existing constants:

```csharp
public const string OfferCodes = "OfferCodes";
```

- [ ] **Step 2: Create the Cosmos document**

```csharp
// api/src/town-crier.infrastructure/OfferCodes/OfferCodeDocument.cs
using System.Text.Json.Serialization;
using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Infrastructure.OfferCodes;

public sealed class OfferCodeDocument
{
    [JsonPropertyName("id")]
    public required string Id { get; init; }

    [JsonPropertyName("code")]
    public required string Code { get; init; }

    [JsonPropertyName("tier")]
    public required string Tier { get; init; }

    [JsonPropertyName("durationDays")]
    public required int DurationDays { get; init; }

    [JsonPropertyName("createdAt")]
    public required DateTimeOffset CreatedAt { get; init; }

    [JsonPropertyName("redeemedByUserId")]
    public string? RedeemedByUserId { get; init; }

    [JsonPropertyName("redeemedAt")]
    public DateTimeOffset? RedeemedAt { get; init; }

    public static OfferCodeDocument FromDomain(OfferCode code) => new()
    {
        Id = code.Code,
        Code = code.Code,
        Tier = code.Tier.ToString(),
        DurationDays = code.DurationDays,
        CreatedAt = code.CreatedAt,
        RedeemedByUserId = code.RedeemedByUserId,
        RedeemedAt = code.RedeemedAt,
    };

    public OfferCode ToDomain() => new(
        code: this.Code,
        tier: Enum.Parse<SubscriptionTier>(this.Tier),
        durationDays: this.DurationDays,
        createdAt: this.CreatedAt,
        redeemedByUserId: this.RedeemedByUserId,
        redeemedAt: this.RedeemedAt);
}
```

- [ ] **Step 3: Register the document in `CosmosJsonSerializerContext`**

Locate `CosmosJsonSerializerContext.cs` and add `OfferCodeDocument` to the `[JsonSerializable]` attributes. If you can't find it, run:

```bash
cd api && grep -r "class CosmosJsonSerializerContext" --include="*.cs" -l
```

Add:

```csharp
[JsonSerializable(typeof(OfferCodeDocument))]
[JsonSerializable(typeof(System.Collections.Generic.List<OfferCodeDocument>))]
```

…along with the matching `using TownCrier.Infrastructure.OfferCodes;`.

- [ ] **Step 4: Create the repository**

```csharp
// api/src/town-crier.infrastructure/OfferCodes/CosmosOfferCodeRepository.cs
using TownCrier.Application.OfferCodes;
using TownCrier.Domain.OfferCodes;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.OfferCodes;

public sealed class CosmosOfferCodeRepository : IOfferCodeRepository
{
    private readonly ICosmosRestClient client;

    public CosmosOfferCodeRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<OfferCode?> GetAsync(string canonicalCode, CancellationToken ct)
    {
        var document = await this.client.ReadDocumentAsync(
            CosmosContainerNames.OfferCodes,
            canonicalCode,
            canonicalCode,
            CosmosJsonSerializerContext.Default.OfferCodeDocument,
            ct).ConfigureAwait(false);

        return document?.ToDomain();
    }

    public async Task CreateAsync(OfferCode code, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(code);

        var document = OfferCodeDocument.FromDomain(code);
        // Note: UpsertDocumentAsync accepts overwrites. The handler relies on collision detection via
        // post-read. Cosmos returns a 409 on CreateDocumentAsync if one is available; if not, the
        // caller (GenerateOfferCodesCommandHandler) treats duplicate-key as a retry signal.
        // If ICosmosRestClient lacks Create semantics, use upsert and treat as best-effort —
        // document that caveat in the test.
        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.OfferCodes,
            document,
            document.Id,
            CosmosJsonSerializerContext.Default.OfferCodeDocument,
            ct).ConfigureAwait(false);
    }

    public async Task SaveAsync(OfferCode code, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(code);

        var document = OfferCodeDocument.FromDomain(code);
        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.OfferCodes,
            document,
            document.Id,
            CosmosJsonSerializerContext.Default.OfferCodeDocument,
            ct).ConfigureAwait(false);
    }
}
```

**Note on `CreateAsync` vs collision detection:** the existing `ICosmosRestClient.UpsertDocumentAsync` does not return a distinct "already exists" signal. If that's the case after reading `CosmosRestClient.cs`, adjust the generator handler: before `CreateAsync`, call `GetAsync` for the same code and retry if it returns non-null. Update `GenerateOfferCodesCommandHandler` (Task B4) accordingly if needed, and mention the adjustment in the commit message. Given 60-bit entropy, collision is astronomically unlikely; the check is paranoia.

- [ ] **Step 5: Create in-memory impl**

```csharp
// api/src/town-crier.infrastructure/OfferCodes/InMemoryOfferCodeRepository.cs
using System.Collections.Concurrent;
using TownCrier.Application.OfferCodes;
using TownCrier.Domain.OfferCodes;

namespace TownCrier.Infrastructure.OfferCodes;

public sealed class InMemoryOfferCodeRepository : IOfferCodeRepository
{
    private readonly ConcurrentDictionary<string, OfferCode> store = new(StringComparer.Ordinal);

    public Task<OfferCode?> GetAsync(string canonicalCode, CancellationToken ct)
    {
        this.store.TryGetValue(canonicalCode, out var code);
        return Task.FromResult(code);
    }

    public Task CreateAsync(OfferCode code, CancellationToken ct)
    {
        if (!this.store.TryAdd(code.Code, code))
        {
            throw new InvalidOperationException($"Offer code '{code.Code}' already exists.");
        }

        return Task.CompletedTask;
    }

    public Task SaveAsync(OfferCode code, CancellationToken ct)
    {
        this.store[code.Code] = code;
        return Task.CompletedTask;
    }
}
```

- [ ] **Step 6: Write integration test**

```csharp
// api/tests/town-crier.infrastructure.tests/OfferCodes/CosmosOfferCodeRepositoryTests.cs
using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.OfferCodes;

namespace TownCrier.Infrastructure.Tests.OfferCodes;

// Follows the existing pattern — Cosmos emulator fixture used by other repo tests in this project.
// If the project uses a different fixture name, mirror it (e.g. CosmosDbEmulatorFixture).
[ClassDataSource<CosmosEmulatorFixture>(Shared = SharedType.PerAssembly)]
public sealed class CosmosOfferCodeRepositoryTests
{
    private readonly CosmosEmulatorFixture fixture;

    public CosmosOfferCodeRepositoryTests(CosmosEmulatorFixture fixture)
    {
        this.fixture = fixture;
    }

    [Test]
    public async Task Should_RoundTrip_OfferCode()
    {
        var repository = new CosmosOfferCodeRepository(this.fixture.Client);
        var code = new OfferCode(
            "A7KMZQR3FNXP",
            SubscriptionTier.Pro,
            30,
            new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));

        await repository.CreateAsync(code, TestContext.Current!.CancellationToken);

        var fetched = await repository.GetAsync("A7KMZQR3FNXP", TestContext.Current.CancellationToken);
        await Assert.That(fetched).IsNotNull();
        await Assert.That(fetched!.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(fetched.IsRedeemed).IsFalse();
    }

    [Test]
    public async Task Should_PersistRedeemedState()
    {
        var repository = new CosmosOfferCodeRepository(this.fixture.Client);
        var code = new OfferCode("BBBBBBBBBBBB", SubscriptionTier.Personal, 14, DateTimeOffset.UtcNow);
        await repository.CreateAsync(code, TestContext.Current!.CancellationToken);

        code.Redeem("auth0|user-99", DateTimeOffset.UtcNow);
        await repository.SaveAsync(code, TestContext.Current.CancellationToken);

        var fetched = await repository.GetAsync("BBBBBBBBBBBB", TestContext.Current.CancellationToken);
        await Assert.That(fetched!.IsRedeemed).IsTrue();
        await Assert.That(fetched.RedeemedByUserId).IsEqualTo("auth0|user-99");
    }
}
```

**Note:** inspect the existing `CosmosUserProfileRepositoryTests` or similar file to get the exact fixture class name and DI wiring. Mirror it exactly; don't invent a new pattern.

- [ ] **Step 7: Run tests**

```bash
cd api && dotnet test api/tests/town-crier.infrastructure.tests --filter "FullyQualifiedName~CosmosOfferCodeRepositoryTests" --no-restore 2>&1 | tail -15
```

Expected: pass against the emulator. If the emulator isn't running locally, skip per the project's existing integration-test convention (these tests may only run in CI).

- [ ] **Step 8: Commit**

```bash
git add api/src/town-crier.infrastructure/Cosmos/CosmosContainerNames.cs \
        api/src/town-crier.infrastructure/Cosmos/CosmosJsonSerializerContext.cs \
        api/src/town-crier.infrastructure/OfferCodes/ \
        api/tests/town-crier.infrastructure.tests/OfferCodes/
git commit -m "feat(infrastructure): add CosmosOfferCodeRepository and in-memory fallback"
```

---

## Phase E — Web Layer

### Task E1: Request/Response DTOs + JSON context

**Files:**
- Create: `api/src/town-crier.web/Endpoints/OfferCodeRequests.cs`
- Modify: `api/src/town-crier.web/AppJsonSerializerContext.cs`

- [ ] **Step 1: Create DTO file**

```csharp
// api/src/town-crier.web/Endpoints/OfferCodeRequests.cs
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Web.Endpoints;

public sealed record GenerateOfferCodesRequest(int Count, SubscriptionTier Tier, int DurationDays);

public sealed record RedeemOfferCodeRequest(string Code);

public sealed record RedeemOfferCodeResponse(string Tier, DateTimeOffset ExpiresAt);
```

- [ ] **Step 2: Register in `AppJsonSerializerContext`**

Open `api/src/town-crier.web/AppJsonSerializerContext.cs` and add:

```csharp
[JsonSerializable(typeof(GenerateOfferCodesRequest))]
[JsonSerializable(typeof(RedeemOfferCodeRequest))]
[JsonSerializable(typeof(RedeemOfferCodeResponse))]
```

…along with `using TownCrier.Web.Endpoints;`.

- [ ] **Step 3: Build to confirm**

```bash
cd api && dotnet build --no-restore 2>&1 | tail -10
```

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.web/Endpoints/OfferCodeRequests.cs \
        api/src/town-crier.web/AppJsonSerializerContext.cs
git commit -m "feat(web): add offer code DTOs and register serializer context"
```

---

### Task E2: Admin generate endpoint + test

**Files:**
- Modify: `api/src/town-crier.web/Endpoints/AdminEndpoints.cs`
- Create: `api/tests/town-crier.web.tests/OfferCodes/GenerateOfferCodesEndpointTests.cs`

- [ ] **Step 1: Write failing endpoint test**

Look at an existing web test (e.g. `api/tests/town-crier.web.tests/Observability/ServerRequestTracingTests.cs`) to see the `WebApplicationFactory` pattern used. Model the new test on it.

```csharp
// api/tests/town-crier.web.tests/OfferCodes/GenerateOfferCodesEndpointTests.cs
using System.Net;
using System.Net.Http.Json;
using TownCrier.Domain.UserProfiles;
using TownCrier.Web.Endpoints;

namespace TownCrier.Web.Tests.OfferCodes;

public sealed class GenerateOfferCodesEndpointTests
{
    [Test]
    public async Task Should_Return401_When_AdminKeyMissing()
    {
        await using var factory = new OfferCodeWebApplicationFactory();
        var client = factory.CreateClient();

        var response = await client.PostAsJsonAsync(
            "/v1/admin/offer-codes",
            new GenerateOfferCodesRequest(1, SubscriptionTier.Pro, 30),
            AppJsonSerializerContext.Default.GenerateOfferCodesRequest);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_Return200_WithPlainTextCodes_When_Valid()
    {
        await using var factory = new OfferCodeWebApplicationFactory();
        var client = factory.CreateClient();
        client.DefaultRequestHeaders.Add("X-Admin-Key", factory.AdminKey);

        var response = await client.PostAsJsonAsync(
            "/v1/admin/offer-codes",
            new GenerateOfferCodesRequest(3, SubscriptionTier.Pro, 30),
            AppJsonSerializerContext.Default.GenerateOfferCodesRequest);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(response.Content.Headers.ContentType!.MediaType).IsEqualTo("text/plain");

        var body = await response.Content.ReadAsStringAsync();
        var lines = body.Split('\n', StringSplitOptions.RemoveEmptyEntries);
        await Assert.That(lines).HasCount().EqualTo(3);
        // Every line matches XXXX-XXXX-XXXX
        foreach (var line in lines)
        {
            await Assert.That(System.Text.RegularExpressions.Regex.IsMatch(line, "^[0-9A-Z]{4}-[0-9A-Z]{4}-[0-9A-Z]{4}$")).IsTrue();
        }
    }

    [Test]
    public async Task Should_Return400_When_CountOutOfRange()
    {
        await using var factory = new OfferCodeWebApplicationFactory();
        var client = factory.CreateClient();
        client.DefaultRequestHeaders.Add("X-Admin-Key", factory.AdminKey);

        var response = await client.PostAsJsonAsync(
            "/v1/admin/offer-codes",
            new GenerateOfferCodesRequest(0, SubscriptionTier.Pro, 30),
            AppJsonSerializerContext.Default.GenerateOfferCodesRequest);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.BadRequest);
    }
}
```

**Note:** `OfferCodeWebApplicationFactory` is a test factory that wires up the in-memory repositories and fake Auth0 client and sets a known admin key. Mirror the existing test factory pattern used in web tests (look for something like `TestWebApplicationFactory` or similar). If no pattern exists, create a minimal one in the same folder exposing an `AdminKey` constant and overriding `IOfferCodeRepository` → `InMemoryOfferCodeRepository`, `IAuth0ManagementClient` → `FakeAuth0ManagementClient`.

- [ ] **Step 2: Run to confirm failure**

```bash
cd api && dotnet test --filter "FullyQualifiedName~GenerateOfferCodesEndpointTests" --no-restore 2>&1 | tail -20
```

- [ ] **Step 3: Add the endpoint**

Edit `api/src/town-crier.web/Endpoints/AdminEndpoints.cs` — after the existing `admin.MapGet("/users", ...)` block, add:

```csharp
admin.MapPost("/offer-codes", async (
    GenerateOfferCodesRequest request,
    GenerateOfferCodesCommandHandler handler,
    CancellationToken ct) =>
{
    try
    {
        var result = await handler.HandleAsync(
            new GenerateOfferCodesCommand(request.Count, request.Tier, request.DurationDays),
            ct).ConfigureAwait(false);

        var body = string.Join('\n',
            result.Codes.Select(OfferCodeFormat.Format)) + "\n";
        return Results.Text(body, contentType: "text/plain");
    }
    catch (ArgumentOutOfRangeException ex)
    {
        return Results.BadRequest(new { error = "invalid_argument", message = ex.Message });
    }
    catch (ArgumentException ex)
    {
        return Results.BadRequest(new { error = "invalid_argument", message = ex.Message });
    }
});
```

Add the required `using` at the top: `using TownCrier.Application.OfferCodes;`.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd api && dotnet test --filter "FullyQualifiedName~GenerateOfferCodesEndpointTests" --no-restore 2>&1 | tail -10
```

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.web/Endpoints/AdminEndpoints.cs \
        api/tests/town-crier.web.tests/OfferCodes/
git commit -m "feat(web): add admin POST /v1/admin/offer-codes endpoint"
```

---

### Task E3: User redeem endpoint + test

**Files:**
- Create: `api/src/town-crier.web/Endpoints/OfferCodeEndpoints.cs`
- Modify: `api/src/town-crier.web/Extensions/WebApplicationExtensions.cs`
- Create: `api/tests/town-crier.web.tests/OfferCodes/RedeemOfferCodeEndpointTests.cs`

- [ ] **Step 1: Write failing endpoint test**

```csharp
// api/tests/town-crier.web.tests/OfferCodes/RedeemOfferCodeEndpointTests.cs
using System.Net;
using System.Net.Http.Json;
using System.Text.Json;
using TownCrier.Web.Endpoints;

namespace TownCrier.Web.Tests.OfferCodes;

public sealed class RedeemOfferCodeEndpointTests
{
    [Test]
    public async Task Should_Return200_WithTierAndExpiry_When_CodeValid()
    {
        await using var factory = new OfferCodeWebApplicationFactory();
        await factory.SeedCodeAsync("A7KMZQR3FNXP", SubscriptionTier.Pro, 30);
        await factory.SeedUserAsync("auth0|user-1", "user@example.com", SubscriptionTier.Free);
        var client = factory.CreateAuthedClient("auth0|user-1");

        var response = await client.PostAsJsonAsync(
            "/v1/offer-codes/redeem",
            new RedeemOfferCodeRequest("A7KM-ZQR3-FNXP"),
            AppJsonSerializerContext.Default.RedeemOfferCodeRequest);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        var body = await response.Content.ReadFromJsonAsync<RedeemOfferCodeResponse>(
            AppJsonSerializerContext.Default.RedeemOfferCodeResponse);
        await Assert.That(body!.Tier).IsEqualTo("Pro");
    }

    [Test]
    public async Task Should_Return400_invalid_code_format_When_CodeMalformed()
    {
        await using var factory = new OfferCodeWebApplicationFactory();
        await factory.SeedUserAsync("auth0|user-1", "user@example.com", SubscriptionTier.Free);
        var client = factory.CreateAuthedClient("auth0|user-1");

        var response = await client.PostAsJsonAsync(
            "/v1/offer-codes/redeem",
            new RedeemOfferCodeRequest("nonsense"),
            AppJsonSerializerContext.Default.RedeemOfferCodeRequest);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.BadRequest);
        await AssertErrorCode(response, "invalid_code_format");
    }

    [Test]
    public async Task Should_Return404_invalid_code_When_CodeNotFound()
    {
        await using var factory = new OfferCodeWebApplicationFactory();
        await factory.SeedUserAsync("auth0|user-1", "user@example.com", SubscriptionTier.Free);
        var client = factory.CreateAuthedClient("auth0|user-1");

        var response = await client.PostAsJsonAsync(
            "/v1/offer-codes/redeem",
            new RedeemOfferCodeRequest("BBBBBBBBBBBB"),
            AppJsonSerializerContext.Default.RedeemOfferCodeRequest);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.NotFound);
        await AssertErrorCode(response, "invalid_code");
    }

    [Test]
    public async Task Should_Return409_code_already_redeemed_When_CodeAlreadyUsed()
    {
        await using var factory = new OfferCodeWebApplicationFactory();
        await factory.SeedCodeAsync("A7KMZQR3FNXP", SubscriptionTier.Pro, 30);
        await factory.SeedUserAsync("auth0|user-1", "user@example.com", SubscriptionTier.Free);
        await factory.SeedRedeemedAsync("A7KMZQR3FNXP", "auth0|previous");
        var client = factory.CreateAuthedClient("auth0|user-1");

        var response = await client.PostAsJsonAsync(
            "/v1/offer-codes/redeem",
            new RedeemOfferCodeRequest("A7KMZQR3FNXP"),
            AppJsonSerializerContext.Default.RedeemOfferCodeRequest);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Conflict);
        await AssertErrorCode(response, "code_already_redeemed");
    }

    [Test]
    public async Task Should_Return409_already_subscribed_When_UserHasPaidTier()
    {
        await using var factory = new OfferCodeWebApplicationFactory();
        await factory.SeedCodeAsync("A7KMZQR3FNXP", SubscriptionTier.Pro, 30);
        await factory.SeedUserAsync("auth0|user-1", "user@example.com", SubscriptionTier.Personal);
        var client = factory.CreateAuthedClient("auth0|user-1");

        var response = await client.PostAsJsonAsync(
            "/v1/offer-codes/redeem",
            new RedeemOfferCodeRequest("A7KMZQR3FNXP"),
            AppJsonSerializerContext.Default.RedeemOfferCodeRequest);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Conflict);
        await AssertErrorCode(response, "already_subscribed");
    }

    private static async Task AssertErrorCode(HttpResponseMessage response, string expected)
    {
        using var stream = await response.Content.ReadAsStreamAsync();
        using var doc = await JsonDocument.ParseAsync(stream);
        await Assert.That(doc.RootElement.GetProperty("error").GetString()).IsEqualTo(expected);
    }
}
```

**Note:** `CreateAuthedClient` issues a test JWT with `sub = userId`. Match whatever existing web tests do for authenticated endpoints — don't invent a new auth shim.

- [ ] **Step 2: Run to confirm failure**

```bash
cd api && dotnet test --filter "FullyQualifiedName~RedeemOfferCodeEndpointTests" --no-restore 2>&1 | tail -20
```

- [ ] **Step 3: Create the endpoint file**

```csharp
// api/src/town-crier.web/Endpoints/OfferCodeEndpoints.cs
using System.Security.Claims;
using TownCrier.Application.OfferCodes;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.OfferCodes;

namespace TownCrier.Web.Endpoints;

internal static class OfferCodeEndpoints
{
    public static void MapOfferCodeEndpoints(this RouteGroupBuilder group)
    {
        var offerCodes = group.MapGroup("/offer-codes").RequireAuthorization();

        offerCodes.MapPost("/redeem", async (
            RedeemOfferCodeRequest request,
            ClaimsPrincipal user,
            RedeemOfferCodeCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue(ClaimTypes.NameIdentifier)
                ?? user.FindFirstValue("sub")
                ?? throw new UnauthorizedAccessException();

            try
            {
                var result = await handler.HandleAsync(
                    new RedeemOfferCodeCommand(userId, request.Code),
                    ct).ConfigureAwait(false);

                return Results.Ok(new RedeemOfferCodeResponse(result.Tier.ToString(), result.ExpiresAt));
            }
            catch (InvalidOfferCodeFormatException ex)
            {
                return Results.BadRequest(new { error = "invalid_code_format", message = ex.Message });
            }
            catch (OfferCodeNotFoundException)
            {
                return Results.NotFound(new { error = "invalid_code", message = "This code isn't valid." });
            }
            catch (OfferCodeAlreadyRedeemedException)
            {
                return Results.Conflict(new { error = "code_already_redeemed", message = "This code has already been used." });
            }
            catch (AlreadySubscribedException ex)
            {
                return Results.Conflict(new { error = "already_subscribed", message = ex.Message });
            }
        });
    }
}
```

- [ ] **Step 4: Register the group**

Edit `api/src/town-crier.web/Extensions/WebApplicationExtensions.cs` — add inside `MapAllEndpoints`, after the other `v1.Map...` calls:

```csharp
v1.MapOfferCodeEndpoints();
```

- [ ] **Step 5: Run tests**

```bash
cd api && dotnet test --filter "FullyQualifiedName~RedeemOfferCodeEndpointTests" --no-restore 2>&1 | tail -10
```

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.web/Endpoints/OfferCodeEndpoints.cs \
        api/src/town-crier.web/Extensions/WebApplicationExtensions.cs \
        api/tests/town-crier.web.tests/OfferCodes/RedeemOfferCodeEndpointTests.cs
git commit -m "feat(web): add POST /v1/offer-codes/redeem endpoint with error mapping"
```

---

### Task E4: DI registration

**Files:**
- Modify: `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`

- [ ] **Step 1: Add registrations**

Open `ServiceCollectionExtensions.cs` and locate where other handlers + repositories are registered (look for `GrantSubscriptionCommandHandler` or `IUserProfileRepository` registrations). Alongside them, add:

```csharp
services.AddSingleton<IOfferCodeGenerator, OfferCodeGenerator>();
services.AddSingleton<IOfferCodeRepository, CosmosOfferCodeRepository>();
services.AddTransient<GenerateOfferCodesCommandHandler>();
services.AddTransient<RedeemOfferCodeCommandHandler>();
services.AddSingleton(TimeProvider.System);   // only if not already registered — check first
```

Required `using` statements:

```csharp
using TownCrier.Application.OfferCodes;
using TownCrier.Infrastructure.OfferCodes;
```

- [ ] **Step 2: Build + run full test suite**

```bash
cd api && dotnet build --no-restore 2>&1 | tail -10 && dotnet test --no-restore --no-build 2>&1 | tail -10
```

Expected: clean build + all tests pass.

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs
git commit -m "feat(web): register offer code handlers and repository in DI"
```

---

## Phase F — Infrastructure (Pulumi)

### Task F1: Add `OfferCodes` container

**Files:**
- Modify: `infra/EnvironmentStack.cs`

- [ ] **Step 1: Add container definition**

In `EnvironmentStack.cs`, locate the `containerDefinitions` array (around line 94). Append a new entry before the closing `};`:

```csharp
// OfferCodes — partitioned by code for point reads on redemption
new("OfferCodes", "/code"),
```

- [ ] **Step 2: Build**

```bash
cd infra && dotnet build --no-restore 2>&1 | tail -10
```

Expected: clean build.

- [ ] **Step 3: Verify via preview (optional, requires Pulumi login)**

```bash
cd infra && pulumi preview --stack dev 2>&1 | grep -i offercodes
```

Expected: one new `SqlResourceSqlContainer` marked `+ create`.

- [ ] **Step 4: Commit**

```bash
git add infra/EnvironmentStack.cs
git commit -m "feat(infra): provision OfferCodes Cosmos container"
```

---

## Phase G — CLI

### Task G1: `tc generate-offer-codes`

**Files:**
- Create: `cli/src/tc/Commands/GenerateOfferCodesCommand.cs`
- Modify: `cli/src/tc/Json/TcJsonContext.cs`
- Modify: `cli/src/tc/Program.cs`
- Create: `cli/tests/tc.tests/GenerateOfferCodesCommandTests.cs` (if a CLI test project exists; otherwise skip tests and rely on manual verification)

- [ ] **Step 1: Add DTO to JSON context**

In `cli/src/tc/Json/TcJsonContext.cs`, add:

```csharp
[JsonSerializable(typeof(GenerateOfferCodesRequest))]
```

Add the record if it's not imported:

```csharp
// cli/src/tc/Json/GenerateOfferCodesRequest.cs (or wherever existing request DTOs live)
namespace Tc.Json;

public sealed record GenerateOfferCodesRequest
{
    public int Count { get; init; }
    public string Tier { get; init; } = string.Empty;
    public int DurationDays { get; init; }
}
```

- [ ] **Step 2: Create the command**

```csharp
// cli/src/tc/Commands/GenerateOfferCodesCommand.cs
using Tc.Json;

namespace Tc.Commands;

internal static class GenerateOfferCodesCommand
{
    private static readonly HashSet<string> ValidTiers = new(StringComparer.OrdinalIgnoreCase)
    {
        "Personal", "Pro",
    };

    public static async Task<int> RunAsync(ApiClient client, ParsedArgs args, CancellationToken ct)
    {
        string countRaw, tierRaw, durationRaw;
        try
        {
            countRaw = args.GetRequired("count");
            tierRaw = args.GetRequired("tier");
            durationRaw = args.GetRequired("duration-days");
        }
        catch (ArgumentException ex)
        {
            await Console.Error.WriteLineAsync($"Missing argument: {ex.Message}").ConfigureAwait(false);
            await Console.Error.WriteLineAsync(
                "Usage: tc generate-offer-codes --count <N> --tier <Personal|Pro> --duration-days <D>")
                .ConfigureAwait(false);
            return 1;
        }

        if (!int.TryParse(countRaw, out var count) || count < 1 || count > 1000)
        {
            await Console.Error.WriteLineAsync("Invalid --count: must be 1..1000.").ConfigureAwait(false);
            return 1;
        }

        if (!ValidTiers.Contains(tierRaw))
        {
            await Console.Error.WriteLineAsync($"Invalid --tier: {tierRaw}. Must be Personal or Pro.").ConfigureAwait(false);
            return 1;
        }

        if (!int.TryParse(durationRaw, out var duration) || duration < 1 || duration > 365)
        {
            await Console.Error.WriteLineAsync("Invalid --duration-days: must be 1..365.").ConfigureAwait(false);
            return 1;
        }

        var normalizedTier = ValidTiers.First(t => string.Equals(t, tierRaw, StringComparison.OrdinalIgnoreCase));

        var request = new GenerateOfferCodesRequest
        {
            Count = count,
            Tier = normalizedTier,
            DurationDays = duration,
        };

        var response = await client.PostAsJsonAsync(
            "/v1/admin/offer-codes",
            request,
            TcJsonContext.Default.GenerateOfferCodesRequest,
            ct).ConfigureAwait(false);

        if (!response.IsSuccessStatusCode)
        {
            var body = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
            await Console.Error.WriteLineAsync($"API error ({(int)response.StatusCode}): {body}")
                .ConfigureAwait(false);
            return 2;
        }

        var codes = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
        await Console.Out.WriteAsync(codes).ConfigureAwait(false);
        await Console.Error.WriteLineAsync(
            $"Generated {count} codes: {normalizedTier} tier, {duration} days duration")
            .ConfigureAwait(false);
        return 0;
    }
}
```

- [ ] **Step 3: Dispatch in `Program.cs`**

Edit `cli/src/tc/Program.cs` — add to the switch:

```csharp
"generate-offer-codes" => await GenerateOfferCodesCommand.RunAsync(client, parsed, cts.Token).ConfigureAwait(false),
```

And extend the help text:

```csharp
// In PrintHelpAsync(), add a line under Commands:
//   generate-offer-codes Generate N single-use codes of a given tier + duration
// And add an options block:
// generate-offer-codes options:
//   --count <n>            Number of codes (1..1000)
//   --tier <tier>          Personal | Pro
//   --duration-days <d>    Days of elevation granted on redemption (1..365)
```

- [ ] **Step 4: Build + smoke**

```bash
cd cli && dotnet build --no-restore 2>&1 | tail -10
```

Expected: clean build.

- [ ] **Step 5: Commit**

```bash
git add cli/src/tc/Commands/GenerateOfferCodesCommand.cs \
        cli/src/tc/Json/ \
        cli/src/tc/Program.cs
git commit -m "feat(cli): add tc generate-offer-codes command"
```

---

## Phase H — iOS

**Invoke `ios-coding-standards` skill and `design-language` skill before starting this phase.** The steps below give the shape; the skills dictate the exact patterns (Coordinator callbacks, spy naming, fixture conventions, theming).

### Task H1: `OfferCodeService` + error model

**Files:**
- Create: `mobile/ios/packages/town-crier-data/Sources/OfferCodes/OfferCodeService.swift`
- Create: `mobile/ios/packages/town-crier-data/Sources/OfferCodes/OfferCodeError.swift`

- [ ] **Step 1: Create the service protocol**

```swift
// mobile/ios/packages/town-crier-data/Sources/OfferCodes/OfferCodeService.swift
import Foundation
import TownCrierDomain

public protocol OfferCodeService: Sendable {
    func redeem(code: String) async throws -> OfferCodeRedemption
}

public struct OfferCodeRedemption: Sendable, Equatable {
    public let tier: SubscriptionTier
    public let expiresAt: Date

    public init(tier: SubscriptionTier, expiresAt: Date) {
        self.tier = tier
        self.expiresAt = expiresAt
    }
}
```

- [ ] **Step 2: Create the error mapping**

```swift
// mobile/ios/packages/town-crier-data/Sources/OfferCodes/OfferCodeError.swift
import Foundation

public enum OfferCodeError: Error, Sendable, Equatable {
    case invalidFormat
    case notFound
    case alreadyRedeemed
    case alreadySubscribed
    case network(String)

    public init(serverErrorCode: String) {
        switch serverErrorCode {
        case "invalid_code_format": self = .invalidFormat
        case "invalid_code": self = .notFound
        case "code_already_redeemed": self = .alreadyRedeemed
        case "already_subscribed": self = .alreadySubscribed
        default: self = .network("Unexpected error: \(serverErrorCode)")
        }
    }
}
```

- [ ] **Step 3: Create the HTTP implementation**

```swift
// mobile/ios/packages/town-crier-data/Sources/OfferCodes/HttpOfferCodeService.swift
import Foundation
import TownCrierDomain

public struct HttpOfferCodeService: OfferCodeService {
    private let apiClient: APIClient  // Match the existing HTTP client protocol used by other services

    public init(apiClient: APIClient) {
        self.apiClient = apiClient
    }

    public func redeem(code: String) async throws -> OfferCodeRedemption {
        struct Request: Encodable { let code: String }
        struct Response: Decodable { let tier: String; let expiresAt: Date }
        struct ErrorBody: Decodable { let error: String }

        do {
            let response: Response = try await apiClient.post(
                path: "/v1/offer-codes/redeem",
                body: Request(code: code))
            guard let tier = SubscriptionTier(rawValue: response.tier) else {
                throw OfferCodeError.network("Unknown tier: \(response.tier)")
            }
            return OfferCodeRedemption(tier: tier, expiresAt: response.expiresAt)
        } catch APIClientError.http(let status, let data) where status >= 400 && status < 500 {
            if let body = try? JSONDecoder().decode(ErrorBody.self, from: data) {
                throw OfferCodeError(serverErrorCode: body.error)
            }
            throw OfferCodeError.network("HTTP \(status)")
        }
    }
}
```

**Note:** the exact `APIClient` protocol, `APIClientError` cases, and JSON date strategy come from the existing service layer in `town-crier-data`. Mirror an existing service (e.g. `SubscriptionService` if it exists) exactly rather than guessing types.

- [ ] **Step 4: Commit**

```bash
git add mobile/ios/packages/town-crier-data/Sources/OfferCodes/
git commit -m "feat(ios): add OfferCodeService protocol and HTTP impl"
```

---

### Task H2: `RedeemOfferCodeViewModel` + tests

**Files:**
- Create: `mobile/ios/packages/town-crier-presentation/Sources/Features/RedeemOfferCode/RedeemOfferCodeViewModel.swift`
- Create: `mobile/ios/packages/town-crier-presentation/Tests/RedeemOfferCode/FakeOfferCodeService.swift`
- Create: `mobile/ios/packages/town-crier-presentation/Tests/RedeemOfferCode/RedeemOfferCodeViewModelTests.swift`

- [ ] **Step 1: Write failing ViewModel tests**

```swift
// mobile/ios/packages/town-crier-presentation/Tests/RedeemOfferCode/RedeemOfferCodeViewModelTests.swift
import XCTest
@testable import TownCrierData
@testable import TownCrierPresentation

final class RedeemOfferCodeViewModelTests: XCTestCase {
    private var fakeService: FakeOfferCodeService!
    private var tokenRefreshCalled: Int = 0
    private var profileRefreshCalled: Int = 0
    private var sut: RedeemOfferCodeViewModel!

    override func setUp() {
        super.setUp()
        fakeService = FakeOfferCodeService()
        tokenRefreshCalled = 0
        profileRefreshCalled = 0
        sut = RedeemOfferCodeViewModel(
            service: fakeService,
            onRefreshToken: { self.tokenRefreshCalled += 1 },
            onRefreshProfile: { self.profileRefreshCalled += 1 })
    }

    func test_redeem_onSuccess_setsSuccessStateAndCallsRefreshers() async {
        fakeService.stubbedResult = .success(.init(tier: .pro, expiresAt: Date(timeIntervalSince1970: 1_777_777_777)))
        sut.code = "A7KM-ZQR3-FNXP"

        await sut.redeem()

        if case .success(let tier, _) = sut.state {
            XCTAssertEqual(tier, .pro)
        } else {
            XCTFail("expected success, got \(sut.state)")
        }
        XCTAssertEqual(tokenRefreshCalled, 1)
        XCTAssertEqual(profileRefreshCalled, 1)
    }

    func test_redeem_onAlreadyRedeemed_setsErrorState() async {
        fakeService.stubbedResult = .failure(.alreadyRedeemed)
        sut.code = "A7KM-ZQR3-FNXP"

        await sut.redeem()

        XCTAssertEqual(sut.state, .error("This code has already been used."))
        XCTAssertEqual(tokenRefreshCalled, 0)
    }

    func test_redeem_onAlreadySubscribed_setsErrorState() async {
        fakeService.stubbedResult = .failure(.alreadySubscribed)
        sut.code = "A7KM-ZQR3-FNXP"

        await sut.redeem()

        XCTAssertEqual(
            sut.state,
            .error("You already have an active subscription. Offer codes are only for new subscribers."))
    }

    func test_redeem_onInvalidCode_setsErrorState() async {
        fakeService.stubbedResult = .failure(.notFound)
        sut.code = "BADCODEBBBB1"

        await sut.redeem()

        XCTAssertEqual(sut.state, .error("This code isn't valid."))
    }
}
```

- [ ] **Step 2: Create the fake**

```swift
// mobile/ios/packages/town-crier-presentation/Tests/RedeemOfferCode/FakeOfferCodeService.swift
import Foundation
import TownCrierData

final class FakeOfferCodeService: OfferCodeService, @unchecked Sendable {
    var stubbedResult: Result<OfferCodeRedemption, OfferCodeError> = .failure(.network("unset"))
    private(set) var receivedCodes: [String] = []

    func redeem(code: String) async throws -> OfferCodeRedemption {
        receivedCodes.append(code)
        switch stubbedResult {
        case .success(let redemption): return redemption
        case .failure(let error): throw error
        }
    }
}
```

- [ ] **Step 3: Create the ViewModel**

```swift
// mobile/ios/packages/town-crier-presentation/Sources/Features/RedeemOfferCode/RedeemOfferCodeViewModel.swift
import Foundation
import Observation
import TownCrierData
import TownCrierDomain

@Observable
@MainActor
public final class RedeemOfferCodeViewModel {
    public enum State: Equatable {
        case idle
        case loading
        case success(tier: SubscriptionTier, expiresAt: Date)
        case error(String)
    }

    public var code: String = ""
    public private(set) var state: State = .idle

    private let service: OfferCodeService
    private let onRefreshToken: @MainActor () -> Void
    private let onRefreshProfile: @MainActor () -> Void

    public init(
        service: OfferCodeService,
        onRefreshToken: @escaping @MainActor () -> Void,
        onRefreshProfile: @escaping @MainActor () -> Void
    ) {
        self.service = service
        self.onRefreshToken = onRefreshToken
        self.onRefreshProfile = onRefreshProfile
    }

    public func redeem() async {
        state = .loading
        do {
            let result = try await service.redeem(code: code)
            state = .success(tier: result.tier, expiresAt: result.expiresAt)
            onRefreshToken()
            onRefreshProfile()
        } catch let error as OfferCodeError {
            state = .error(errorMessage(for: error))
        } catch {
            state = .error("Something went wrong. Please try again.")
        }
    }

    private func errorMessage(for error: OfferCodeError) -> String {
        switch error {
        case .invalidFormat: return "Please check the code and try again."
        case .notFound: return "This code isn't valid."
        case .alreadyRedeemed: return "This code has already been used."
        case .alreadySubscribed:
            return "You already have an active subscription. Offer codes are only for new subscribers."
        case .network(let message): return message
        }
    }
}
```

- [ ] **Step 4: Run tests**

```bash
cd mobile/ios && swift test --filter RedeemOfferCodeViewModelTests 2>&1 | tail -15
```

- [ ] **Step 5: Commit**

```bash
git add mobile/ios/packages/town-crier-presentation/Sources/Features/RedeemOfferCode/ \
        mobile/ios/packages/town-crier-presentation/Tests/RedeemOfferCode/
git commit -m "feat(ios): add RedeemOfferCodeViewModel with error mapping"
```

---

### Task H3: SwiftUI view + Settings integration

**Files:**
- Create: `mobile/ios/packages/town-crier-presentation/Sources/Features/RedeemOfferCode/RedeemOfferCodeView.swift`
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/Settings/SettingsView.swift`
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Coordinators/SettingsCoordinator.swift` (or equivalent routing point)

- [ ] **Step 1: Build the view**

```swift
// mobile/ios/packages/town-crier-presentation/Sources/Features/RedeemOfferCode/RedeemOfferCodeView.swift
import SwiftUI
import TownCrierDomain

public struct RedeemOfferCodeView: View {
    @Bindable private var viewModel: RedeemOfferCodeViewModel
    @Environment(\.dismiss) private var dismiss

    public init(viewModel: RedeemOfferCodeViewModel) {
        self.viewModel = viewModel
    }

    public var body: some View {
        NavigationStack {
            VStack(spacing: 24) {
                Text("Enter the 12-character code you received.")
                    .foregroundStyle(.secondary)

                TextField("XXXX-XXXX-XXXX", text: Binding(
                    get: { viewModel.code },
                    set: { viewModel.code = Self.formatAsUserTypes($0) }))
                    .textFieldStyle(.roundedBorder)
                    .font(.system(.title3, design: .monospaced))
                    .textInputAutocapitalization(.characters)
                    .autocorrectionDisabled()

                Button(action: { Task { await viewModel.redeem() } }) {
                    if case .loading = viewModel.state {
                        ProgressView()
                    } else {
                        Text("Redeem").frame(maxWidth: .infinity)
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled({ if case .loading = viewModel.state { return true } else { return viewModel.code.isEmpty } }())

                Group {
                    switch viewModel.state {
                    case .error(let message):
                        Text(message).foregroundStyle(.red)
                    case .success(let tier, let expires):
                        Text("You're on \(tier.displayName) until \(expires.formatted(.dateTime.year().month().day())). Enjoy!")
                            .foregroundStyle(.green)
                    default:
                        EmptyView()
                    }
                }

                Spacer()
            }
            .padding()
            .navigationTitle("Redeem offer code")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                }
            }
        }
    }

    private static func formatAsUserTypes(_ input: String) -> String {
        let stripped = input.uppercased().filter { $0.isLetter || $0.isNumber }
        let limited = String(stripped.prefix(12))
        var buffer = ""
        for (index, char) in limited.enumerated() {
            if index == 4 || index == 8 { buffer.append("-") }
            buffer.append(char)
        }
        return buffer
    }
}
```

- [ ] **Step 2: Add the Settings row**

In `SettingsView.swift`, locate the subscription section and add a new row:

```swift
Button("Redeem offer code") {
    coordinator.showRedeemOfferCode()
}
```

Wire `showRedeemOfferCode()` through the Settings coordinator — pattern match whatever coordinator method style already exists.

- [ ] **Step 3: Manual smoke**

Run the app in the simulator. Open Settings → Redeem offer code → type a known test code → observe success toast.

```bash
cd mobile/ios && swift build 2>&1 | tail -10
```

- [ ] **Step 4: Commit**

```bash
git add mobile/ios/packages/town-crier-presentation/Sources/Features/RedeemOfferCode/ \
        mobile/ios/packages/town-crier-presentation/Sources/Features/Settings/ \
        mobile/ios/packages/town-crier-presentation/Sources/Coordinators/
git commit -m "feat(ios): add RedeemOfferCodeView and wire into Settings"
```

---

## Phase I — Web

**Invoke `react-coding-standards` skill and `design-language` skill before starting this phase.**

### Task I1: API client + types

**Files:**
- Create: `web/src/features/offerCode/api/types.ts`
- Create: `web/src/features/offerCode/api/redeemOfferCode.ts`

- [ ] **Step 1: Define types**

```typescript
// web/src/features/offerCode/api/types.ts
export type Tier = 'Personal' | 'Pro';

export type RedeemResult = {
    tier: Tier;
    expiresAt: string; // ISO-8601
};

export type RedeemErrorCode =
    | 'invalid_code_format'
    | 'invalid_code'
    | 'code_already_redeemed'
    | 'already_subscribed'
    | 'network';

export class RedeemError extends Error {
    constructor(public readonly code: RedeemErrorCode, message: string) {
        super(message);
    }
}
```

- [ ] **Step 2: Implement the client**

```typescript
// web/src/features/offerCode/api/redeemOfferCode.ts
import { RedeemError, type RedeemResult } from './types';

export type RedeemOfferCodeClient = (code: string) => Promise<RedeemResult>;

export const createRedeemOfferCodeClient = (
    getAccessToken: () => Promise<string>,
    apiBaseUrl: string,
): RedeemOfferCodeClient => async (code) => {
    const token = await getAccessToken();
    const response = await fetch(`${apiBaseUrl}/v1/offer-codes/redeem`, {
        method: 'POST',
        headers: {
            'Authorization': `Bearer ${token}`,
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ code }),
    });

    if (response.ok) {
        return (await response.json()) as RedeemResult;
    }

    let serverErrorCode: string | undefined;
    try {
        const body = await response.json();
        serverErrorCode = body.error;
    } catch {
        // fall through
    }

    const known: Record<string, RedeemError['code']> = {
        invalid_code_format: 'invalid_code_format',
        invalid_code: 'invalid_code',
        code_already_redeemed: 'code_already_redeemed',
        already_subscribed: 'already_subscribed',
    };
    const mapped = serverErrorCode && known[serverErrorCode] ? known[serverErrorCode] : 'network';
    throw new RedeemError(mapped, `Redeem failed (${response.status})`);
};
```

- [ ] **Step 3: Commit**

```bash
git add web/src/features/offerCode/api/
git commit -m "feat(web): add offer code API client and error types"
```

---

### Task I2: `useRedeemOfferCode` hook + tests

**Files:**
- Create: `web/src/features/offerCode/hooks/useRedeemOfferCode.ts`
- Create: `web/src/features/offerCode/__tests__/spies/SpyRedeemOfferCodeClient.ts`
- Create: `web/src/features/offerCode/__tests__/useRedeemOfferCode.test.ts`

- [ ] **Step 1: Write the failing hook test**

```typescript
// web/src/features/offerCode/__tests__/useRedeemOfferCode.test.ts
import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useRedeemOfferCode } from '../hooks/useRedeemOfferCode';
import { SpyRedeemOfferCodeClient } from './spies/SpyRedeemOfferCodeClient';
import { RedeemError } from '../api/types';

describe('useRedeemOfferCode', () => {
    it('returns success and triggers refreshers', async () => {
        const client = new SpyRedeemOfferCodeClient();
        client.stubResult({ tier: 'Pro', expiresAt: '2026-05-18T00:00:00Z' });
        const onRefreshToken = vi.fn();
        const onRefreshProfile = vi.fn();

        const { result } = renderHook(() =>
            useRedeemOfferCode({ client: client.redeem, onRefreshToken, onRefreshProfile }));

        await act(async () => { await result.current.redeem('A7KM-ZQR3-FNXP'); });

        expect(result.current.state).toEqual({ kind: 'success', tier: 'Pro', expiresAt: expect.any(String) });
        expect(onRefreshToken).toHaveBeenCalledTimes(1);
        expect(onRefreshProfile).toHaveBeenCalledTimes(1);
    });

    it.each<[RedeemError['code'] | 'network', string]>([
        ['invalid_code_format', 'Please check the code and try again.'],
        ['invalid_code', "This code isn't valid."],
        ['code_already_redeemed', 'This code has already been used.'],
        ['already_subscribed', 'You already have an active subscription. Offer codes are only for new subscribers.'],
        ['network', 'Something went wrong. Please try again.'],
    ])('maps %s error to %s', async (errorCode, expectedMessage) => {
        const client = new SpyRedeemOfferCodeClient();
        if (errorCode === 'network') {
            client.stubError(new Error('fetch failed'));
        } else {
            client.stubError(new RedeemError(errorCode, 'server-said'));
        }

        const { result } = renderHook(() => useRedeemOfferCode({
            client: client.redeem,
            onRefreshToken: () => {},
            onRefreshProfile: () => {},
        }));

        await act(async () => { await result.current.redeem('A7KM-ZQR3-FNXP'); });

        expect(result.current.state).toEqual({ kind: 'error', message: expectedMessage });
    });

    it('does not call refreshers on failure', async () => {
        const client = new SpyRedeemOfferCodeClient();
        client.stubError(new RedeemError('invalid_code', 'nope'));
        const onRefreshToken = vi.fn();
        const onRefreshProfile = vi.fn();

        const { result } = renderHook(() =>
            useRedeemOfferCode({ client: client.redeem, onRefreshToken, onRefreshProfile }));

        await act(async () => { await result.current.redeem('A7KM-ZQR3-FNXP'); });

        expect(onRefreshToken).not.toHaveBeenCalled();
        expect(onRefreshProfile).not.toHaveBeenCalled();
    });
});
```

- [ ] **Step 2: Create the spy**

```typescript
// web/src/features/offerCode/__tests__/spies/SpyRedeemOfferCodeClient.ts
import type { RedeemOfferCodeClient } from '../../api/redeemOfferCode';
import type { RedeemResult } from '../../api/types';

export class SpyRedeemOfferCodeClient {
    private stubbed: { ok?: RedeemResult; err?: Error } = {};
    public calls: string[] = [];

    stubResult(result: RedeemResult): void { this.stubbed = { ok: result }; }
    stubError(error: Error): void { this.stubbed = { err: error }; }

    redeem: RedeemOfferCodeClient = async (code) => {
        this.calls.push(code);
        if (this.stubbed.err) throw this.stubbed.err;
        if (this.stubbed.ok) return this.stubbed.ok;
        throw new Error('SpyRedeemOfferCodeClient not stubbed');
    };
}
```

- [ ] **Step 3: Implement the hook**

```typescript
// web/src/features/offerCode/hooks/useRedeemOfferCode.ts
import { useState, useCallback } from 'react';
import { RedeemError, type RedeemResult, type RedeemErrorCode } from '../api/types';
import type { RedeemOfferCodeClient } from '../api/redeemOfferCode';

type State =
    | { kind: 'idle' }
    | { kind: 'loading' }
    | { kind: 'success', tier: RedeemResult['tier'], expiresAt: string }
    | { kind: 'error', message: string };

const ERROR_MESSAGES: Record<RedeemErrorCode, string> = {
    invalid_code_format: 'Please check the code and try again.',
    invalid_code: "This code isn't valid.",
    code_already_redeemed: 'This code has already been used.',
    already_subscribed: 'You already have an active subscription. Offer codes are only for new subscribers.',
    network: 'Something went wrong. Please try again.',
};

type Args = {
    client: RedeemOfferCodeClient;
    onRefreshToken: () => void | Promise<void>;
    onRefreshProfile: () => void | Promise<void>;
};

export const useRedeemOfferCode = ({ client, onRefreshToken, onRefreshProfile }: Args) => {
    const [state, setState] = useState<State>({ kind: 'idle' });

    const redeem = useCallback(async (code: string) => {
        setState({ kind: 'loading' });
        try {
            const result = await client(code);
            setState({ kind: 'success', tier: result.tier, expiresAt: result.expiresAt });
            await Promise.all([onRefreshToken(), onRefreshProfile()]);
        } catch (error) {
            const errorCode = error instanceof RedeemError ? error.code : 'network';
            setState({ kind: 'error', message: ERROR_MESSAGES[errorCode] });
        }
    }, [client, onRefreshToken, onRefreshProfile]);

    return { state, redeem };
};
```

- [ ] **Step 4: Run tests**

```bash
cd web && npx vitest run src/features/offerCode/__tests__/useRedeemOfferCode.test.ts 2>&1 | tail -15
```

- [ ] **Step 5: Commit**

```bash
git add web/src/features/offerCode/
git commit -m "feat(web): add useRedeemOfferCode hook with error mapping"
```

---

### Task I3: Component + Settings integration

**Files:**
- Create: `web/src/features/offerCode/format/formatOfferCode.ts`
- Create: `web/src/features/offerCode/components/RedeemOfferCode.tsx`
- Create: `web/src/features/offerCode/components/RedeemOfferCode.module.css`
- Modify: `web/src/features/Settings/SettingsPage.tsx`
- Modify: `web/src/features/Settings/ConnectedSettingsPage.tsx`

- [ ] **Step 1: Write the formatter + test**

```typescript
// web/src/features/offerCode/format/formatOfferCode.ts
export const formatAsUserTypes = (raw: string): string => {
    const canonical = raw.toUpperCase().replace(/[^0-9A-Z]/g, '').slice(0, 12);
    const groups: string[] = [];
    for (let i = 0; i < canonical.length; i += 4) groups.push(canonical.slice(i, i + 4));
    return groups.join('-');
};
```

```typescript
// web/src/features/offerCode/__tests__/formatOfferCode.test.ts
import { describe, it, expect } from 'vitest';
import { formatAsUserTypes } from '../format/formatOfferCode';

describe('formatAsUserTypes', () => {
    it.each([
        ['', ''],
        ['a', 'A'],
        ['abcd', 'ABCD'],
        ['abcde', 'ABCD-E'],
        ['abcdefgh', 'ABCD-EFGH'],
        ['abcdefghi', 'ABCD-EFGH-I'],
        ['abcdefghijkl', 'ABCD-EFGH-IJKL'],
        ['abcdefghijklmno', 'ABCD-EFGH-IJKL'], // truncates at 12
        ['a-b c,d', 'ABCD'],                   // strips separators
    ])('formats %s → %s', (input, expected) => {
        expect(formatAsUserTypes(input)).toBe(expected);
    });
});
```

- [ ] **Step 2: Write the component**

```tsx
// web/src/features/offerCode/components/RedeemOfferCode.tsx
import { useState } from 'react';
import type { RedeemOfferCodeClient } from '../api/redeemOfferCode';
import { useRedeemOfferCode } from '../hooks/useRedeemOfferCode';
import { formatAsUserTypes } from '../format/formatOfferCode';
import styles from './RedeemOfferCode.module.css';

type Props = {
    client: RedeemOfferCodeClient;
    onRefreshToken: () => void | Promise<void>;
    onRefreshProfile: () => void | Promise<void>;
};

export const RedeemOfferCode = ({ client, onRefreshToken, onRefreshProfile }: Props) => {
    const { state, redeem } = useRedeemOfferCode({ client, onRefreshToken, onRefreshProfile });
    const [value, setValue] = useState('');

    const isLoading = state.kind === 'loading';
    const isValidLength = value.replace(/-/g, '').length === 12;

    return (
        <section className={styles.section} aria-labelledby="redeem-heading">
            <h3 id="redeem-heading" className={styles.heading}>Redeem offer code</h3>
            <p className={styles.description}>Enter the 12-character code you received.</p>
            <form
                className={styles.form}
                onSubmit={(event) => {
                    event.preventDefault();
                    if (!isLoading && isValidLength) void redeem(value);
                }}
            >
                <input
                    type="text"
                    className={styles.input}
                    placeholder="XXXX-XXXX-XXXX"
                    value={value}
                    onChange={(event) => setValue(formatAsUserTypes(event.target.value))}
                    aria-invalid={state.kind === 'error' || undefined}
                    autoComplete="off"
                    spellCheck={false}
                />
                <button
                    type="submit"
                    className={styles.button}
                    disabled={isLoading || !isValidLength}
                >
                    {isLoading ? 'Redeeming…' : 'Redeem'}
                </button>
            </form>
            {state.kind === 'error' && <p className={styles.error} role="alert">{state.message}</p>}
            {state.kind === 'success' && (
                <p className={styles.success} role="status">
                    You're on {state.tier}. Enjoy!
                </p>
            )}
        </section>
    );
};
```

- [ ] **Step 3: Styles (follow `design-language` tokens)**

```css
/* web/src/features/offerCode/components/RedeemOfferCode.module.css */
.section {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
}

.heading {
    font: var(--font-heading-3);
    margin: 0;
}

.description {
    color: var(--color-text-muted);
    margin: 0;
}

.form {
    display: flex;
    gap: var(--space-2);
    align-items: stretch;
}

.input {
    flex: 1;
    font: var(--font-mono-body);
    padding: var(--space-2) var(--space-3);
    text-transform: uppercase;
    letter-spacing: 0.15em;
}

.button {
    /* follow existing button token pattern */
}

.error { color: var(--color-error); }
.success { color: var(--color-success); }
```

**Adjust token names** to match what's actually defined in the design-language skill. Don't invent tokens.

- [ ] **Step 4: Mount in Settings**

Edit `web/src/features/Settings/SettingsPage.tsx` — add the component inside the subscription section. Thread the `client`, `onRefreshToken`, `onRefreshProfile` props down from `ConnectedSettingsPage`.

- [ ] **Step 5: Run tests + dev server**

```bash
cd web && npx vitest run src/features/offerCode 2>&1 | tail -15
cd web && npx tsc --noEmit
cd web && npm run dev &
```

Open the web app → Settings → verify the new section appears and accepts codes.

- [ ] **Step 6: Commit**

```bash
git add web/src/features/offerCode/ web/src/features/Settings/
git commit -m "feat(web): add offer code redemption section to Settings"
```

---

## Phase J — End-to-end smoke

### Task J1: Generate, redeem, verify

- [ ] **Step 1: Generate codes via CLI against dev**

```bash
cd cli && dotnet run --project src/tc -- generate-offer-codes --count 3 --tier Pro --duration-days 7
```

Expected: three `XXXX-XXXX-XXXX` lines on stdout; summary on stderr.

- [ ] **Step 2: Redeem one on web**

Log in as a free user on the dev deployment, open Settings, paste a code. Expect success toast + tier reflected after refresh.

- [ ] **Step 3: Redeem same code again (from another user or same user)**

Expect 409 "This code has already been used."

- [ ] **Step 4: Attempt with already-subscribed user**

Expect 409 "You already have an active subscription."

- [ ] **Step 5: Verify Cosmos state**

```bash
# Using the admin portal or az cosmosdb query: confirm the OfferCodes container has rows with redeemedByUserId populated.
```

- [ ] **Step 6: Open a PR**

This branch (`feat/offer-codes-spec`) already has the spec and will pick up all implementation commits. Rename the branch if desired (e.g. `feat/offer-codes`) and invoke the `/ship` skill to open the PR.

---

## Self-Review Notes

**Spec coverage:** every section of `docs/specs/offer-codes.md` is covered:
- Domain model → Phase A
- Code format → Task B1
- Cosmos container → Task D1 + F1
- Admin generate endpoint → Task E2
- User redeem endpoint → Task E3
- CLI → Task G1
- iOS Settings → Phase H
- Web Settings → Phase I
- Native AOT — JSON contexts updated in Tasks D1, E1, G1
- Auth0 sync — reused in Task C1 handler; no new code
- Error mapping — covered in E3 + H2 + I2
- Testing — each phase commits tests alongside code

**Known deferred items (from the spec):**
- ETag optimistic concurrency on `CosmosOfferCodeRepository`
- Rate limiting on the redeem endpoint

**Assumptions this plan makes that the executor should verify first:**
1. The `ICosmosRestClient` surface is as shown in `CosmosUserProfileRepository.cs`. If `UpsertDocumentAsync` silently overwrites, the collision-retry loop in `GenerateOfferCodesCommandHandler` becomes best-effort (document in the commit).
2. A web-test authentication shim exists (check `TestWebApplicationFactory` or similar before Task E2/E3).
3. The iOS `APIClient` / `APIClientError` names in `town-crier-data` match the shape assumed in Task H1; adjust imports accordingly.
4. The web design tokens used in Task I3 CSS exist; if not, pick closest tokens from the `design-language` skill.

Each of these is a 5-minute spike; if anything diverges significantly, file a follow-up bead and continue with the deviation called out in the commit message.
