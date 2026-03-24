using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.DecisionAlerts;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Groups;
using TownCrier.Application.Notifications;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Infrastructure.DecisionAlerts;
using TownCrier.Infrastructure.DeviceRegistrations;
using TownCrier.Infrastructure.Groups;
using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.PlanningApplications;
using TownCrier.Infrastructure.SavedApplications;
using TownCrier.Infrastructure.UserProfiles;
using TownCrier.Infrastructure.WatchZones;

namespace TownCrier.Web.Tests.DependencyInjection;

public sealed class RepositoryOverrideTests
{
    [Test]
    public async Task Should_ResolveAllRepositories_AsInMemoryImplementations()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var scope = factory.Services.CreateScope();
        var provider = scope.ServiceProvider;

        // Act & Assert — all 9 repository interfaces must resolve to InMemory implementations
        await Assert.That(provider.GetRequiredService<IUserProfileRepository>())
            .IsTypeOf<InMemoryUserProfileRepository>();

        await Assert.That(provider.GetRequiredService<IGroupRepository>())
            .IsTypeOf<InMemoryGroupRepository>();

        await Assert.That(provider.GetRequiredService<IGroupInvitationRepository>())
            .IsTypeOf<InMemoryGroupInvitationRepository>();

        await Assert.That(provider.GetRequiredService<IDecisionAlertRepository>())
            .IsTypeOf<InMemoryDecisionAlertRepository>();

        await Assert.That(provider.GetRequiredService<IPlanningApplicationRepository>())
            .IsTypeOf<InMemoryPlanningApplicationRepository>();

        await Assert.That(provider.GetRequiredService<IWatchZoneRepository>())
            .IsTypeOf<InMemoryWatchZoneRepository>();

        await Assert.That(provider.GetRequiredService<IDeviceRegistrationRepository>())
            .IsTypeOf<InMemoryDeviceRegistrationRepository>();

        await Assert.That(provider.GetRequiredService<INotificationRepository>())
            .IsTypeOf<InMemoryNotificationRepository>();

        await Assert.That(provider.GetRequiredService<ISavedApplicationRepository>())
            .IsTypeOf<InMemorySavedApplicationRepository>();
    }
}
