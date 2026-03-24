using System.Text.Json;
using TownCrier.Domain.SavedApplications;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.SavedApplications;

namespace TownCrier.Infrastructure.Tests.SavedApplications;

public sealed class SavedApplicationDocumentTests
{
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
    public async Task Should_RoundTripThroughJsonSerialization_When_UsingCosmosSerializer()
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

        var serializer = new SystemTextJsonCosmosSerializer(jsonOptions);

        // Act
        using var stream = serializer.ToStream(original);
        var deserialized = serializer.FromStream<SavedApplicationDocument>(stream);

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
        // The Cosmos serializer context must support string deserialization for this to work.
        var jsonOptions = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        jsonOptions.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);

        var serializer = new SystemTextJsonCosmosSerializer(jsonOptions);
        var userId = "auth0|user-1";

        // Act
        using var stream = serializer.ToStream(userId);
        var deserialized = serializer.FromStream<string>(stream);

        // Assert
        await Assert.That(deserialized).IsEqualTo(userId);
    }
}
