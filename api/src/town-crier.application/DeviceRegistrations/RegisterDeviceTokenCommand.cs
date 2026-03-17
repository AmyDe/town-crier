using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.DeviceRegistrations;

public sealed record RegisterDeviceTokenCommand(string UserId, string Token, DevicePlatform Platform);
