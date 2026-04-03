using System.Text.Json.Serialization;
using TownCrier.Application.Admin;
using TownCrier.Application.Authorities;
using TownCrier.Application.DemoAccount;
using TownCrier.Application.Designations;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Health;
using TownCrier.Application.Notifications;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Search;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.VersionConfig;
using TownCrier.Application.WatchZones;

namespace TownCrier.Web;

[JsonSerializable(typeof(ApiErrorResponse))]
[JsonSerializable(typeof(UserIdResponse))]
[JsonSerializable(typeof(GetAuthoritiesResult))]
[JsonSerializable(typeof(GetAuthorityByIdResult))]
[JsonSerializable(typeof(GetDemoAccountResult))]
[JsonSerializable(typeof(GetDesignationContextResult))]
[JsonSerializable(typeof(HealthStatus))]
[JsonSerializable(typeof(GeocodePostcodeResult))]
[JsonSerializable(typeof(CreateUserProfileResult))]
[JsonSerializable(typeof(GetUserProfileResult))]
[JsonSerializable(typeof(UpdateUserProfileCommand))]
[JsonSerializable(typeof(UpdateUserProfileResult))]
[JsonSerializable(typeof(ExportUserDataResult))]
[JsonSerializable(typeof(RegisterDeviceTokenRequest))]
[JsonSerializable(typeof(RemoveInvalidDeviceTokenRequest))]
[JsonSerializable(typeof(PlanningApplicationResult))]
[JsonSerializable(typeof(IReadOnlyList<PlanningApplicationResult>))]
[JsonSerializable(typeof(SearchPlanningApplicationsResult))]
[JsonSerializable(typeof(GetNotificationsResult))]
[JsonSerializable(typeof(IReadOnlyList<SavedApplicationResult>))]
[JsonSerializable(typeof(CreateWatchZoneRequest))]
[JsonSerializable(typeof(CreateWatchZoneResult))]
[JsonSerializable(typeof(ListWatchZonesResult))]
[JsonSerializable(typeof(UpdateZonePreferencesCommand))]
[JsonSerializable(typeof(UpdateZonePreferencesResult))]
[JsonSerializable(typeof(GetZonePreferencesResult))]
[JsonSerializable(typeof(GetVersionConfigResult))]
[JsonSerializable(typeof(GrantSubscriptionCommand))]
[JsonSerializable(typeof(GrantSubscriptionResult))]
[JsonSerializable(typeof(GetUserApplicationAuthoritiesResult))]
internal sealed partial class AppJsonSerializerContext : JsonSerializerContext;
