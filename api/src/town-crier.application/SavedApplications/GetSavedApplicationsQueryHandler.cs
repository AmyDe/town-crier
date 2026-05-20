using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.SavedApplications;

public sealed class GetSavedApplicationsQueryHandler
{
    private readonly ISavedApplicationRepository savedRepository;
    private readonly IPlanningApplicationRepository applicationRepository;

    public GetSavedApplicationsQueryHandler(
        ISavedApplicationRepository savedRepository,
        IPlanningApplicationRepository applicationRepository)
    {
        this.savedRepository = savedRepository;
        this.applicationRepository = applicationRepository;
    }

    public async Task<IReadOnlyList<SavedApplicationResult>> HandleAsync(GetSavedApplicationsQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        // One partitioned query — Cosmos returns the saved rows together with their
        // embedded planning-application snapshot. No N-fan-out cross-partition
        // hydration. See bd tc-udby for the 429 storm this design eliminates.
        var saved = await this.savedRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);

        // Track the canonical uids already emitted this read so a legacy+canonical
        // duplicate pair for the same app collapses to a single row (bd tc-sqr3).
        var emittedCanonicalUids = new HashSet<string>(StringComparer.Ordinal);
        var results = new List<SavedApplicationResult>(saved.Count);

        foreach (var record in saved)
        {
            var hydrated = await this.HydrateAsync(record, ct).ConfigureAwait(false);
            if (hydrated is null)
            {
                // Master record gone — exclude rather than failing the whole list.
                continue;
            }

            // Lazy migration (bd tc-sqr3): pre-PR#398 rows are keyed on the raw
            // PlanIt bare ref rather than the canonical {areaId}/{name} uid. Re-key
            // them to the canonical form so the saved icon colours in and re-saves
            // stop minting duplicates.
            var canonical = IsLegacyKeyed(hydrated)
                ? await this.ReKeyToCanonicalAsync(hydrated, ct).ConfigureAwait(false)
                : hydrated;

            // Dedup the confirmed legacy+canonical pair: the legacy and canonical
            // rows both resolve to the same canonical doc id, so emit it at most once.
            if (!emittedCanonicalUids.Add(canonical.ApplicationUid))
            {
                continue;
            }

            results.Add(new SavedApplicationResult(
                canonical.ApplicationUid,
                canonical.SavedAt,
                GetApplicationByUidQueryHandler.ToResult(canonical.Application!)));
        }

        return results;
    }

    /// <summary>
    /// True when the row is keyed on a legacy bare-ref uid rather than the canonical
    /// {areaId}/{name} uid. Only decidable once the snapshot is embedded — the
    /// canonical uid is derived from the snapshot.
    /// </summary>
    private static bool IsLegacyKeyed(SavedApplication record) =>
        record.Application is not null
        && !string.Equals(record.ApplicationUid, record.Application.CanonicalUid, StringComparison.Ordinal);

    /// <summary>
    /// Ensures the saved record carries an embedded snapshot. Rows persisted before
    /// the snapshot column existed hold only the uid; they are hydrated once via the
    /// partition-scoped planning lookup and rewritten in place so subsequent reads are
    /// zero-hydration. Returns null when the master planning application is gone.
    /// </summary>
    private async Task<SavedApplication?> HydrateAsync(SavedApplication record, CancellationToken ct)
    {
        if (record.Application is not null)
        {
            return record;
        }

        // Lazy backfill: rows persisted before the snapshot column existed hold only
        // the uid. Hydrate once and upsert so subsequent reads are zero-hydration.
        // Uses the partition-scoped uid overload (authorityCode as pk) to avoid a
        // cross-partition scan. GH#395 Invariant 2.
        var authorityCode = record.AuthorityId.ToString(System.Globalization.CultureInfo.InvariantCulture);
        var fetched = await this.applicationRepository.GetByUidAsync(record.ApplicationUid, authorityCode, ct).ConfigureAwait(false);
        if (fetched is null)
        {
            return null;
        }

        // Rewrite in place: embed the snapshot but keep the row's existing
        // ApplicationUid. Re-keying to the canonical uid happens separately, after
        // hydration, so the two migration steps stay independently testable.
        var refreshed = record.WithEmbeddedSnapshot(fetched);
        await this.savedRepository.SaveAsync(refreshed, ct).ConfigureAwait(false);
        return refreshed;
    }

    /// <summary>
    /// Re-keys a legacy-format saved row to the canonical {areaId}/{name} uid.
    /// Cosmos doc ids are immutable, so a re-key is a write of the canonical doc plus
    /// a delete of the legacy doc. When a canonical doc already exists for the same
    /// user+app (the confirmed legacy+canonical duplicate case) the canonical doc is
    /// kept untouched and only the legacy doc is deleted (bd tc-sqr3).
    /// </summary>
    private async Task<SavedApplication> ReKeyToCanonicalAsync(SavedApplication legacy, CancellationToken ct)
    {
        var canonical = SavedApplication.Create(legacy.UserId, legacy.Application!, legacy.SavedAt);

        var canonicalAlreadyExists = await this.savedRepository
            .ExistsAsync(canonical.UserId, canonical.ApplicationUid, ct).ConfigureAwait(false);
        if (!canonicalAlreadyExists)
        {
            // Write the canonical doc before deleting the legacy one so an
            // interrupted run leaves a recoverable duplicate, never a lost save.
            await this.savedRepository.SaveAsync(canonical, ct).ConfigureAwait(false);
        }

        // The canonical doc is the survivor — drop the legacy duplicate.
        await this.savedRepository.DeleteAsync(legacy.UserId, legacy.ApplicationUid, ct).ConfigureAwait(false);

        return canonical;
    }
}
