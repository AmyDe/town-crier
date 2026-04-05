using System.Diagnostics.CodeAnalysis;
using TownCrier.Infrastructure.PlanIt;

namespace TownCrier.Infrastructure.Tests.PlanIt;

[SuppressMessage("Reliability", "CA2000:Dispose objects before losing scope", Justification = "HttpClient disposes the handler")]
[SuppressMessage("Minor Code Smell", "S1075:URIs should not be hardcoded", Justification = "Test base address")]
public sealed class CachedPlanItAuthorityProviderTests
{
    private const string BaseUrl = "https://www.planit.org.uk";

    [Test]
    public async Task Should_ReturnAllAuthorities_When_SinglePageOfResults()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        var json = BuildAreasResponseJson(CreateAreaRecordsJson(3), total: 3);
        handler.SetupJsonResponse("/api/areas/json", json);
        using var provider = CreateProvider(handler);

        // Act
        var authorities = await provider.GetAllAsync(CancellationToken.None);

        // Assert
        await Assert.That(authorities).HasCount().EqualTo(3);
    }

    [Test]
    public async Task Should_PaginateThroughAllPages_When_FirstPageIsFull()
    {
        // Arrange
        using var handler = new FakePlanItHandler();

        var page1Json = BuildAreasResponseJson(CreateAreaRecordsJson(100), total: 150);
        handler.SetupJsonResponse("page=1", page1Json);

        var page2Json = BuildAreasResponseJson(CreateAreaRecordsJson(50, startIndex: 100), total: 150);
        handler.SetupJsonResponse("page=2", page2Json);

        using var provider = CreateProvider(handler);

        // Act
        var authorities = await provider.GetAllAsync(CancellationToken.None);

        // Assert
        await Assert.That(authorities).HasCount().EqualTo(150);
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_UsePageSizeOf100_When_FetchingAreas()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        var json = BuildAreasResponseJson(CreateAreaRecordsJson(1), total: 1);
        handler.SetupJsonResponse("/api/areas/json", json);
        using var provider = CreateProvider(handler);

        // Act
        await provider.GetAllAsync(CancellationToken.None);

        // Assert
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
        await Assert.That(handler.RequestUrls[0]).Contains("pg_sz=100");
        await Assert.That(handler.RequestUrls[0]).DoesNotContain("pg_sz=500");
    }

    [Test]
    public async Task Should_StopPaginating_When_PageHasFewerRecordsThanPageSize()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        var json = BuildAreasResponseJson(CreateAreaRecordsJson(50), total: 50);
        handler.SetupJsonResponse("/api/areas/json", json);
        using var provider = CreateProvider(handler);

        // Act
        await provider.GetAllAsync(CancellationToken.None);

        // Assert - only one request made
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_AccumulateRecordsAcrossPages_When_MultiplePagesFetched()
    {
        // Arrange - simulate 3 pages: 100 + 100 + 25 = 225 authorities
        using var handler = new FakePlanItHandler();

        var page1Json = BuildAreasResponseJson(CreateAreaRecordsJson(100), total: 225);
        handler.SetupJsonResponse("page=1", page1Json);

        var page2Json = BuildAreasResponseJson(CreateAreaRecordsJson(100, startIndex: 100), total: 225);
        handler.SetupJsonResponse("page=2", page2Json);

        var page3Json = BuildAreasResponseJson(CreateAreaRecordsJson(25, startIndex: 200), total: 225);
        handler.SetupJsonResponse("page=3", page3Json);

        using var provider = CreateProvider(handler);

        // Act
        var authorities = await provider.GetAllAsync(CancellationToken.None);

        // Assert
        await Assert.That(authorities).HasCount().EqualTo(225);
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(3);
    }

    [Test]
    public async Task Should_IncludePageParameterInUrl_When_Paginating()
    {
        // Arrange
        using var handler = new FakePlanItHandler();

        var page1Json = BuildAreasResponseJson(CreateAreaRecordsJson(100), total: 150);
        handler.SetupJsonResponse("page=1", page1Json);

        var page2Json = BuildAreasResponseJson(CreateAreaRecordsJson(50, startIndex: 100), total: 150);
        handler.SetupJsonResponse("page=2", page2Json);

        using var provider = CreateProvider(handler);

        // Act
        await provider.GetAllAsync(CancellationToken.None);

        // Assert
        await Assert.That(handler.RequestUrls[0]).Contains("page=1");
        await Assert.That(handler.RequestUrls[1]).Contains("page=2");
    }

    private static CachedPlanItAuthorityProvider CreateProvider(FakePlanItHandler handler)
    {
        var httpClient = new HttpClient(handler, disposeHandler: false)
        {
            BaseAddress = new Uri(BaseUrl),
        };

        return new CachedPlanItAuthorityProvider(httpClient, TimeProvider.System);
    }

    private static string BuildAreasResponseJson(string recordsJson, int total)
    {
        return $$"""
            {
                "records": {{recordsJson}},
                "total": {{total}}
            }
            """;
    }

    private static string CreateAreaRecordsJson(int count, int startIndex = 0)
    {
        var records = new System.Text.StringBuilder("[");
        for (var i = 0; i < count; i++)
        {
            if (i > 0)
            {
                records.Append(',');
            }

            var index = startIndex + i;
            var record = $$"""
                {
                    "area_name": "Authority {{index}}",
                    "area_id": {{index}},
                    "area_type": "London borough",
                    "url": "https://example.com/{{index}}",
                    "planning_url": "https://planning.example.com/{{index}}"
                }
                """;
            records.Append(record);
        }

        records.Append(']');
        return records.ToString();
    }
}
