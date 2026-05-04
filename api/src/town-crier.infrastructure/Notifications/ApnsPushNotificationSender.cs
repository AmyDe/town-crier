using System.Net;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json;
using Microsoft.Extensions.Logging;
using TownCrier.Application.Notifications;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;

namespace TownCrier.Infrastructure.Notifications;

/// <summary>
/// Direct APNs HTTP/2 push sender. Posts one request per device token, attaches
/// an ES256-signed provider JWT (cached by <see cref="ApnsJwtProvider"/>), and
/// reports tokens that APNs has rejected as permanently invalid (410 Unregistered,
/// 400 BadDeviceToken) for the handler to prune.
/// </summary>
public sealed class ApnsPushNotificationSender : IPushNotificationSender
{
    private const int MaxAttempts = 3;
    private static readonly TimeSpan InitialBackoff = TimeSpan.FromMilliseconds(100);

    private readonly HttpClient httpClient;
    private readonly ApnsJwtProvider jwtProvider;
    private readonly ApnsOptions options;
    private readonly ILogger<ApnsPushNotificationSender> logger;
    private readonly TimeProvider timeProvider;

    public ApnsPushNotificationSender(
        HttpClient httpClient,
        ApnsJwtProvider jwtProvider,
        ApnsOptions options,
        ILogger<ApnsPushNotificationSender> logger,
        TimeProvider timeProvider)
    {
        ArgumentNullException.ThrowIfNull(httpClient);
        ArgumentNullException.ThrowIfNull(jwtProvider);
        ArgumentNullException.ThrowIfNull(options);
        ArgumentNullException.ThrowIfNull(logger);
        ArgumentNullException.ThrowIfNull(timeProvider);

        this.httpClient = httpClient;
        this.jwtProvider = jwtProvider;
        this.options = options;
        this.logger = logger;
        this.timeProvider = timeProvider;
    }

    public async Task<PushSendResult> SendAsync(
        Notification notification,
        IReadOnlyList<DeviceRegistration> devices,
        int totalUnreadCount,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(notification);
        ArgumentNullException.ThrowIfNull(devices);

        if (devices.Count == 0)
        {
            return PushSendResult.Empty;
        }

        var payload = BuildAlertPayload(notification, totalUnreadCount);
        return await this.SendToDevicesAsync(devices, payload, ApnsJsonSerializerContext.Default.ApnsAlertPayload, ct).ConfigureAwait(false);
    }

    public async Task<PushSendResult> SendDigestAsync(
        int applicationCount,
        int totalUnreadCount,
        IReadOnlyList<DeviceRegistration> devices,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(devices);

        if (devices.Count == 0)
        {
            return PushSendResult.Empty;
        }

        var payload = BuildDigestPayload(applicationCount, totalUnreadCount);
        return await this.SendToDevicesAsync(devices, payload, ApnsJsonSerializerContext.Default.ApnsDigestPayload, ct).ConfigureAwait(false);
    }

    private static ApnsAlertPayload BuildAlertPayload(Notification notification, int totalUnreadCount)
    {
        var title = notification.WatchZoneId is not null
            ? "Planning update near you"
            : "Town Crier";
        var bodyText = notification.EventType == NotificationEventType.DecisionUpdate
            ? BuildDecisionBody(notification)
            : notification.ApplicationAddress;

        return new ApnsAlertPayload(
            new ApnsAlertAps(
                new ApnsAlertContent(title, bodyText),
                Sound: "default",
                Badge: totalUnreadCount),
            NotificationId: notification.Id,
            ApplicationRef: notification.ApplicationName,
            CreatedAt: notification.CreatedAt);
    }

    private static string BuildDecisionBody(Notification notification)
    {
        var label = UkPlanningVocabulary.GetDisplayString(notification.Decision);
        return string.IsNullOrEmpty(label)
            ? notification.ApplicationAddress
            : $"{notification.ApplicationAddress} — {label}";
    }

    private static ApnsDigestPayload BuildDigestPayload(int applicationCount, int totalUnreadCount)
    {
        var body = $"{applicationCount} new application{(applicationCount == 1 ? string.Empty : "s")} this week";
        return new ApnsDigestPayload(
            new ApnsDigestAps(
                new ApnsAlertContent("Town Crier", body),
                Sound: "default",
                Badge: totalUnreadCount));
    }

    private static async Task<string?> ReadReasonAsync(HttpResponseMessage response, CancellationToken ct)
    {
        try
        {
            var body = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
            if (string.IsNullOrEmpty(body))
            {
                return null;
            }

            var error = JsonSerializer.Deserialize(body, ApnsJsonSerializerContext.Default.ApnsErrorResponse);
            return error?.Reason;
        }
#pragma warning disable CA1031 // Best-effort parse — APNs error bodies are tiny and well-shaped.
        catch (Exception)
#pragma warning restore CA1031
        {
            return null;
        }
    }

