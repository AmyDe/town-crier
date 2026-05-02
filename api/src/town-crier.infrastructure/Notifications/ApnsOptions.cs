namespace TownCrier.Infrastructure.Notifications;

/// <summary>
/// Subset of APNs sender configuration consumed by
/// <see cref="ApnsPushNotificationSender"/>. Full configuration binding,
/// validation, and DI swap-out behind the <c>Apns:Enabled</c> flag are wired
/// in tc-fqun.6 — this minimal shape keeps the sender's surface narrow.
/// </summary>
public sealed class ApnsOptions
{
    // S1075: APNs endpoints are well-known Apple-published URLs, not configurable infrastructure.
#pragma warning disable S1075
    private const string ProductionUrl = "https://api.push.apple.com";
    private const string SandboxUrl = "https://api.sandbox.push.apple.com";
#pragma warning restore S1075

    /// <summary>
    /// Gets or sets the iOS app's bundle identifier. Sent on every request as
    /// the <c>apns-topic</c> header so APNs can route the push to the right
    /// app on the device.
    /// </summary>
    public string BundleId { get; set; } = string.Empty;

    /// <summary>
    /// Gets or sets a value indicating whether the sender targets the APNs
    /// sandbox environment (TestFlight / development builds) or production
    /// (App Store builds). The two environments use distinct base URLs and
    /// reject tokens minted against the other.
    /// </summary>
    public bool UseSandbox { get; set; }

    /// <summary>
    /// Gets or sets the maximum number of concurrent in-flight APNs requests.
    /// APNs is request-per-token, so a small cap is enough to keep throughput
    /// healthy without flooding Apple. Defaults to 10.
    /// </summary>
    public int MaxParallelism { get; set; } = 10;

    /// <summary>
    /// Resolves the APNs base URL for the configured environment.
    /// </summary>
    /// <returns>The sandbox URL when <see cref="UseSandbox"/> is true, otherwise the production URL.</returns>
    public Uri ResolveBaseAddress()
    {
        return new Uri(this.UseSandbox ? SandboxUrl : ProductionUrl);
    }
}
