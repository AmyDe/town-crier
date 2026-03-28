using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.Authorities;
using TownCrier.Application.DecisionAlerts;
using TownCrier.Application.DemoAccount;
using TownCrier.Application.Designations;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Groups;
using TownCrier.Application.Notifications;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Search;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Web.DependencyInjection;

namespace TownCrier.Web.Tests.DependencyInjection;

public sealed class ServiceRegistrationExtensionsTests
{
    [Test]
    public async Task Should_ResolveAllCommandHandlers_When_ApplicationServicesRegistered()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var scope = factory.Services.CreateScope();
        var provider = scope.ServiceProvider;

        // Act & Assert - all command handlers must be resolvable
        await Assert.That(provider.GetService<CreateUserProfileCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<UpdateUserProfileCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<DeleteUserProfileCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<CreateWatchZoneCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<DeleteWatchZoneCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<UpdateZonePreferencesCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<RegisterDeviceTokenCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<RemoveInvalidDeviceTokenCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<SaveApplicationCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<RemoveSavedApplicationCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<CreateGroupCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<InviteMemberCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<AcceptInvitationCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<RemoveGroupMemberCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<DeleteGroupCommandHandler>()).IsNotNull();
    }

    [Test]
    public async Task Should_ResolveAllQueryHandlers_When_ApplicationServicesRegistered()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var scope = factory.Services.CreateScope();
        var provider = scope.ServiceProvider;

        // Act & Assert - all query handlers must be resolvable
        await Assert.That(provider.GetService<GeocodePostcodeQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetAuthoritiesQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetAuthorityByIdQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetDesignationContextQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetUserProfileQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<ExportUserDataQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetZonePreferencesQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<ListWatchZonesQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetApplicationByUidQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetApplicationsByAuthorityQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<SearchPlanningApplicationsQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetNotificationsQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetSavedApplicationsQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetDemoAccountQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetGroupQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetUserGroupsQueryHandler>()).IsNotNull();
    }

    [Test]
    public async Task Should_ResolveAllRepositories_When_InfrastructureServicesRegistered()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var scope = factory.Services.CreateScope();
        var provider = scope.ServiceProvider;

        // Act & Assert - all repository ports must be resolvable
        await Assert.That(provider.GetService<IUserProfileRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IGroupRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IGroupInvitationRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IDecisionAlertRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IPlanningApplicationRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IWatchZoneRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IDeviceRegistrationRepository>()).IsNotNull();
        await Assert.That(provider.GetService<INotificationRepository>()).IsNotNull();
        await Assert.That(provider.GetService<ISavedApplicationRepository>()).IsNotNull();
    }
}
