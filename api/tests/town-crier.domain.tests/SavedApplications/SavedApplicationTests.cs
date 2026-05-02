using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.SavedApplications;
using TownCrier.Domain.Tests.PlanningApplications;

namespace TownCrier.Domain.Tests.SavedApplications;

public sealed class SavedApplicationTests
{
    [Test]
    public async Task Should_CarryEmbeddedSnapshot_When_CreatedWithApplication()
    {
        // Arrange — saved-list rendering reads the snapshot directly to eliminate
        // the cross-partition fan-out hydration that caused 429 storms (bd tc-udby).
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);
        var application = new PlanningApplicationBuilder()
            .WithUid("planit-uid-abc")
            .WithName("Camden/CAM/24/0042/FUL")
            .Build();

        // Act
        var saved = SavedApplication.Create("auth0|user-1", application, savedAt);

        // Assert
        await Assert.That(saved.UserId).IsEqualTo("auth0|user-1");
        await Assert.That(saved.ApplicationUid).IsEqualTo("planit-uid-abc");
        await Assert.That(saved.SavedAt).IsEqualTo(savedAt);
        await Assert.That(saved.Application).IsNotNull();
        await Assert.That(saved.Application!.Uid).IsEqualTo("planit-uid-abc");
        await Assert.That(saved.Application.Name).IsEqualTo("Camden/CAM/24/0042/FUL");
    }

    [Test]
    public async Task Should_HaveNullSnapshot_When_CreatedFromLegacyRowWithoutEmbed()
    {
        // Arrange — backfill path: rows persisted before the snapshot column existed
        // hold only the uid. The list handler must be able to detect this and fall
        // back to a one-time hydration.
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);

        // Act
        var saved = SavedApplication.Create("auth0|user-1", "planit-uid-abc", savedAt);

        // Assert
        await Assert.That(saved.UserId).IsEqualTo("auth0|user-1");
        await Assert.That(saved.ApplicationUid).IsEqualTo("planit-uid-abc");
        await Assert.That(saved.Application).IsNull();
    }

    [Test]
    public async Task Should_ReplaceSnapshot_When_RefreshedWithFreshApplication()
    {
        // Arrange — refresh-on-tap upserts the latest application snapshot into the
        // saved row so the saved-list self-heals on the items the user actually opens.
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);
        var stale = new PlanningApplicationBuilder()
            .WithUid("planit-uid-abc")
            .WithAppState("Undecided")
            .Build();
        var saved = SavedApplication.Create("auth0|user-1", stale, savedAt);

        var fresh = new PlanningApplicationBuilder()
            .WithUid("planit-uid-abc")
            .WithAppState("Permitted")
            .Build();

        // Act
        var refreshed = saved.WithFreshSnapshot(fresh);

        // Assert
        await Assert.That(refreshed.UserId).IsEqualTo(saved.UserId);
        await Assert.That(refreshed.ApplicationUid).IsEqualTo(saved.ApplicationUid);
        await Assert.That(refreshed.SavedAt).IsEqualTo(saved.SavedAt);
        await Assert.That(refreshed.Application).IsNotNull();
        await Assert.That(refreshed.Application!.AppState).IsEqualTo("Permitted");
    }

    [Test]
    public async Task Should_RejectMismatchedSnapshot_When_RefreshedWithDifferentUid()
    {
        // Arrange — defensive: refusing to swap in a snapshot for the wrong uid
        // protects against handler bugs where the wrong application is paired with a save.
        var saved = SavedApplication.Create("auth0|user-1", "planit-uid-abc", DateTimeOffset.UtcNow);
        var wrongUid = new PlanningApplicationBuilder().WithUid("planit-uid-other").Build();

        // Act + Assert
        Assert.Throws<ArgumentException>(() => saved.WithFreshSnapshot(wrongUid));
        await Task.CompletedTask;
    }
}
