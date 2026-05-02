using System.Text.Json;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.SavedApplications;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.SavedApplications;

namespace TownCrier.Infrastructure.Tests.SavedApplications;

public sealed class SavedApplicationDocumentTests
{
    [Test]
    public async Task Should_RoundTripEmbeddedSnapshot_When_MappedFromDomainAndBack()
    {
        // Arrange — saved-list rendering reads the embedded snapshot directly to
        // eliminate the cross-partition fan-out hydration storm (bd tc-udby).
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);
        var application = new PlanningApplication(
            name: "Camden/CAM/24/0042/FUL",
            uid: "planit-uid-abc",
            areaName: "Camden",
            areaId: 42,
            address: "10 Test Lane",
            postcode: "NW1 1AA",
            description: "Test application",
            appType: "Full",
            appState: "Permitted",
            appSize: "Small",
            startDate: new DateOnly(2026, 1, 15),
            decidedDate: new DateOnly(2026, 4, 30),
            consultedDate: new DateOnly(2026, 2, 1),
            longitude: -0.1278,
            latitude: 51.5074,
            url: "planit-example-url",
            link: "council-example-url",
            lastDifferent: new DateTimeOffset(2026, 4, 30, 12, 0, 0, TimeSpan.Zero));
        var domain = SavedApplication.Create("auth0|user-1", application, savedAt);

        // Act
        var document = SavedApplicationDocument.FromDomain(domain);
        var roundTripped = document.ToDomain();

