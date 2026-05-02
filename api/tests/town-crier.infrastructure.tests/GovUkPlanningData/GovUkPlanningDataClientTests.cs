using System.Diagnostics.CodeAnalysis;
using TownCrier.Infrastructure.GovUkPlanningData;

namespace TownCrier.Infrastructure.Tests.GovUkPlanningData;

[SuppressMessage("Reliability", "CA2000:Dispose objects before losing scope", Justification = "HttpClient disposes the handler")]
[SuppressMessage("Minor Code Smell", "S1075:URIs should not be hardcoded", Justification = "Test base address")]
public sealed class GovUkPlanningDataClientTests
{
    private const string BaseUrl = "https://www.planning.data.gov.uk";

    [Test]
    public async Task Should_MapConservationArea_When_ApiReturnsConservationAreaEntity()
    {
        // Arrange
        const string json = """
            {
                "entities": [
                    {
                        "dataset": "conservation-area",
                        "name": "Westminster",
                        "reference": "CA-001"
                    }
                ]
            }
            """;

        using var handler = new FakeGovUkHandler();
        handler.SetupJsonResponse("entity.json", json);
        var client = CreateClient(handler);

        // Act
        var result = await client.GetDesignationsAsync(51.501009, -0.141588, CancellationToken.None);

        // Assert
        await Assert.That(result.IsWithinConservationArea).IsTrue();
        await Assert.That(result.ConservationAreaName).IsEqualTo("Westminster");
        await Assert.That(result.IsWithinListedBuildingCurtilage).IsFalse();
        await Assert.That(result.IsWithinArticle4Area).IsFalse();
    }

    [Test]
    public async Task Should_MapListedBuilding_When_ApiReturnsListedBuildingEntity()
    {
        // Arrange
        const string json = """
            {
                "entities": [
                    {
                        "dataset": "listed-building-outline",
                        "name": "Grade II Listed House",
                        "reference": "LB-001",
                        "listed-building-grade": "II"
                    }
                ]
            }
            """;

        using var handler = new FakeGovUkHandler();
        handler.SetupJsonResponse("entity.json", json);
        var client = CreateClient(handler);

        // Act
        var result = await client.GetDesignationsAsync(51.5, -0.1, CancellationToken.None);

        // Assert
        await Assert.That(result.IsWithinListedBuildingCurtilage).IsTrue();
        await Assert.That(result.ListedBuildingGrade).IsEqualTo("II");
        await Assert.That(result.IsWithinConservationArea).IsFalse();
        await Assert.That(result.IsWithinArticle4Area).IsFalse();
    }

    [Test]
    public async Task Should_MapArticle4Area_When_ApiReturnsArticle4Entity()
    {
        // Arrange
        const string json = """
            {
                "entities": [
                    {
                        "dataset": "article-4-direction-area",
                        "name": "Article 4 Direction",
                        "reference": "A4-001"
                    }
                ]
            }
            """;

        using var handler = new FakeGovUkHandler();
        handler.SetupJsonResponse("entity.json", json);
        var client = CreateClient(handler);

        // Act
        var result = await client.GetDesignationsAsync(51.5, -0.1, CancellationToken.None);

        // Assert
        await Assert.That(result.IsWithinArticle4Area).IsTrue();
        await Assert.That(result.IsWithinConservationArea).IsFalse();
        await Assert.That(result.IsWithinListedBuildingCurtilage).IsFalse();
    }

