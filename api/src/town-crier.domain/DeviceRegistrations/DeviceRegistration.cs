namespace TownCrier.Domain.DeviceRegistrations;

public sealed class DeviceRegistration
{
    private DeviceRegistration(string userId, string token, DevicePlatform platform, DateTimeOffset registeredAt)
    {
        this.UserId = userId;
        this.Token = token;
        this.Platform = platform;
        this.RegisteredAt = registeredAt;
    }

    public string UserId { get; }

    public string Token { get; }

    public DevicePlatform Platform { get; }

    public DateTimeOffset RegisteredAt { get; private set; }

    public static DeviceRegistration Create(string userId, string token, DevicePlatform platform, DateTimeOffset now)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        ArgumentException.ThrowIfNullOrWhiteSpace(token);

        return new DeviceRegistration(userId, token, platform, now);
    }

    public void RefreshRegistration(DateTimeOffset now)
    {
        this.RegisteredAt = now;
    }
}