        // Assert — every field round-trips
        await Assert.That(roundTripped.Application).IsNotNull();
        await Assert.That(roundTripped.Application!.Name).IsEqualTo(application.Name);
        await Assert.That(roundTripped.Application.Uid).IsEqualTo(application.Uid);
        await Assert.That(roundTripped.Application.AreaName).IsEqualTo(application.AreaName);
        await Assert.That(roundTripped.Application.AreaId).IsEqualTo(application.AreaId);
        await Assert.That(roundTripped.Application.Address).IsEqualTo(application.Address);
        await Assert.That(roundTripped.Application.Postcode).IsEqualTo(application.Postcode);
        await Assert.That(roundTripped.Application.Description).IsEqualTo(application.Description);
        await Assert.That(roundTripped.Application.AppType).IsEqualTo(application.AppType);
        await Assert.That(roundTripped.Application.AppState).IsEqualTo(application.AppState);
        await Assert.That(roundTripped.Application.AppSize).IsEqualTo(application.AppSize);
        await Assert.That(roundTripped.Application.StartDate).IsEqualTo(application.StartDate);
        await Assert.That(roundTripped.Application.DecidedDate).IsEqualTo(application.DecidedDate);
        await Assert.That(roundTripped.Application.ConsultedDate).IsEqualTo(application.ConsultedDate);
        await Assert.That(roundTripped.Application.Longitude).IsEqualTo(application.Longitude);
        await Assert.That(roundTripped.Application.Latitude).IsEqualTo(application.Latitude);
        await Assert.That(roundTripped.Application.Url).IsEqualTo(application.Url);
        await Assert.That(roundTripped.Application.Link).IsEqualTo(application.Link);
        await Assert.That(roundTripped.Application.LastDifferent).IsEqualTo(application.LastDifferent);
    }

    [Test]
    public async Task Should_RoundTripSnapshotThroughJson_When_SerializedWithSourceGenerators()
    {
        // Arrange — Native AOT JSON path: prove the embedded snapshot survives
        // System.Text.Json source-generator round-trip.
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);
        var application = new PlanningApplication(
            name: "Camden/CAM/24/0042/FUL",
            uid: "planit-uid-abc",
            areaName: "Camden",
            areaId: 42,
            address: "10 Test Lane",
            postcode: "NW1 1AA",
            description: "Test application",
            appType: "Full",
            appState: "Permitted",
            appSize: null,
            startDate: new DateOnly(2026, 1, 15),
            decidedDate: null,
            consultedDate: null,
            longitude: -0.1278,
            latitude: 51.5074,
            url: null,
            link: null,
            lastDifferent: new DateTimeOffset(2026, 4, 30, 12, 0, 0, TimeSpan.Zero));
        var original = SavedApplicationDocument.FromDomain(
            SavedApplication.Create("auth0|user-1", application, savedAt));

        var jsonOptions = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        jsonOptions.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);

        // Act
        var json = JsonSerializer.Serialize(original, jsonOptions);
        var deserialized = JsonSerializer.Deserialize<SavedApplicationDocument>(json, jsonOptions)!;
        var domain = deserialized.ToDomain();

        // Assert
        await Assert.That(domain.Application).IsNotNull();
        await Assert.That(domain.Application!.Uid).IsEqualTo("planit-uid-abc");
        await Assert.That(domain.Application.AppState).IsEqualTo("Permitted");
        await Assert.That(domain.Application.AreaId).IsEqualTo(42);
        await Assert.That(domain.Application.Latitude).IsEqualTo(51.5074);
        await Assert.That(domain.Application.Longitude).IsEqualTo(-0.1278);
    }

    [Test]
    public async Task Should_PreserveNullSnapshot_When_LegacyRowHasUidOnly()
    {
        // Arrange — backfill compatibility: existing saved rows persisted before
        // the snapshot column existed deserialize with a null Application.
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);
        var legacyDomain = SavedApplication.Create("auth0|user-1", "planit-uid-abc", savedAt);

        // Act
        var document = SavedApplicationDocument.FromDomain(legacyDomain);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.Application).IsNull();
        await Assert.That(roundTripped.ApplicationUid).IsEqualTo("planit-uid-abc");
    }

    [Test]
    public async Task Should_CreateCompositeId_When_MappedFromDomain()
    {
        // Arrange
        var savedAt = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);
        var domain = SavedApplication.Create("auth0|user-1", "planit-uid-abc", savedAt);

        // Act
        var document = SavedApplicationDocument.FromDomain(domain);

        // Assert
        await Assert.That(document.Id).IsEqualTo("auth0|user-1:planit-uid-abc");
    }

    [Test]
    public async Task Should_SetUserIdAsPartitionKey_When_MappedFromDomain()
    {
        // Arrange
        var savedAt = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);
        var domain = SavedApplication.Create("auth0|user-1", "planit-uid-abc", savedAt);

        // Act
        var document = SavedApplicationDocument.FromDomain(domain);

        // Assert
        await Assert.That(document.UserId).IsEqualTo("auth0|user-1");
    }

    [Test]
    public async Task Should_PreserveAllFields_When_MappedFromDomain()
    {
        // Arrange
        var savedAt = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);
        var domain = SavedApplication.Create("auth0|user-1", "planit-uid-abc", savedAt);

        // Act
        var document = SavedApplicationDocument.FromDomain(domain);

        // Assert
        await Assert.That(document.ApplicationUid).IsEqualTo("planit-uid-abc");
        await Assert.That(document.SavedAt).IsEqualTo(savedAt);
    }

    [Test]
    public async Task Should_RoundTripToDomain_When_MappedBackAndForth()
    {
        // Arrange
        var savedAt = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);
        var original = SavedApplication.Create("auth0|user-1", "planit-uid-abc", savedAt);

        // Act
        var document = SavedApplicationDocument.FromDomain(original);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.UserId).IsEqualTo(original.UserId);
        await Assert.That(roundTripped.ApplicationUid).IsEqualTo(original.ApplicationUid);
        await Assert.That(roundTripped.SavedAt).IsEqualTo(original.SavedAt);
    }

    [Test]
    public async Task Should_RoundTripThroughJsonSerialization_When_SerializedWithSourceGenerators()
    {
        // Arrange
        var savedAt = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);
        var original = SavedApplicationDocument.FromDomain(
            SavedApplication.Create("auth0|user-1", "planit-uid-abc", savedAt));

        var jsonOptions = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        jsonOptions.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);

        // Act
        var json = JsonSerializer.Serialize(original, jsonOptions);
        var deserialized = JsonSerializer.Deserialize<SavedApplicationDocument>(json, jsonOptions)!;

        // Assert
        await Assert.That(deserialized.Id).IsEqualTo(original.Id);
        await Assert.That(deserialized.UserId).IsEqualTo(original.UserId);
        await Assert.That(deserialized.ApplicationUid).IsEqualTo(original.ApplicationUid);
        await Assert.That(deserialized.SavedAt).IsEqualTo(original.SavedAt);
    }

    [Test]
    public async Task Should_FailDeserialization_When_PartialProjectionMissesRequiredFields()
    {
        // Arrange — SELECT c.userId returns {"userId":"auth0|user-1"} which is missing
        // the required Id, ApplicationUid, and SavedAt fields on SavedApplicationDocument.
        // This documents why the repository must use SELECT VALUE with GetItemQueryIterator<string>.
        var partialJson = """{"userId":"auth0|user-1"}"""u8.ToArray();
        var jsonOptions = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        jsonOptions.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);

        // Act & Assert — deserializing a partial document into SavedApplicationDocument throws
        Assert.Throws<JsonException>(() => JsonSerializer.Deserialize<SavedApplicationDocument>(
            (ReadOnlySpan<byte>)partialJson, jsonOptions));
    }

    [Test]
    public async Task Should_DeserializeStringValue_When_UsingSelectValueProjection()
    {
        // Arrange — SELECT VALUE c.userId returns a bare JSON string, not a document.
        // The serializer context must support string deserialization for this to work.
        var jsonOptions = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        jsonOptions.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);

        var userId = "auth0|user-1";

        // Act
        var json = JsonSerializer.Serialize(userId, jsonOptions);
        var deserialized = JsonSerializer.Deserialize<string>(json, jsonOptions)!;

        // Assert
        await Assert.That(deserialized).IsEqualTo(userId);
    }
}
