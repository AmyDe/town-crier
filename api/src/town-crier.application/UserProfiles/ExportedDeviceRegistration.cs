using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.UserProfiles;

public sealed record ExportedDeviceRegistration(
    string Token,
    DevicePlatform Platform,
    DateTimeOffset RegisteredAt);
