using System.Text.Json;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.Polling;

namespace TownCrier.Infrastructure.Tests.Polling;

public sealed class PollStateDocumentTests
{
    private readonly JsonSerializerOptions jsonOptions;

    public PollStateDocumentTests()
    {
        this.jsonOptions = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        this.jsonOptions.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);
    }

    [Test]
    public async Task Should_RoundTripCursorFields_When_Serialized()
    {
        // Arrange
        var original = new PollStateDocument
        {
            Id = "poll-state-123",
            LastPollTime = "2026-04-18T12:00:00.0000000+00:00",
            AuthorityId = 123,
            CursorDifferentStart = "2026-04-18",
            CursorNextPage = 4,
            CursorKnownTotal = 7200,
        };

        // Act
        var json = JsonSerializer.Serialize(original, this.jsonOptions);
        var deserialized = JsonSerializer.Deserialize<PollStateDocument>(json, this.jsonOptions)!;

        // Assert
        await Assert.That(deserialized.Id).IsEqualTo(original.Id);
        await Assert.That(deserialized.LastPollTime).IsEqualTo(original.LastPollTime);
        await Assert.That(deserialized.AuthorityId).IsEqualTo(original.AuthorityId);
        await Assert.That(deserialized.CursorDifferentStart).IsEqualTo(original.CursorDifferentStart);
        await Assert.That(deserialized.CursorNextPage).IsEqualTo(original.CursorNextPage);
        await Assert.That(deserialized.CursorKnownTotal).IsEqualTo(original.CursorKnownTotal);
    }

    [Test]
    public async Task Should_DeserializeWithNullCursorFields_When_JsonOmitsCursor()
    {
        // Arrange — legacy documents written before the cursor extension have no cursor fields.
        var legacyJson =
            """
            {
                "id": "poll-state-45",
                "lastPollTime": "2026-04-18T12:00:00.0000000+00:00",
                "authorityId": 45
            }
            """;

        // Act
        var deserialized = JsonSerializer.Deserialize<PollStateDocument>(legacyJson, this.jsonOptions)!;

        // Assert
        await Assert.That(deserialized.Id).IsEqualTo("poll-state-45");
        await Assert.That(deserialized.AuthorityId).IsEqualTo(45);
        await Assert.That(deserialized.CursorDifferentStart).IsNull();
        await Assert.That(deserialized.CursorNextPage).IsNull();
        await Assert.That(deserialized.CursorKnownTotal).IsNull();
    }

    [Test]
    public async Task Should_UseCamelCasePropertyNames_When_Serialized()
    {
        // Arrange
        var document = new PollStateDocument
        {
            Id = "poll-state-7",
            LastPollTime = "2026-04-18T12:00:00.0000000+00:00",
            AuthorityId = 7,
            CursorDifferentStart = "2026-04-18",
            CursorNextPage = 2,
            CursorKnownTotal = 500,
        };

        // Act
        var json = JsonSerializer.Serialize(document, this.jsonOptions);

        // Assert
        await Assert.That(json).Contains("\"cursorDifferentStart\"");
        await Assert.That(json).Contains("\"cursorNextPage\"");
        await Assert.That(json).Contains("\"cursorKnownTotal\"");
    }
}