    [Test]
    public async Task Should_MapAllDesignations_When_ApiReturnsMultipleEntities()
    {
        // Arrange
        const string json = """
            {
                "entities": [
                    {
                        "dataset": "conservation-area",
                        "name": "Belgravia",
                        "reference": "CA-002"
                    },
                    {
                        "dataset": "listed-building-outline",
                        "name": "Grade I Manor",
                        "reference": "LB-002",
                        "listed-building-grade": "I"
                    },
                    {
                        "dataset": "article-4-direction-area",
                        "name": "HMO Direction",
                        "reference": "A4-002"
                    }
                ]
            }
            """;

        using var handler = new FakeGovUkHandler();
        handler.SetupJsonResponse("entity.json", json);
        var client = CreateClient(handler);

        // Act
        var result = await client.GetDesignationsAsync(51.5, -0.15, CancellationToken.None);

        // Assert
        await Assert.That(result.IsWithinConservationArea).IsTrue();
        await Assert.That(result.ConservationAreaName).IsEqualTo("Belgravia");
        await Assert.That(result.IsWithinListedBuildingCurtilage).IsTrue();
        await Assert.That(result.ListedBuildingGrade).IsEqualTo("I");
        await Assert.That(result.IsWithinArticle4Area).IsTrue();
    }

    [Test]
    public async Task Should_ReturnNone_When_ApiReturnsEmptyEntities()
    {
        // Arrange
        const string json = """
            {
                "entities": []
            }
            """;

        using var handler = new FakeGovUkHandler();
        handler.SetupJsonResponse("entity.json", json);
        var client = CreateClient(handler);

        // Act
        var result = await client.GetDesignationsAsync(52.0, -1.0, CancellationToken.None);

        // Assert
        await Assert.That(result.IsWithinConservationArea).IsFalse();
        await Assert.That(result.ConservationAreaName).IsNull();
        await Assert.That(result.IsWithinListedBuildingCurtilage).IsFalse();
        await Assert.That(result.ListedBuildingGrade).IsNull();
        await Assert.That(result.IsWithinArticle4Area).IsFalse();
    }

    [Test]
    public async Task Should_ConstructCorrectUrl_When_CalledWithCoordinates()
    {
        // Arrange
        const string json = """{ "entities": [] }""";
        using var handler = new FakeGovUkHandler();
        handler.SetupJsonResponse("entity.json", json);
        var client = CreateClient(handler);

        // Act
        await client.GetDesignationsAsync(51.501009, -0.141588, CancellationToken.None);

        // Assert
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
        await Assert.That(handler.RequestUrls[0]).Contains("geometry_intersects=");
        await Assert.That(handler.RequestUrls[0]).Contains("-0.141588");
        await Assert.That(handler.RequestUrls[0]).Contains("51.501009");
        await Assert.That(handler.RequestUrls[0]).Contains("dataset=conservation-area");
    }

    [Test]
    public async Task Should_ThrowHttpRequestException_When_ApiReturnsServerError()
    {
        // Arrange
        using var handler = new FakeGovUkHandler();
        handler.SetupErrorResponse("entity.json", System.Net.HttpStatusCode.InternalServerError);
        var client = CreateClient(handler);

        // Act & Assert
        await Assert.ThrowsAsync<HttpRequestException>(
            async () => await client.GetDesignationsAsync(51.5, -0.1, CancellationToken.None));
    }

    [Test]
    public async Task Should_ReturnNone_When_ApiReturnsNotFound()
    {
        // Arrange
        // The planning.data.gov.uk entity endpoint returns 404 when the query
        // geometry doesn't intersect any entity in the requested datasets.
        // That's an expected "no designations here" response, not an error.
        using var handler = new FakeGovUkHandler();
        handler.SetupErrorResponse("entity.json", System.Net.HttpStatusCode.NotFound);
        var client = CreateClient(handler);

        // Act
        var result = await client.GetDesignationsAsync(54.0, -2.0, CancellationToken.None);

        // Assert
        await Assert.That(result.IsWithinConservationArea).IsFalse();
        await Assert.That(result.ConservationAreaName).IsNull();
        await Assert.That(result.IsWithinListedBuildingCurtilage).IsFalse();
        await Assert.That(result.ListedBuildingGrade).IsNull();
        await Assert.That(result.IsWithinArticle4Area).IsFalse();
    }

    private static GovUkPlanningDataClient CreateClient(FakeGovUkHandler handler)
    {
        var httpClient = new HttpClient(handler, disposeHandler: false)
        {
            BaseAddress = new Uri(BaseUrl),
        };

        return new GovUkPlanningDataClient(httpClient);
    }
}
