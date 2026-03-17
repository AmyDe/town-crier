using System.Text.Json.Serialization;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Health;
using TownCrier.Application.Notifications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Search;
using TownCrier.Application.UserProfiles;

namespace TownCrier.Web;

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
internal sealed partial class AppJsonSerializerContext : JsonSerializerContext;
