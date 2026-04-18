namespace TownCrier.Infrastructure.Cosmos;

/// <summary>
/// Central constants for Cosmos DB database and container names.
/// All repository adapters must reference these constants rather than hardcoding strings.
/// </summary>
public static class CosmosContainerNames
{
    public const string DatabaseName = "town-crier";

    public const string Users = "Users";
    public const string Notifications = "Notifications";
    public const string DeviceRegistrations = "DeviceRegistrations";
    public const string DecisionAlerts = "DecisionAlerts";
    public const string SavedApplications = "SavedApplications";
    public const string WatchZones = "WatchZones";
    public const string Applications = "Applications";
    public const string PollState = "PollState";
    public const string OfferCodes = "OfferCodes";
}
