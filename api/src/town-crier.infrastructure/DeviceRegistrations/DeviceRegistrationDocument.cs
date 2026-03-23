using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Infrastructure.DeviceRegistrations;

internal sealed class DeviceRegistrationDocument
{
    public required string Id { get; init; }

    public required string UserId { get; init; }

    public required string Token { get; init; }

    public required string Platform { get; init; }

    public required DateTimeOffset RegisteredAt { get; init; }

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
        };
    }

    public DeviceRegistration ToDomain()
    {
        var platform = Enum.Parse<DevicePlatform>(this.Platform);
        return DeviceRegistration.Create(this.UserId, this.Token, platform, this.RegisteredAt);
    }
}
