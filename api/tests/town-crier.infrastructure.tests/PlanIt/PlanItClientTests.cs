using System.Diagnostics.CodeAnalysis;
using System.Globalization;
using TownCrier.Infrastructure.PlanIt;

namespace TownCrier.Infrastructure.Tests.PlanIt;

[SuppressMessage("Reliability", "CA2000:Dispose objects before losing scope", Justification = "HttpClient disposes the handler")]
[SuppressMessage("Minor Code Smell", "S1075:URIs should not be hardcoded", Justification = "Test base address")]
public sealed class PlanItClientTests
{
    private const string BaseUrl = "https://www.planit.org.uk";

    private const string SingleRecordResponse = """
        {
            "records": [
                {
                    "name": "Leeds/26/01471/TR",
                    "uid": "26/01471/TR",
                    "area_name": "Leeds",
                    "area_id": 292,
                    "address": "Highgate House Grove Lane Leeds",
                    "postcode": "LS6 2AP",
                    "description": "T1 lime tree - crown reduction",
                    "app_type": "Trees",
                    "app_state": "Undecided",
                    "app_size": "Small",
                    "start_date": "2026-03-13",
                    "decided_date": null,
                    "consulted_date": null,
                    "location_x": -1.577373,
                    "location_y": 53.824035,
                    "url": "https://publicaccess.leeds.gov.uk/example",
                    "link": "https://www.planit.org.uk/planapplic/26-01471-TR",
                    "last_different": "2026-03-14T11:59:17.642"
                }
            ],
            "pg_sz": 5000,
            "from": 0,
            "total": 1
        }
        """;

    private const string EmptyResponse = """
        {
            "records": [],
            "pg_sz": 5000,
            "from": 0,
            "total": 0
        }
        """;

    [Test]
    public async Task Should_ReturnApplications_When_ApiReturnsResults()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", SingleRecordResponse);
        var client = CreateClient(handler);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert
        await Assert.That(results).HasCount().EqualTo(1);

        var application = results[0];
        await Assert.That(application.Name).IsEqualTo("Leeds/26/01471/TR");
        await Assert.That(application.Uid).IsEqualTo("26/01471/TR");
        await Assert.That(application.AreaName).IsEqualTo("Leeds");
        await Assert.That(application.AreaId).IsEqualTo(292);
        await Assert.That(application.Address).IsEqualTo("Highgate House Grove Lane Leeds");
        await Assert.That(application.Postcode).IsEqualTo("LS6 2AP");
        await Assert.That(application.Description).IsEqualTo("T1 lime tree - crown reduction");
        await Assert.That(application.AppType).IsEqualTo("Trees");
        await Assert.That(application.AppState).IsEqualTo("Undecided");
        await Assert.That(application.Longitude).IsEqualTo(-1.577373);
        await Assert.That(application.Latitude).IsEqualTo(53.824035);

        var expected = DateTimeOffset.Parse("2026-03-14T11:59:17.642", CultureInfo.InvariantCulture);
        await Assert.That(application.LastDifferent).IsEqualTo(expected);
    }

    [Test]
    public async Task Should_PassDifferentStartParameter_When_Provided()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("/api/applics/json", EmptyResponse);
        var client = CreateClient(handler);
        var differentStart = new DateTimeOffset(2026, 3, 15, 10, 30, 0, TimeSpan.Zero);

        // Act
        await ConsumeAsync(client, differentStart);

        // Assert
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
        await Assert.That(handler.RequestUrls[0]).Contains("different_start=2026-03-15T10:30:00");
    }

    [Test]
    public async Task Should_PaginateThroughAllPages_When_ResultsEqualPageSize()
    {
        // Arrange
        using var handler = new FakePlanItHandler();

        var page1Records = CreateRecordsJson(5000);
        var page1Json = BuildResponseJson(page1Records, total: 5500);
        handler.SetupJsonResponse("page=1", page1Json);

        var page2Records = CreateRecordsJson(500, startIndex: 5000);
        var page2Json = BuildResponseJson(page2Records, total: 5500, from: 5000);
        handler.SetupJsonResponse("page=2", page2Json);

        var client = CreateClient(handler);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert
        await Assert.That(results).HasCount().EqualTo(5500);
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(2);
        await Assert.That(handler.RequestUrls[0]).Contains("page=1");
        await Assert.That(handler.RequestUrls[1]).Contains("page=2");
    }

    [Test]
    public async Task Should_StopPaginating_When_ResultsLessThanPageSize()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", SingleRecordResponse);
        var client = CreateClient(handler);

        // Act
        await ConsumeAsync(client, differentStart: null);

        // Assert — only one page requested
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_ReturnEmptySequence_When_NoResults()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", EmptyResponse);
        var client = CreateClient(handler);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert
        await Assert.That(results).HasCount().EqualTo(0);
    }

    private static PlanItClient CreateClient(FakePlanItHandler handler)
    {
        var httpClient = new HttpClient(handler, disposeHandler: false)
        {
            BaseAddress = new Uri(BaseUrl),
        };
        return new PlanItClient(httpClient);
    }

    private static async Task<List<TownCrier.Domain.PlanningApplications.PlanningApplication>> ConsumeAsync(
        PlanItClient client,
        DateTimeOffset? differentStart)
    {
        var results = new List<TownCrier.Domain.PlanningApplications.PlanningApplication>();
        await foreach (var app in client.FetchApplicationsAsync(differentStart, CancellationToken.None))
        {
            results.Add(app);
        }

        return results;
    }

    private static string BuildResponseJson(string recordsJson, int total, int from = 0)
    {
        return $$$"""
            {
                "records": {{{recordsJson}}},
                "pg_sz": 5000,
                "from": {{{from}}},
                "total": {{{total}}}
            }
            """;
    }

    private static string CreateRecordsJson(int count, int startIndex = 0)
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
                    "name": "Leeds/APP-{{index}}",
                    "uid": "APP-{{index}}",
                    "area_name": "Leeds",
                    "area_id": 292,
                    "address": "Address {{index}}",
                    "description": "Description {{index}}",
                    "app_type": "Full",
                    "app_state": "Undecided",
                    "last_different": "2026-03-14T11:59:17.642"
                }
                """;
            records.Append(record);
        }

        records.Append(']');
        return records.ToString();
    }
}
