using System.Text.Json.Serialization;
using TownCrier.Application.Admin;
using TownCrier.Application.Authorities;
using TownCrier.Application.DemoAccount;
using TownCrier.Application.Designations;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Health;
using TownCrier.Application.Legal;
using TownCrier.Application.Notifications;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Search;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.VersionConfig;
using TownCrier.Application.WatchZones;
using TownCrier.Web.Endpoints;

namespace TownCrier.Web;

[JsonSerializable(typeof(ApiErrorResponse))]
[JsonSerializable(typeof(EntitlementErrorResponse))]
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
[JsonSerializable(typeof(ExportedNotificationPreferences))]
[JsonSerializable(typeof(ExportedZonePreferences))]
[JsonSerializable(typeof(ExportedSubscription))]
[JsonSerializable(typeof(ExportedWatchZone))]
[JsonSerializable(typeof(ExportedNotification))]
[JsonSerializable(typeof(ExportedDecisionAlert))]
[JsonSerializable(typeof(ExportedSavedApplication))]
[JsonSerializable(typeof(ExportedDeviceRegistration))]
[JsonSerializable(typeof(ExportedOfferCodeRedemption))]
[JsonSerializable(typeof(IReadOnlyList<ExportedZonePreferences>))]
[JsonSerializable(typeof(IReadOnlyList<ExportedWatchZone>))]
[JsonSerializable(typeof(IReadOnlyList<ExportedNotification>))]
[JsonSerializable(typeof(IReadOnlyList<ExportedDecisionAlert>))]
[JsonSerializable(typeof(IReadOnlyList<ExportedSavedApplication>))]
[JsonSerializable(typeof(IReadOnlyList<ExportedDeviceRegistration>))]
[JsonSerializable(typeof(IReadOnlyList<ExportedOfferCodeRedemption>))]
[JsonSerializable(typeof(RegisterDeviceTokenRequest))]
[JsonSerializable(typeof(RemoveInvalidDeviceTokenRequest))]
[JsonSerializable(typeof(PlanningApplicationResult))]
[JsonSerializable(typeof(IReadOnlyList<PlanningApplicationResult>))]
[JsonSerializable(typeof(SearchPlanningApplicationsResult))]
[JsonSerializable(typeof(GetUserApplicationAuthoritiesResult))]
[JsonSerializable(typeof(GetNotificationsResult))]
[JsonSerializable(typeof(IReadOnlyList<SavedApplicationResult>))]
[JsonSerializable(typeof(CreateWatchZoneRequest))]
[JsonSerializable(typeof(CreateWatchZoneResult))]
[JsonSerializable(typeof(UpdateWatchZoneRequest))]
[JsonSerializable(typeof(UpdateWatchZoneResult))]
[JsonSerializable(typeof(ListWatchZonesResult))]
[JsonSerializable(typeof(UpdateZonePreferencesCommand))]
[JsonSerializable(typeof(UpdateZonePreferencesResult))]
[JsonSerializable(typeof(GetZonePreferencesResult))]
[JsonSerializable(typeof(GetVersionConfigResult))]
[JsonSerializable(typeof(GetLegalDocumentResult))]
[JsonSerializable(typeof(GrantSubscriptionCommand))]
[JsonSerializable(typeof(GrantSubscriptionResult))]
[JsonSerializable(typeof(ListUsersResult))]
[JsonSerializable(typeof(GenerateOfferCodesRequest))]
[JsonSerializable(typeof(RedeemOfferCodeRequest))]
[JsonSerializable(typeof(RedeemOfferCodeResponse))]
internal sealed partial class AppJsonSerializerContext : JsonSerializerContext;
