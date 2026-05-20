namespace TownCrier.Application.DeviceRegistrations;

public sealed record RemoveInvalidDeviceTokenCommand(string UserId, string Token);
