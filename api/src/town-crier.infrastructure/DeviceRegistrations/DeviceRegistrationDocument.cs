using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Infrastructure.DeviceRegistrations;

internal sealed class DeviceRegistrationDocument
{
    // Cosmos TTL in seconds. Device registrations that stop refreshing
    // (app uninstalled, logged out) are purged after 180 days so the push
    // token store doesn't accumulate permanently-stale records. Active
    // clients re-upsert on every PUT /me/device-token, which resets _ts.
    // Enforces UK GDPR Art. 5(1)(e) storage limitation for device identifiers.
    private const int OneHundredEightyDaysInSeconds = 180 * 24 * 60 * 60;

    public required string Id { get; init; }

    public required string UserId { get; init; }

    public required string Token { get; init; }

    public required string Platform { get; init; }

    public required DateTimeOffset RegisteredAt { get; init; }

    public int Ttl { get; init; } = OneHundredEightyDaysInSeconds;

    public static DeviceRegistrationDocument FromDomain(DeviceRegistration registration)
    {
        ArgumentNullException.ThrowIfNull(registration);

        return new DeviceRegistrationDocument
        {
            Id = registration.Token,
            UserId = registration.UserId,
            Token = registration.Token,
            Platform = registration.Platform.ToString(),
            RegisteredAt = registration.RegisteredAt,
            Ttl = OneHundredEightyDaysInSeconds,
        };
    }

    public DeviceRegistration ToDomain()
    {
        var platform = Enum.Parse<DevicePlatform>(this.Platform);
        return DeviceRegistration.Create(this.UserId, this.Token, platform, this.RegisteredAt);
    }
}
