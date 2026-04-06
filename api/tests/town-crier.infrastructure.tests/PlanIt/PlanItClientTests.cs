using System.Diagnostics.CodeAnalysis;
using System.Diagnostics.Metrics;
using System.Globalization;
using System.Net;
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
            "pg_sz": 100,
            "from": 0,
            "total": 1
        }
        """;

    private const string EmptyResponse = """
        {
            "records": [],
            "pg_sz": 100,
            "from": 0,
            "total": 0
        }
        """;

    private const string NullDescriptionResponse = """
        {
            "records": [
                {
                    "name": "Leeds/26/01500/FUL",
                    "uid": "26/01500/FUL",
                    "area_name": "Leeds",
                    "area_id": 292,
                    "address": "1 Example Road Leeds",
                    "description": null,
                    "app_type": "Full",
                    "app_state": "Undecided",
                    "last_different": "2026-03-14T11:59:17.642"
                }
            ],
            "pg_sz": 100,
            "from": 0,
            "total": 1
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
    public async Task Should_IncludeAuthorityIdInUrl_When_FetchingApplications()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("/api/applics/json", EmptyResponse);
        var client = CreateClient(handler);

        // Act
        await ConsumeAsync(client, differentStart: null, authorityId: 292);

        // Assert
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
        await Assert.That(handler.RequestUrls[0]).Contains("auth=292");
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
        await Assert.That(handler.RequestUrls[0]).Contains("different_start=2026-03-15");
        await Assert.That(handler.RequestUrls[0]).DoesNotContain("T10:30:00");
    }

    [Test]
    public async Task Should_PaginateThroughAllPages_When_ResultsEqualPageSize()
    {
        // Arrange
        using var handler = new FakePlanItHandler();

        var page1Records = CreateRecordsJson(100);
        var page1Json = BuildResponseJson(page1Records, total: 150);
        handler.SetupJsonResponse("page=1", page1Json);

        var page2Records = CreateRecordsJson(50, startIndex: 100);
        var page2Json = BuildResponseJson(page2Records, total: 150, from: 100);
        handler.SetupJsonResponse("page=2", page2Json);

        var client = CreateClient(handler);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert
        await Assert.That(results).HasCount().EqualTo(150);
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
        var client = CreateClient(handler, retryOptions: new PlanItRetryOptions { MaxRetries = 3, BaseDelaySeconds = 0.01 }, delays: delays);

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
        var options = new PlanItRetryOptions { MaxRetries = 5, BaseDelaySeconds = 1 };
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
        var options = new PlanItRetryOptions { MaxRetries = 3, BaseDelaySeconds = 0.001 };
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
        var options = new PlanItRetryOptions { MaxRetries = 3, BaseDelaySeconds = 0.001 };
        var client = CreateClient(handler, retryOptions: options);

        // Act & Assert — should throw immediately without retrying
        await Assert.ThrowsAsync<HttpRequestException>(
            async () => await ConsumeAsync(client, differentStart: null));

        // Only 1 request — no retries for non-429 errors
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_UseSearchParameter_When_SearchingApplications()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("/api/applics/json", SingleRecordResponse);
        var client = CreateClient(handler);

        // Act
        await client.SearchApplicationsAsync("car park", 314, 1, CancellationToken.None);

        // Assert — must use 'search=' not 'q=', and pg_sz must be small (not 100)
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
        await Assert.That(handler.RequestUrls[0]).Contains("search=car%20park");
        await Assert.That(handler.RequestUrls[0]).DoesNotContain("&q=");
        await Assert.That(handler.RequestUrls[0]).Contains("pg_sz=20");
    }

    [Test]
    public async Task Should_DelayBeforeEachRequest_When_ThrottleConfigured()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", SingleRecordResponse);
        var throttleDelays = new List<TimeSpan>();
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0.5 };
        var client = CreateClient(handler, throttleOptions: throttleOptions, throttleDelays: throttleDelays);

        // Act
        await ConsumeAsync(client, differentStart: null);

        // Assert — one throttle delay before the single request
        await Assert.That(throttleDelays).HasCount().EqualTo(1);
        await Assert.That(throttleDelays[0]).IsEqualTo(TimeSpan.FromMilliseconds(500));
    }

    [Test]
    public async Task Should_DelayBeforeEachPageRequest_When_Paginating()
    {
        // Arrange
        using var handler = new FakePlanItHandler();

        var page1Records = CreateRecordsJson(100);
        var page1Json = BuildResponseJson(page1Records, total: 150);
        handler.SetupJsonResponse("page=1", page1Json);

        var page2Records = CreateRecordsJson(50, startIndex: 100);
        var page2Json = BuildResponseJson(page2Records, total: 150, from: 100);
        handler.SetupJsonResponse("page=2", page2Json);

        var throttleDelays = new List<TimeSpan>();
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0.2 };
        var client = CreateClient(handler, throttleOptions: throttleOptions, throttleDelays: throttleDelays);

        // Act
        await ConsumeAsync(client, differentStart: null);

        // Assert — one throttle delay per page (2 pages = 2 delays)
        await Assert.That(throttleDelays).HasCount().EqualTo(2);
        await Assert.That(throttleDelays[0]).IsEqualTo(TimeSpan.FromMilliseconds(200));
        await Assert.That(throttleDelays[1]).IsEqualTo(TimeSpan.FromMilliseconds(200));
    }

    [Test]
    public async Task Should_DelayBeforeSearchRequest_When_ThrottleConfigured()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("/api/applics/json", SingleRecordResponse);
        var throttleDelays = new List<TimeSpan>();
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0.3 };
        var client = CreateClient(handler, throttleOptions: throttleOptions, throttleDelays: throttleDelays);

        // Act
        await client.SearchApplicationsAsync("car park", 314, 1, CancellationToken.None);

        // Assert — one throttle delay before the search request
        await Assert.That(throttleDelays).HasCount().EqualTo(1);
        await Assert.That(throttleDelays[0]).IsEqualTo(TimeSpan.FromMilliseconds(300));
    }

    [Test]
    public async Task Should_ThrottleBeforeEveryRetryAttempt_When_RateLimitedThenSucceeds()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupRateLimitThenSuccess("page=1", count: 1, SingleRecordResponse);
        var allDelays = new List<TimeSpan>();
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0.1 };
        var retryOptions = new PlanItRetryOptions { MaxRetries = 3, BaseDelaySeconds = 0.01 };
        var client = CreateClient(handler, retryOptions: retryOptions, throttleOptions: throttleOptions, throttleDelays: allDelays);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert — got results after retry
        await Assert.That(results).HasCount().EqualTo(1);

        // 1 x 429 + 1 success = 2 total HTTP requests
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(2);

        // Throttle fires before EVERY HTTP request (2 throttle delays) plus 1 backoff delay after 429 = 3 total
        var throttleCount = allDelays.Count(d => d == TimeSpan.FromMilliseconds(100));
        await Assert.That(throttleCount).IsEqualTo(2);
    }

    [Test]
    public async Task Should_DeserializeAndReturnApplications_When_PaginationFieldsAreNull()
    {
        // Arrange
        const string nullPaginationResponse = """
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
                "pg_sz": null,
                "from": null,
                "total": null
            }
            """;

        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", nullPaginationResponse);
        var client = CreateClient(handler);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert — should deserialize without throwing
        await Assert.That(results).HasCount().EqualTo(1);
        await Assert.That(results[0].Name).IsEqualTo("Leeds/26/01471/TR");
    }

    [Test]
    public async Task Should_ReturnZeroTotal_When_SearchResponseHasNullTotal()
    {
        // Arrange
        const string nullTotalSearchResponse = """
            {
                "records": [
                    {
                        "name": "Leeds/26/01471/TR",
                        "uid": "26/01471/TR",
                        "area_name": "Leeds",
                        "area_id": 292,
                        "address": "Highgate House Grove Lane Leeds",
                        "description": "T1 lime tree - crown reduction",
                        "app_type": "Trees",
                        "app_state": "Undecided",
                        "last_different": "2026-03-14T11:59:17.642"
                    }
                ],
                "pg_sz": null,
                "from": null,
                "total": null
            }
            """;

        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("/api/applics/json", nullTotalSearchResponse);
        var client = CreateClient(handler);

        // Act
        var result = await client.SearchApplicationsAsync("tree", 292, 1, CancellationToken.None);

        // Assert — null total should be treated as 0
        await Assert.That(result.Total).IsEqualTo(0);
        await Assert.That(result.Applications).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_UseDefaultOneSecondDelay_When_NoThrottleOptionsProvided()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", SingleRecordResponse);
        var throttleDelays = new List<TimeSpan>();
        var client = CreateClient(handler, throttleDelays: throttleDelays);

        // Act
        await ConsumeAsync(client, differentStart: null);

        // Assert — default 1s throttle delay
        await Assert.That(throttleDelays).HasCount().EqualTo(1);
        await Assert.That(throttleDelays[0]).IsEqualTo(TimeSpan.FromSeconds(1));
    }

    [Test]
    public async Task Should_DefaultToEmptyString_When_DescriptionIsNull()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", NullDescriptionResponse);
        var client = CreateClient(handler);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert
        await Assert.That(results).HasCount().EqualTo(1);
        await Assert.That(results[0].Description).IsEqualTo(string.Empty);
    }

    [Test]
    [NotInParallel]
    public async Task Should_RecordHttpErrorMetric_When_ApiReturns429()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupRateLimitThenSuccess("page=1", count: 2, SingleRecordResponse);
        var client = CreateClient(handler, retryOptions: new PlanItRetryOptions { MaxRetries = 3, BaseDelaySeconds = 0.001 });

        var recorded = new List<(long Value, int StatusCode, int AuthorityCode)>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.planit.http_errors")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            var statusCode = 0;
            var authorityCode = 0;
            foreach (var tag in tags)
            {
                if (tag.Key == "http.response.status_code")
                {
                    statusCode = (int)tag.Value!;
                }

                if (tag.Key == "planit.authority_code")
                {
                    authorityCode = (int)tag.Value!;
                }
            }

            recorded.Add((measurement, statusCode, authorityCode));
        });
        listener.Start();

        // Act
        await ConsumeAsync(client, differentStart: null, authorityId: 292);

        // Assert — 2 rate limit responses recorded
        await Assert.That(recorded).HasCount().EqualTo(2);
        await Assert.That(recorded[0].StatusCode).IsEqualTo(429);
        await Assert.That(recorded[0].AuthorityCode).IsEqualTo(292);
        await Assert.That(recorded[1].StatusCode).IsEqualTo(429);
        await Assert.That(recorded[1].AuthorityCode).IsEqualTo(292);
    }

    [Test]
    [NotInParallel]
    public async Task Should_RecordHttpErrorMetric_When_ApiReturns500()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupStatusCodeResponse("page=1", HttpStatusCode.InternalServerError);
        var client = CreateClient(handler);

        var recorded = new List<(long Value, int StatusCode, int AuthorityCode)>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.planit.http_errors")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            var statusCode = 0;
            var authorityCode = 0;
            foreach (var tag in tags)
            {
                if (tag.Key == "http.response.status_code")
                {
                    statusCode = (int)tag.Value!;
                }

                if (tag.Key == "planit.authority_code")
                {
                    authorityCode = (int)tag.Value!;
                }
            }

            recorded.Add((measurement, statusCode, authorityCode));
        });
        listener.Start();

        // Act & Assert — EnsureSuccessStatusCode throws, but metric should still be recorded
        await Assert.ThrowsAsync<HttpRequestException>(
            async () => await ConsumeAsync(client, differentStart: null, authorityId: 314));

        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].StatusCode).IsEqualTo(500);
        await Assert.That(recorded[0].AuthorityCode).IsEqualTo(314);
    }

    [Test]
    [NotInParallel]
    public async Task Should_NotRecordHttpErrorMetric_When_ApiReturns200()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", SingleRecordResponse);
        var client = CreateClient(handler);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.planit.http_errors")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await ConsumeAsync(client, differentStart: null);

        // Assert — no errors recorded for successful response
        await Assert.That(recorded).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_UseRetryAfterDelay_When_429ResponseHasDeltaSecondsHeader()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupRateLimitWithRetryAfter("page=1", count: 1, SingleRecordResponse, retryAfterValue: "10");
        var backoffDelays = new List<TimeSpan>();
        var options = new PlanItRetryOptions { MaxRetries = 3, BaseDelaySeconds = 1 };
        var client = CreateClient(handler, retryOptions: options, delays: backoffDelays);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert — got results after retry
        await Assert.That(results).HasCount().EqualTo(1);

        // The backoff delay should be exactly 10 seconds (from Retry-After header),
        // not the exponential backoff of ~1s (base delay * 2^0)
        await Assert.That(backoffDelays).HasCount().EqualTo(1);
        await Assert.That(backoffDelays[0]).IsEqualTo(TimeSpan.FromSeconds(10));
    }

    private static PlanItClient CreateClient(
        FakePlanItHandler handler,
        PlanItRetryOptions? retryOptions = null,
        PlanItThrottleOptions? throttleOptions = null,
        List<TimeSpan>? delays = null,
        List<TimeSpan>? throttleDelays = null)
    {
        var httpClient = new HttpClient(handler, disposeHandler: false)
        {
            BaseAddress = new Uri(BaseUrl),
        };

        // When tracking backoff delays but not throttle delays, disable throttle
        // so throttle delays don't pollute the backoff delay list.
        throttleOptions ??= delays is not null && throttleDelays is null
            ? new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 }
            : new PlanItThrottleOptions();

        Func<TimeSpan, CancellationToken, Task>? delayFunc = null;
        if (delays is not null || throttleDelays is not null)
        {
            delayFunc = (delay, _) =>
            {
                delays?.Add(delay);
                throttleDelays?.Add(delay);
                return Task.CompletedTask;
            };
        }

        return new PlanItClient(httpClient, retryOptions ?? new PlanItRetryOptions(), throttleOptions, delayFunc);
    }

    private static async Task<List<TownCrier.Domain.PlanningApplications.PlanningApplication>> ConsumeAsync(
        PlanItClient client,
        DateTimeOffset? differentStart,
        int authorityId = 292)
    {
        var results = new List<TownCrier.Domain.PlanningApplications.PlanningApplication>();
        await foreach (var app in client.FetchApplicationsAsync(authorityId, differentStart, CancellationToken.None))
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
                "pg_sz": 100,
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