    private async Task<PushSendResult> SendToDevicesAsync<TPayload>(
        IReadOnlyList<DeviceRegistration> devices,
        TPayload payload,
        System.Text.Json.Serialization.Metadata.JsonTypeInfo<TPayload> jsonTypeInfo,
        CancellationToken ct)
    {
        var parallelism = Math.Max(1, this.options.MaxParallelism);
        using var gate = new SemaphoreSlim(parallelism, parallelism);
        var invalid = new List<string>();
        var lockObj = new Lock();
        var tasks = new List<Task>(devices.Count);

        foreach (var device in devices)
        {
            await gate.WaitAsync(ct).ConfigureAwait(false);
            tasks.Add(Task.Run(
                async () =>
                {
                    try
                    {
                        var rejected = await this.SendOneAsync(device, payload, jsonTypeInfo, ct).ConfigureAwait(false);
                        if (rejected)
                        {
                            lock (lockObj)
                            {
                                invalid.Add(device.Token);
                            }
                        }
                    }
                    finally
                    {
                        gate.Release();
                    }
                },
                ct));
        }

        await Task.WhenAll(tasks).ConfigureAwait(false);
        return invalid.Count == 0 ? PushSendResult.Empty : new PushSendResult(invalid);
    }

    private async Task<bool> SendOneAsync<TPayload>(
        DeviceRegistration device,
        TPayload payload,
        System.Text.Json.Serialization.Metadata.JsonTypeInfo<TPayload> jsonTypeInfo,
        CancellationToken ct)
    {
        var attempt = 0;
        var backoff = InitialBackoff;
        var jwtRefreshed = false;

        while (true)
        {
            attempt++;
            using var request = this.BuildRequest(device, payload, jsonTypeInfo);
            HttpResponseMessage? response = null;
            try
            {
                response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

                if (response.IsSuccessStatusCode)
                {
                    return false;
                }

                var status = (int)response.StatusCode;
                var reason = await ReadReasonAsync(response, ct).ConfigureAwait(false);

                if (response.StatusCode == HttpStatusCode.Gone)
                {
                    ApnsLog.TokenUnregistered(this.logger, device.Token);
                    return true;
                }

                if (response.StatusCode == HttpStatusCode.BadRequest && string.Equals(reason, "BadDeviceToken", StringComparison.Ordinal))
                {
                    ApnsLog.BadDeviceToken(this.logger, device.Token);
                    return true;
                }

                if (response.StatusCode == HttpStatusCode.Forbidden && string.Equals(reason, "ExpiredProviderToken", StringComparison.Ordinal) && !jwtRefreshed)
                {
                    ApnsLog.ExpiredProviderToken(this.logger);
                    this.jwtProvider.Invalidate();
                    jwtRefreshed = true;
                    continue;
                }

                if (response.StatusCode == HttpStatusCode.TooManyRequests && string.Equals(reason, "TooManyProviderTokenUpdates", StringComparison.Ordinal))
                {
                    ApnsLog.TooManyProviderTokenUpdates(this.logger);
                    return false;
                }

                if (status >= 500 && status < 600 && attempt < MaxAttempts)
                {
                    ApnsLog.TransientError(this.logger, status, reason ?? string.Empty, attempt);
                    await Task.Delay(backoff, this.timeProvider, ct).ConfigureAwait(false);
                    backoff = TimeSpan.FromTicks(backoff.Ticks * 2);
                    continue;
                }

                ApnsLog.UnhandledStatus(this.logger, status, reason ?? string.Empty, device.Token);
                return false;
            }
#pragma warning disable CA1031 // A failure on one device must not abort the rest.
            catch (HttpRequestException ex) when (attempt < MaxAttempts)
            {
                ApnsLog.HttpException(this.logger, attempt, ex);
                await Task.Delay(backoff, this.timeProvider, ct).ConfigureAwait(false);
                backoff = TimeSpan.FromTicks(backoff.Ticks * 2);
                continue;
            }
            catch (Exception ex)
            {
                ApnsLog.SendFailed(this.logger, device.Token, ex);
                return false;
            }
#pragma warning restore CA1031
            finally
            {
                response?.Dispose();
            }
        }
    }

    private HttpRequestMessage BuildRequest<TPayload>(
        DeviceRegistration device,
        TPayload payload,
        System.Text.Json.Serialization.Metadata.JsonTypeInfo<TPayload> jsonTypeInfo)
    {
        var request = new HttpRequestMessage(HttpMethod.Post, $"/3/device/{device.Token}")
        {
            Version = HttpVersion.Version20,
            VersionPolicy = HttpVersionPolicy.RequestVersionOrHigher,
            Content = JsonContent.Create(payload, jsonTypeInfo),
        };
        request.Headers.Authorization = new AuthenticationHeaderValue("bearer", this.jwtProvider.Current());
        request.Headers.TryAddWithoutValidation("apns-topic", this.options.BundleId);
        request.Headers.TryAddWithoutValidation("apns-push-type", "alert");
        request.Headers.TryAddWithoutValidation("apns-priority", "10");
        request.Headers.TryAddWithoutValidation("apns-expiration", "0");
        return request;
    }
}
