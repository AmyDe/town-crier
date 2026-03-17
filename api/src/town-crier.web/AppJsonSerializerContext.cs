using System.Text.Json.Serialization;
using TownCrier.Application.Authorities;
using TownCrier.Application.Designations;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Health;
using TownCrier.Application.Notifications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Search;
using TownCrier.Application.UserProfiles;

namespace TownCrier.Web;

[JsonSerializable(typeof(GetAuthoritiesResult))]
[JsonSerializable(typeof(GetAuthorityByIdResult))]
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
[JsonSerializable(typeof(SearchPlanningApplicationsResult))]
[JsonSerializable(typeof(GetNotificationsResult))]
[JsonSerializable(typeof(IReadOnlyList<SavedApplicationResult>))]
[JsonSerializable(typeof(UpdateZonePreferencesCommand))]
[JsonSerializable(typeof(UpdateZonePreferencesResult))]
[JsonSerializable(typeof(GetZonePreferencesResult))]
internal sealed partial class AppJsonSerializerContext : JsonSerializerContext;
