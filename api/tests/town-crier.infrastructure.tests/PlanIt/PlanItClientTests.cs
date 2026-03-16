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

    [Test]
    public async Task Should_RetryAndSucceed_When_ApiReturns429ThenSuccess()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupRateLimitThenSuccess("page=1", count: 2, SingleRecordResponse);
        var delays = new List<TimeSpan>();
        var client = CreateClient(handler, retryOptions: new PlanItRetryOptions { MaxRetries = 3, BaseDelay = TimeSpan.FromMilliseconds(10) }, delays: delays);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert — got results after retries
        await Assert.That(results).HasCount().EqualTo(1);

        // 2 x 429 + 1 success = 3 total requests
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(3);
    }

    [Test]
    public async Task Should_ApplyExponentialBackoff_When_Retrying429()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupRateLimitThenSuccess("page=1", count: 3, SingleRecordResponse);
        var delays = new List<TimeSpan>();
        var options = new PlanItRetryOptions { MaxRetries = 5, BaseDelay = TimeSpan.FromSeconds(1) };
        var client = CreateClient(handler, retryOptions: options, delays: delays);

        // Act
        await ConsumeAsync(client, differentStart: null);

        // Assert — backoff progression: 1s, 2s, 4s (exponential, ignoring jitter for range check)
        await Assert.That(delays).HasCount().EqualTo(3);
        await Assert.That(delays[0]).IsGreaterThanOrEqualTo(TimeSpan.FromMilliseconds(500))
            .And.IsLessThanOrEqualTo(TimeSpan.FromMilliseconds(1500));
        await Assert.That(delays[1]).IsGreaterThanOrEqualTo(TimeSpan.FromMilliseconds(1000))
            .And.IsLessThanOrEqualTo(TimeSpan.FromMilliseconds(3000));
        await Assert.That(delays[2]).IsGreaterThanOrEqualTo(TimeSpan.FromMilliseconds(2000))
            .And.IsLessThanOrEqualTo(TimeSpan.FromMilliseconds(6000));
    }

    [Test]
    public async Task Should_ThrowHttpRequestException_When_MaxRetriesExhausted()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupRateLimitForever("page=1");
        var options = new PlanItRetryOptions { MaxRetries = 3, BaseDelay = TimeSpan.FromMilliseconds(1) };
        var client = CreateClient(handler, retryOptions: options);

        // Act & Assert
        await Assert.ThrowsAsync<HttpRequestException>(
            async () => await ConsumeAsync(client, differentStart: null));
    }

    [Test]
    public async Task Should_PropagateNon429Errors_When_ServerReturns500()
    {
        // Arrange
        using var handler = new FakePlanItHandler();

        // No response configured → returns 404, which is a non-429 error
        var options = new PlanItRetryOptions { MaxRetries = 3, BaseDelay = TimeSpan.FromMilliseconds(1) };
        var client = CreateClient(handler, retryOptions: options);

        // Act & Assert — should throw immediately without retrying
        await Assert.ThrowsAsync<HttpRequestException>(
            async () => await ConsumeAsync(client, differentStart: null));

        // Only 1 request — no retries for non-429 errors
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
    }

    private static PlanItClient CreateClient(
        FakePlanItHandler handler,
        PlanItRetryOptions? retryOptions = null,
        List<TimeSpan>? delays = null)
    {
        var httpClient = new HttpClient(handler, disposeHandler: false)
        {
            BaseAddress = new Uri(BaseUrl),
        };

        Func<TimeSpan, CancellationToken, Task>? delayFunc = null;
        if (delays is not null)
        {
            delayFunc = (delay, _) =>
            {
                delays.Add(delay);
                return Task.CompletedTask;
            };
        }

        return new PlanItClient(httpClient, retryOptions ?? new PlanItRetryOptions(), delayFunc);
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
