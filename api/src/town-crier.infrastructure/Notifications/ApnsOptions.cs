using System.Globalization;
using Microsoft.Extensions.Configuration;

namespace TownCrier.Infrastructure.Notifications;

/// <summary>
/// Configuration for the APNs push notification sender. Bound from
/// <c>IConfiguration</c> section <c>"Apns"</c>. When <see cref="Enabled"/> is
/// false the host registers <see cref="NoOpPushNotificationSender"/> so the
/// app boots cleanly in local dev without a real .p8 auth key. When true, the
/// host registers <see cref="ApnsPushNotificationSender"/> + a singleton
/// <see cref="ApnsJwtProvider"/> + a named HTTP/2 <c>HttpClient</c>.
/// </summary>
public sealed class ApnsOptions
{
    /// <summary>
    /// The configuration section name bound by <see cref="LoadFromConfiguration"/>.
    /// </summary>
    public const string SectionName = "Apns";

    /// <summary>
    /// The required length of an Apple Key ID and Team ID. Apple issues both as
    /// fixed 10-character identifiers; any other length is a misconfiguration.
    /// </summary>
    private const int AppleIdLength = 10;

    // S1075: APNs endpoints are well-known Apple-published URLs, not configurable infrastructure.
#pragma warning disable S1075
    private const string ProductionUrl = "https://api.push.apple.com";
    private const string SandboxUrl = "https://api.sandbox.push.apple.com";
#pragma warning restore S1075

    /// <summary>
    /// Gets or sets a value indicating whether the APNs sender is enabled.
    /// When false, the host wires a no-op sender so missing auth keys are not
    /// fatal in local dev. When true, the host wires the real APNs sender
    /// and validates that all auth fields are populated.
    /// </summary>
    public bool Enabled { get; set; }

    /// <summary>
    /// Gets or sets the PEM-encoded contents of the .p8 APNs auth key issued
    /// by Apple. Required when <see cref="Enabled"/> is true; loaded once at
    /// startup into <see cref="ApnsJwtProvider"/> for ES256 JWT signing.
    /// </summary>
    public string AuthKey { get; set; } = string.Empty;

    /// <summary>
    /// Gets or sets the 10-character Apple Key ID associated with the .p8
    /// auth key. Carried in the JWT header's <c>kid</c> claim.
    /// </summary>
    public string KeyId { get; set; } = string.Empty;

    /// <summary>
    /// Gets or sets the 10-character Apple Team ID. Carried in the JWT
    /// payload's <c>iss</c> claim.
    /// </summary>
    public string TeamId { get; set; } = string.Empty;

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
    /// Loads APNs options from the <c>"Apns"</c> section of an
    /// <see cref="IConfiguration"/>. Native AOT-friendly: properties are read
    /// directly from <see cref="IConfiguration"/> indexers, no reflection-based
    /// <see cref="ConfigurationBinder"/> path.
    /// </summary>
    /// <param name="configuration">The configuration root or section to read from.</param>
    /// <returns>A populated <see cref="ApnsOptions"/>; defaults are kept when keys are missing.</returns>
    public static ApnsOptions LoadFromConfiguration(IConfiguration configuration)
    {
        ArgumentNullException.ThrowIfNull(configuration);

        var section = configuration.GetSection(SectionName);
        return new ApnsOptions
        {
            Enabled = ReadBool(section, nameof(Enabled), defaultValue: false),
            AuthKey = section[nameof(AuthKey)] ?? string.Empty,
            KeyId = section[nameof(KeyId)] ?? string.Empty,
            TeamId = section[nameof(TeamId)] ?? string.Empty,
            BundleId = section[nameof(BundleId)] ?? string.Empty,
            UseSandbox = ReadBool(section, nameof(UseSandbox), defaultValue: false),
            MaxParallelism = ReadInt(section, nameof(MaxParallelism), defaultValue: 10),
        };
    }

    /// <summary>
    /// Resolves the APNs base URL for the configured environment.
    /// </summary>
    /// <returns>The sandbox URL when <see cref="UseSandbox"/> is true, otherwise the production URL.</returns>
    public Uri ResolveBaseAddress()
    {
        return new Uri(this.UseSandbox ? SandboxUrl : ProductionUrl);
    }

    /// <summary>
    /// Validates the options. When <see cref="Enabled"/> is true, all of
    /// <see cref="AuthKey"/>, <see cref="KeyId"/>, <see cref="TeamId"/>, and
    /// <see cref="BundleId"/> must be non-empty, and <see cref="KeyId"/> /
    /// <see cref="TeamId"/> must be exactly 10 characters (Apple's fixed
    /// identifier length). When <see cref="Enabled"/> is false this method
    /// is a no-op so local dev hosts can boot without auth fields.
    /// </summary>
    /// <exception cref="InvalidOperationException">Thrown when validation fails.</exception>
    public void Validate()
    {
        if (!this.Enabled)
        {
            return;
        }

        if (string.IsNullOrWhiteSpace(this.AuthKey))
        {
            throw new InvalidOperationException(
                $"Apns:Enabled is true but {nameof(this.AuthKey)} is empty. Set Apns:AuthKey to the PEM contents of the .p8 file.");
        }

        if (string.IsNullOrWhiteSpace(this.KeyId))
        {
            throw new InvalidOperationException(
                $"Apns:Enabled is true but {nameof(this.KeyId)} is empty. Set Apns:KeyId to the 10-character Apple Key ID.");
        }

        if (this.KeyId.Length != AppleIdLength)
        {
            throw new InvalidOperationException(
                $"Apns:{nameof(this.KeyId)} must be exactly {AppleIdLength} characters (Apple's fixed Key ID length). Got {this.KeyId.Length}.");
        }

        if (string.IsNullOrWhiteSpace(this.TeamId))
        {
            throw new InvalidOperationException(
                $"Apns:Enabled is true but {nameof(this.TeamId)} is empty. Set Apns:TeamId to the 10-character Apple Team ID.");
        }

        if (this.TeamId.Length != AppleIdLength)
        {
            throw new InvalidOperationException(
                $"Apns:{nameof(this.TeamId)} must be exactly {AppleIdLength} characters (Apple's fixed Team ID length). Got {this.TeamId.Length}.");
        }

        if (string.IsNullOrWhiteSpace(this.BundleId))
        {
            throw new InvalidOperationException(
                $"Apns:Enabled is true but {nameof(this.BundleId)} is empty. Set Apns:BundleId to the iOS app's bundle identifier.");
        }
    }

    private static bool ReadBool(IConfigurationSection section, string key, bool defaultValue)
    {
        var raw = section[key];
        return string.IsNullOrWhiteSpace(raw) || !bool.TryParse(raw, out var parsed) ? defaultValue : parsed;
    }

    private static int ReadInt(IConfigurationSection section, string key, int defaultValue)
    {
        var raw = section[key];
        return string.IsNullOrWhiteSpace(raw)
            || !int.TryParse(raw, NumberStyles.Integer, CultureInfo.InvariantCulture, out var parsed)
                ? defaultValue
                : parsed;
    }
}
