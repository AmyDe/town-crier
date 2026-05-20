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
            .WithAreaId(42)
            .Build();

        // Act
        var saved = SavedApplication.Create("auth0|user-1", application, savedAt);

        // Assert — the saved record keys on the canonical {areaId}/{name} uid, NOT the
        // raw PlanIt uid string. This is what keeps the Cosmos doc id stable and makes
        // re-saves idempotent regardless of what uid format the client sent (bd tc-o88i).
        await Assert.That(saved.UserId).IsEqualTo("auth0|user-1");
        await Assert.That(saved.ApplicationUid).IsEqualTo("42/Camden/CAM/24/0042/FUL");
        await Assert.That(saved.AuthorityId).IsEqualTo(42);
        await Assert.That(saved.SavedAt).IsEqualTo(savedAt);
        await Assert.That(saved.Application).IsNotNull();
        await Assert.That(saved.Application!.Uid).IsEqualTo("planit-uid-abc");
        await Assert.That(saved.Application.Name).IsEqualTo("Camden/CAM/24/0042/FUL");
    }

    [Test]
    public async Task Should_KeyOnCanonicalUid_When_RawUidFormatDiffers()
    {
        // Arrange — two saves of the same application where the client supplied a
        // different raw uid string each time (the PR #398 stale-format scenario).
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);
        var legacyFormat = new PlanningApplicationBuilder()
            .WithAreaId(42).WithName("CAM/24/0042/FUL").WithUid("CAM/24/0042/FUL").Build();
        var newFormat = new PlanningApplicationBuilder()
            .WithAreaId(42).WithName("CAM/24/0042/FUL").WithUid("42/CAM/24/0042/FUL").Build();

        // Act
        var fromLegacy = SavedApplication.Create("auth0|user-1", legacyFormat, savedAt);
        var fromNew = SavedApplication.Create("auth0|user-1", newFormat, savedAt);

        // Assert — both land on the identical canonical key, so the Cosmos
        // {userId}:{applicationUid} doc id is identical and the upsert is idempotent.
        await Assert.That(fromLegacy.ApplicationUid).IsEqualTo("42/CAM/24/0042/FUL");
        await Assert.That(fromNew.ApplicationUid).IsEqualTo(fromLegacy.ApplicationUid);
    }

    [Test]
    public async Task Should_HaveNullSnapshot_When_CreatedFromLegacyRowWithoutEmbed()
    {
        // Arrange — backfill path: rows persisted before the snapshot column existed
        // hold only the uid + authorityId. The list handler must detect a null
        // snapshot and fall back to a one-time hydration. AuthorityId is part of the
        // identity because PlanIt uids are only unique within a council (tc-th98).
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);

        // Act
        var saved = SavedApplication.Create("auth0|user-1", "planit-uid-abc", authorityId: 314, savedAt);

        // Assert
        await Assert.That(saved.UserId).IsEqualTo("auth0|user-1");
        await Assert.That(saved.ApplicationUid).IsEqualTo("planit-uid-abc");
        await Assert.That(saved.AuthorityId).IsEqualTo(314);
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
        var saved = SavedApplication.Create("auth0|user-1", "planit-uid-abc", authorityId: 1, DateTimeOffset.UtcNow);
        var wrongUid = new PlanningApplicationBuilder().WithUid("planit-uid-other").Build();

        // Act + Assert
        Assert.Throws<ArgumentException>(() => saved.WithFreshSnapshot(wrongUid));
        await Task.CompletedTask;
    }
}
