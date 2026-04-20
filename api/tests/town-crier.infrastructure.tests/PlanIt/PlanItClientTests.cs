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
    public async Task Should_ThrowAfterRetries_When_ApiReturns429Forever()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupRateLimitForever("page=1");
        var delays = new List<TimeSpan>();
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 };
        var client = CreateClient(handler, throttleOptions: throttleOptions, delays: delays);

        // Act & Assert — should throw after exhausting retries (default 3)
        await Assert.ThrowsAsync<HttpRequestException>(
            async () => await ConsumeAsync(client, differentStart: null));

        // 4 requests: 1 initial + 3 retries
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(4);
    }

    [Test]
    public async Task Should_PropagateNon429Errors_When_ServerReturns500()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupStatusCodeResponse("page=1", HttpStatusCode.InternalServerError);
        var client = CreateClient(handler);

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
        var client = CreateClient(handler, throttleOptions: throttleOptions, delays: throttleDelays);

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
        var client = CreateClient(handler, throttleOptions: throttleOptions, delays: throttleDelays);

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
        var client = CreateClient(handler, throttleOptions: throttleOptions, delays: throttleDelays);

        // Act
        await client.SearchApplicationsAsync("car park", 314, 1, CancellationToken.None);

        // Assert — one throttle delay before the search request
        await Assert.That(throttleDelays).HasCount().EqualTo(1);
        await Assert.That(throttleDelays[0]).IsEqualTo(TimeSpan.FromMilliseconds(300));
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
    public async Task Should_UseDefaultTwoSecondDelay_When_NoThrottleOptionsProvided()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", SingleRecordResponse);
        var throttleDelays = new List<TimeSpan>();
        var client = CreateClient(handler, delays: throttleDelays);

        // Act
        await ConsumeAsync(client, differentStart: null);

        // Assert — default 2s throttle delay
        await Assert.That(throttleDelays).HasCount().EqualTo(1);
        await Assert.That(throttleDelays[0]).IsEqualTo(TimeSpan.FromSeconds(2));
    }

    [Test]
    public async Task Should_UseAscendingSortOrder_When_FetchingApplicationsForPolling()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", EmptyResponse);
        var client = CreateClient(handler);

        // Act
        await ConsumeAsync(client, differentStart: null, authorityId: 292);

        // Assert — polling uses ascending sort for resumable progress
        await Assert.That(handler.RequestUrls[0]).Contains("sort=last_different");
        await Assert.That(handler.RequestUrls[0]).DoesNotContain("sort=-last_different");
    }

    [Test]
    public async Task Should_UseDescendingSortOrder_When_SearchingApplications()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("/api/applics/json", SingleRecordResponse);
        var client = CreateClient(handler);

        // Act
        await client.SearchApplicationsAsync("car park", 314, 1, CancellationToken.None);

        // Assert — search uses descending sort (newest first)
        await Assert.That(handler.RequestUrls[0]).Contains("sort=-last_different");
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
        handler.SetupRateLimitForever("page=1");
        var delays = new List<TimeSpan>();
        var client = CreateClient(handler, delays: delays);

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

        // Act — 429 throws after exhausting retries (default 3)
        await Assert.ThrowsAsync<HttpRequestException>(
            async () => await ConsumeAsync(client, differentStart: null, authorityId: 292));

        // Assert — 4 error metrics recorded (1 initial + 3 retries)
        await Assert.That(recorded).HasCount().EqualTo(4);
        await Assert.That(recorded[0].StatusCode).IsEqualTo(429);
        await Assert.That(recorded[0].AuthorityCode).IsEqualTo(292);
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
    public async Task Should_RetryAndSucceed_When_504OccursOnce()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupTransientFailure("page=1", failCount: 1, HttpStatusCode.GatewayTimeout, SingleRecordResponse);
        var delays = new List<TimeSpan>();
        var retryOptions = new PlanItRetryOptions { MaxRetries = 3 };
        var client = CreateClient(handler, retryOptions: retryOptions, delays: delays);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert — should succeed after retry
        await Assert.That(results).HasCount().EqualTo(1);
        await Assert.That(results[0].Name).IsEqualTo("Leeds/26/01471/TR");

        // 2 HTTP requests: 1 failure + 1 success (plus throttle delays)
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_RetryWithExponentialBackoff_When_504OccursTwice()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupTransientFailure("page=1", failCount: 2, HttpStatusCode.GatewayTimeout, SingleRecordResponse);
        var delays = new List<TimeSpan>();
        var retryOptions = new PlanItRetryOptions
        {
            MaxRetries = 3,
            InitialBackoffSeconds = 1,
        };
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 };
        var client = CreateClient(handler, throttleOptions: throttleOptions, retryOptions: retryOptions, delays: delays);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert — should succeed after 2 retries
        await Assert.That(results).HasCount().EqualTo(1);

        // 3 HTTP requests: 2 failures + 1 success
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(3);

        // Backoff delays: 1s, then 2s (exponential)
        // Filter out zero-length throttle delays to find retry delays
        var retryDelays = delays.Where(d => d > TimeSpan.Zero).ToList();
        await Assert.That(retryDelays).HasCount().EqualTo(2);
        await Assert.That(retryDelays[0]).IsEqualTo(TimeSpan.FromSeconds(1));
        await Assert.That(retryDelays[1]).IsEqualTo(TimeSpan.FromSeconds(2));
    }

    [Test]
    public async Task Should_ThrowAfterMaxRetries_When_504PersistsForever()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupStatusCodeResponse("page=1", HttpStatusCode.GatewayTimeout);
        var delays = new List<TimeSpan>();
        var retryOptions = new PlanItRetryOptions { MaxRetries = 2 };
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 };
        var client = CreateClient(handler, throttleOptions: throttleOptions, retryOptions: retryOptions, delays: delays);

        // Act & Assert — should throw after exhausting retries
        await Assert.ThrowsAsync<HttpRequestException>(
            async () => await ConsumeAsync(client, differentStart: null));

        // 3 HTTP requests: 1 initial + 2 retries
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(3);
    }

    [Test]
    public async Task Should_RetryOn429WithLongerBackoff_When_RateLimited()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupTransientRateLimit("page=1", failCount: 1, SingleRecordResponse);
        var delays = new List<TimeSpan>();
        var retryOptions = new PlanItRetryOptions
        {
            MaxRetries = 3,
            RateLimitBackoffSeconds = 5,
        };
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 };
        var client = CreateClient(handler, throttleOptions: throttleOptions, retryOptions: retryOptions, delays: delays);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert — should succeed after retry
        await Assert.That(results).HasCount().EqualTo(1);
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(2);

        // Rate limit backoff: 5s (first attempt)
        var retryDelays = delays.Where(d => d > TimeSpan.Zero).ToList();
        await Assert.That(retryDelays).HasCount().EqualTo(1);
        await Assert.That(retryDelays[0]).IsEqualTo(TimeSpan.FromSeconds(5));
    }

    [Test]
    public async Task Should_NotRetryOn400_When_BadRequest()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupStatusCodeResponse("page=1", HttpStatusCode.BadRequest);
        var retryOptions = new PlanItRetryOptions { MaxRetries = 3 };
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 };
        var client = CreateClient(handler, throttleOptions: throttleOptions, retryOptions: retryOptions);

        // Act & Assert — should throw immediately, no retries
        await Assert.ThrowsAsync<HttpRequestException>(
            async () => await ConsumeAsync(client, differentStart: null));

        // Only 1 request — 400 is not retryable
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_RetryOn502_When_BadGateway()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupTransientFailure("page=1", failCount: 1, HttpStatusCode.BadGateway, SingleRecordResponse);
        var delays = new List<TimeSpan>();
        var retryOptions = new PlanItRetryOptions { MaxRetries = 3 };
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 };
        var client = CreateClient(handler, throttleOptions: throttleOptions, retryOptions: retryOptions, delays: delays);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert — should succeed after retry
        await Assert.That(results).HasCount().EqualTo(1);
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_RetryOn503_When_ServiceUnavailable()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupTransientFailure("page=1", failCount: 1, HttpStatusCode.ServiceUnavailable, SingleRecordResponse);
        var delays = new List<TimeSpan>();
        var retryOptions = new PlanItRetryOptions { MaxRetries = 3 };
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 };
        var client = CreateClient(handler, throttleOptions: throttleOptions, retryOptions: retryOptions, delays: delays);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert — should succeed after retry
        await Assert.That(results).HasCount().EqualTo(1);
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_RetryOn408_When_RequestTimeout()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupTransientFailure("page=1", failCount: 1, HttpStatusCode.RequestTimeout, SingleRecordResponse);
        var delays = new List<TimeSpan>();
        var retryOptions = new PlanItRetryOptions { MaxRetries = 3 };
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 };
        var client = CreateClient(handler, throttleOptions: throttleOptions, retryOptions: retryOptions, delays: delays);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert — should succeed after retry
        await Assert.That(results).HasCount().EqualTo(1);
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_UseDefaultRetryOptions_When_NoneProvided()
    {
        // Arrange — no retry options provided, so the client should use defaults (3 retries)
        using var handler = new FakePlanItHandler();
        handler.SetupTransientFailure("page=1", failCount: 1, HttpStatusCode.GatewayTimeout, SingleRecordResponse);
        var delays = new List<TimeSpan>();
        var client = CreateClient(handler, delays: delays);

        // Act
        var results = await ConsumeAsync(client, differentStart: null);

        // Assert — should succeed after retry using default options
        await Assert.That(results).HasCount().EqualTo(1);
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_DisableRetries_When_MaxRetriesIsZero()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupStatusCodeResponse("page=1", HttpStatusCode.GatewayTimeout);
        var retryOptions = new PlanItRetryOptions { MaxRetries = 0 };
        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 };
        var client = CreateClient(handler, throttleOptions: throttleOptions, retryOptions: retryOptions);

        // Act & Assert — should throw immediately, no retries
        await Assert.ThrowsAsync<HttpRequestException>(
            async () => await ConsumeAsync(client, differentStart: null));

        // Only 1 request — retries disabled
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(1);
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
    public async Task Should_StopAtPageCap_When_MaxPagesProvidedMidPagination()
    {
        // Arrange — 5 pages of 100 records each (500 total), but cap at 3.
        // Each page carries a distinct last_different so the caller can compute
        // a high-water mark from the final streamed record.
        using var handler = new FakePlanItHandler();

        var page1Json = BuildResponseJson(CreateRecordsJson(100, lastDifferent: "2026-03-10T09:00:00.000"), total: 500);
        var page2Json = BuildResponseJson(CreateRecordsJson(100, startIndex: 100, lastDifferent: "2026-03-11T09:00:00.000"), total: 500, from: 100);
        var page3Json = BuildResponseJson(CreateRecordsJson(100, startIndex: 200, lastDifferent: "2026-03-12T09:00:00.000"), total: 500, from: 200);
        var page4Json = BuildResponseJson(CreateRecordsJson(100, startIndex: 300, lastDifferent: "2026-03-13T09:00:00.000"), total: 500, from: 300);
        var page5Json = BuildResponseJson(CreateRecordsJson(100, startIndex: 400, lastDifferent: "2026-03-14T09:00:00.000"), total: 500, from: 400);
        handler.SetupJsonResponse("page=1", page1Json);
        handler.SetupJsonResponse("page=2", page2Json);
        handler.SetupJsonResponse("page=3", page3Json);
        handler.SetupJsonResponse("page=4", page4Json);
        handler.SetupJsonResponse("page=5", page5Json);

        var client = CreateClient(handler);

        // Act
        var results = await ConsumeAsync(client, differentStart: null, maxPages: 3);

        // Assert — exactly 300 records returned, only 3 pages fetched
        await Assert.That(results).HasCount().EqualTo(300);
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(3);
        await Assert.That(handler.RequestUrls[0]).Contains("page=1");
        await Assert.That(handler.RequestUrls[1]).Contains("page=2");
        await Assert.That(handler.RequestUrls[2]).Contains("page=3");

        // The final streamed app (the 300th) must carry the page-3 last_different —
        // this is what the handler uses to advance its high-water mark.
        var expectedHwm = DateTimeOffset.Parse("2026-03-12T09:00:00.000", CultureInfo.InvariantCulture);
        await Assert.That(results[^1].LastDifferent).IsEqualTo(expectedHwm);
    }

    [Test]
    public async Task Should_PaginateUnbounded_When_MaxPagesIsNull()
    {
        // Arrange — 5 full pages of data (500 records). With maxPages=null we must
        // paginate to natural exit (page 6 returns fewer than DefaultPageSize=100).
        // This is the regression guard for the watched-cycle / Search paths.
        using var handler = new FakePlanItHandler();

        handler.SetupJsonResponse("page=1", BuildResponseJson(CreateRecordsJson(100), total: 450));
        handler.SetupJsonResponse("page=2", BuildResponseJson(CreateRecordsJson(100, startIndex: 100), total: 450, from: 100));
        handler.SetupJsonResponse("page=3", BuildResponseJson(CreateRecordsJson(100, startIndex: 200), total: 450, from: 200));
        handler.SetupJsonResponse("page=4", BuildResponseJson(CreateRecordsJson(100, startIndex: 300), total: 450, from: 300));
        handler.SetupJsonResponse("page=5", BuildResponseJson(CreateRecordsJson(50, startIndex: 400), total: 450, from: 400));

        var client = CreateClient(handler);

        // Act
        var results = await ConsumeAsync(client, differentStart: null, maxPages: null);

        // Assert — natural exit after the short page
        await Assert.That(results).HasCount().EqualTo(450);
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(5);
    }

    [Test]
    public async Task Should_ExitNaturally_When_MaxPagesNotReached()
    {
        // Arrange — 2 pages of data, cap=10 (cap well above actual page count).
        // The natural end-of-data short-page must still terminate pagination.
        using var handler = new FakePlanItHandler();

        handler.SetupJsonResponse("page=1", BuildResponseJson(CreateRecordsJson(100), total: 150));
        handler.SetupJsonResponse("page=2", BuildResponseJson(CreateRecordsJson(50, startIndex: 100), total: 150, from: 100));

        var client = CreateClient(handler);

        // Act
        var results = await ConsumeAsync(client, differentStart: null, maxPages: 10);

        // Assert — 2 page fetches, no call to page=3
        await Assert.That(results).HasCount().EqualTo(150);
        await Assert.That(handler.RequestUrls).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_ReturnHasMorePagesTrue_When_FullPageReturned()
    {
        // Arrange — 100 records (== DefaultPageSize) signals more pages may follow.
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", BuildResponseJson(CreateRecordsJson(100), total: 250));
        var client = CreateClient(handler);

        // Act
        var page = await client.FetchApplicationsPageAsync(
            authorityId: 292,
            differentStart: null,
            page: 1,
            ct: CancellationToken.None);

        // Assert
        await Assert.That(page.PageNumber).IsEqualTo(1);
        await Assert.That(page.Applications).HasCount().EqualTo(100);
        await Assert.That(page.Total).IsEqualTo(250);
        await Assert.That(page.HasMorePages).IsTrue();
    }

    [Test]
    public async Task Should_ReturnHasMorePagesFalse_When_PartialPage()
    {
        // Arrange — fewer records than DefaultPageSize signals end of data.
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", BuildResponseJson(CreateRecordsJson(42), total: 42));
        var client = CreateClient(handler);

        // Act
        var page = await client.FetchApplicationsPageAsync(
            authorityId: 292,
            differentStart: null,
            page: 1,
            ct: CancellationToken.None);

        // Assert
        await Assert.That(page.Applications).HasCount().EqualTo(42);
        await Assert.That(page.Total).IsEqualTo(42);
        await Assert.That(page.HasMorePages).IsFalse();
    }

    private static PlanItClient CreateClient(
        FakePlanItHandler handler,
        PlanItThrottleOptions? throttleOptions = null,
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

        return new PlanItClient(httpClient, throttleOptions, retryOptions, delayFunc);
    }

    // Legacy streaming helper — emulates the previous FetchApplicationsAsync
    // contract (including maxPages) on top of the new page-level port so the
    // bulk of these tests can stay behaviour-focused. The new HasMorePages
    // semantics are covered by dedicated tests further down.
    private static async Task<List<TownCrier.Domain.PlanningApplications.PlanningApplication>> ConsumeAsync(
        PlanItClient client,
        DateTimeOffset? differentStart,
        int authorityId = 292,
        int? maxPages = null)
    {
        var results = new List<TownCrier.Domain.PlanningApplications.PlanningApplication>();
        var pagesFetched = 0;
        var page = 1;
        while (true)
        {
            var pageResult = await client.FetchApplicationsPageAsync(authorityId, differentStart, page, CancellationToken.None);
            results.AddRange(pageResult.Applications);
            pagesFetched++;

            if (!pageResult.HasMorePages)
            {
                break;
            }

            if (maxPages.HasValue && pagesFetched >= maxPages.Value)
            {
                break;
            }

            page++;
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

    private static string CreateRecordsJson(int count, int startIndex = 0, string lastDifferent = "2026-03-14T11:59:17.642")
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
                    "last_different": "{{lastDifferent}}"
                }
                """;
            records.Append(record);
        }

        records.Append(']');
        return records.ToString();
    }
}
