using TownCrier.Application.PlanningApplications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.Tests.SavedApplications;

public sealed class GetSavedApplicationsQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnEmptyList_When_UserHasNoSavedApplications()
    {
        // Arrange
        var savedRepository = new FakeSavedApplicationRepository();
        var applicationRepository = new FakePlanningApplicationRepository();
        var handler = new GetSavedApplicationsQueryHandler(savedRepository, applicationRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ProjectFromEmbeddedSnapshot_WithoutTouchingPlanningRepository_When_AllSavesCarrySnapshot()
    {
        // Arrange — the saved-list endpoint must render with one partitioned
        // query and zero hydration calls on the happy path. A spy fake fails the
        // test if the handler reaches into the planning repository at all.
        // See bd tc-udby for the 429 storm this design eliminates.
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FailIfCalledPlanningApplicationRepository();
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);

        var app1 = new PlanningApplicationBuilder()
            .WithUid("planit-uid-abc").WithName("APP/2026/001").WithAreaName("Wiltshire").Build();
        var app2 = new PlanningApplicationBuilder()
            .WithUid("planit-uid-def").WithName("APP/2026/002").WithAreaName("Somerset").Build();

        await savedRepository.SaveAsync(SavedApplication.Create("auth0|user-1", app1, savedAt), CancellationToken.None);
        await savedRepository.SaveAsync(SavedApplication.Create("auth0|user-1", app2, savedAt.AddHours(1)), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, planningRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert — both items projected from the embedded snapshot. ApplicationUid
        // is the canonical {areaId}/{name} key, not the raw PlanIt uid (bd tc-o88i).
        await Assert.That(result).HasCount().EqualTo(2);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("1/APP/2026/001");
        await Assert.That(result[0].Application.Name).IsEqualTo("APP/2026/001");
        await Assert.That(result[0].Application.AreaName).IsEqualTo("Wiltshire");
        await Assert.That(result[1].ApplicationUid).IsEqualTo("1/APP/2026/002");
        await Assert.That(result[1].Application.Name).IsEqualTo("APP/2026/002");
    }

    [Test]
    public async Task Should_OnlyReturnOwnApplications_When_MultipleUsersHaveSaved()
    {
        // Arrange
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FailIfCalledPlanningApplicationRepository();
        var savedAt = DateTimeOffset.UtcNow;

        var app1 = new PlanningApplicationBuilder()
            .WithUid("planit-uid-abc").WithAreaId(7).WithName("APP/own").Build();
        var app2 = new PlanningApplicationBuilder().WithUid("planit-uid-def").Build();

        await savedRepository.SaveAsync(SavedApplication.Create("auth0|user-1", app1, savedAt), CancellationToken.None);
        await savedRepository.SaveAsync(SavedApplication.Create("auth0|user-2", app2, savedAt), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, planningRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert — ApplicationUid is the canonical {areaId}/{name} key (bd tc-o88i).
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("7/APP/own");
    }

    [Test]
    public async Task Should_LazilyBackfillSnapshot_When_LegacyRowHasUidOnly()
    {
        // Arrange — rows persisted before the snapshot column existed hold only
        // the uid. The handler hydrates them once via the planning repo, re-keys
        // them to the canonical uid (bd tc-sqr3), and upserts so subsequent reads
        // are zero-hydration.
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FakePlanningApplicationRepository();
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);

        // Seed: planning record exists, saved row does NOT carry the snapshot and
        // is keyed on the raw legacy uid.
        var app = new PlanningApplicationBuilder()
            .WithUid("planit-uid-legacy").WithName("APP/legacy").WithAreaId(1).WithAreaName("Camden").Build();
        await planningRepository.UpsertAsync(app, CancellationToken.None);
        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-legacy", authorityId: 1, savedAt), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, planningRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act — first read triggers hydration plus re-key.
        var firstResult = await handler.HandleAsync(query, CancellationToken.None);

        // Assert — first read returned the hydrated snapshot under the canonical uid.
        await Assert.That(firstResult).HasCount().EqualTo(1);
        await Assert.That(firstResult[0].ApplicationUid).IsEqualTo("1/APP/legacy");
        await Assert.That(firstResult[0].Application.Name).IsEqualTo("APP/legacy");

        // Assert — the saved row was hydrated AND re-keyed: the legacy-uid doc is
        // gone and a single canonical doc carrying the embedded snapshot remains.
        var rows = await savedRepository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(rows).HasCount().EqualTo(1);
        await Assert.That(rows[0].ApplicationUid).IsEqualTo("1/APP/legacy");
        await Assert.That(rows[0].Application).IsNotNull();
        await Assert.That(rows[0].Application!.Name).IsEqualTo("APP/legacy");
        await Assert.That(rows[0].SavedAt).IsEqualTo(savedAt);

        // Act — second read with the planning repo unavailable: still works
        // because backfill self-healed and there are no more legacy rows.
        var sealedRepository = new FailIfCalledPlanningApplicationRepository();
        var sealedHandler = new GetSavedApplicationsQueryHandler(savedRepository, sealedRepository);
        var secondResult = await sealedHandler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(secondResult).HasCount().EqualTo(1);
        await Assert.That(secondResult[0].Application.Name).IsEqualTo("APP/legacy");
    }

    [Test]
    public async Task Should_ReKeyLegacyRowToCanonicalUid_When_StoredUidIsLegacyBareRefFormat()
    {
        // Arrange — a legacy-format saved row persisted before PR#398: its stored
        // ApplicationUid is the raw PlanIt bare ref (e.g. 25/02755/CLC), not the
        // canonical {areaId}/{name} key. All such rows carry an embedded snapshot,
        // so the canonical uid is derivable in place — no cross-container lookup.
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FailIfCalledPlanningApplicationRepository();
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);

        var snapshot = new PlanningApplicationBuilder()
            .WithUid("25/02755/CLC").WithName("Kingston/25/02755/CLC").WithAreaId(314).Build();

        // Legacy row: keyed on the raw uid but carrying the snapshot.
        var legacyRow = SavedApplication
            .Create("auth0|user-1", "25/02755/CLC", authorityId: 314, savedAt)
            .WithEmbeddedSnapshot(snapshot);
        await savedRepository.SaveAsync(legacyRow, CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, planningRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act — the read self-heals the legacy row.
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert — exactly one row, reported under the canonical uid.
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("314/Kingston/25/02755/CLC");

        // Assert — the legacy doc was deleted and a canonical doc written in its
        // place. Cosmos doc ids are immutable, so a re-key is delete + write.
        var rows = await savedRepository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(rows).HasCount().EqualTo(1);
        await Assert.That(rows[0].ApplicationUid).IsEqualTo("314/Kingston/25/02755/CLC");
        await Assert.That(rows[0].Application).IsNotNull();
        await Assert.That(rows[0].SavedAt).IsEqualTo(savedAt);
    }

    [Test]
    public async Task Should_NotReKey_When_StoredUidIsAlreadyCanonical()
    {
        // Arrange — an already-canonical row. Re-running the migration over it
        // must be a no-op: no delete, no spurious rewrite.
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FailIfCalledPlanningApplicationRepository();
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);

        var app = new PlanningApplicationBuilder()
            .WithUid("25/02755/CLC").WithName("Kingston/25/02755/CLC").WithAreaId(314).Build();
        await savedRepository.SaveAsync(SavedApplication.Create("auth0|user-1", app, savedAt), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, planningRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");
        var savesBefore = savedRepository.SaveCallCount;

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert — projected unchanged, and the canonical row was not rewritten.
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("314/Kingston/25/02755/CLC");
        await Assert.That(savedRepository.SaveCallCount).IsEqualTo(savesBefore);
        var rows = await savedRepository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(rows).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_DropLegacyRow_When_CanonicalDuplicateAlreadyExistsForSameUser()
    {
        // Arrange — the confirmed prod case: a user holds BOTH a legacy doc and a
        // canonical doc for the same application. The re-key must keep the
        // canonical doc and delete the legacy one, leaving a single row.
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FailIfCalledPlanningApplicationRepository();
        var legacySavedAt = new DateTimeOffset(2026, 4, 1, 9, 0, 0, TimeSpan.Zero);
        var canonicalSavedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);

        var snapshot = new PlanningApplicationBuilder()
            .WithUid("25/02755/CLC").WithName("Kingston/25/02755/CLC").WithAreaId(314).Build();

        var legacyRow = SavedApplication
            .Create("auth0|user-1", "25/02755/CLC", authorityId: 314, legacySavedAt)
            .WithEmbeddedSnapshot(snapshot);
        await savedRepository.SaveAsync(legacyRow, CancellationToken.None);
        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", snapshot, canonicalSavedAt), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, planningRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert — one row only; the canonical doc survives, the legacy one is gone.
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("314/Kingston/25/02755/CLC");

        var rows = await savedRepository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(rows).HasCount().EqualTo(1);
        await Assert.That(rows[0].ApplicationUid).IsEqualTo("314/Kingston/25/02755/CLC");
        await Assert.That(rows[0].SavedAt).IsEqualTo(canonicalSavedAt);
    }

    [Test]
    public async Task Should_ExcludeSavedApplication_When_LegacyRowAndPlanningApplicationNoLongerExists()
    {
        // Arrange — legacy uid-only row whose master planning application is
        // no longer in Cosmos. Excluded from the result rather than failing the
        // entire response.
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FakePlanningApplicationRepository();
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);

        var present = new PlanningApplicationBuilder()
            .WithUid("planit-uid-abc").WithName("APP/abc").Build();
        await planningRepository.UpsertAsync(present, CancellationToken.None);

        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", present, savedAt), CancellationToken.None);
        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-orphaned", authorityId: 1, savedAt.AddHours(1)), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, planningRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert — the present row survives; ApplicationUid is the canonical key.
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("1/APP/abc");
    }
}
