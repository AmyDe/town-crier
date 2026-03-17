using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.DeviceRegistrations;

public sealed record RegisterDeviceTokenRequest(string Token, DevicePlatform Platform);
