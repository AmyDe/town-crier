using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Domain.SavedApplications;

public sealed class SavedApplication
{
    private SavedApplication(string userId, string applicationUid, int authorityId, DateTimeOffset savedAt, PlanningApplication? application)
    {
        this.UserId = userId;
        this.ApplicationUid = applicationUid;
        this.AuthorityId = authorityId;
        this.SavedAt = savedAt;
        this.Application = application;
    }

    public string UserId { get; }

    public string ApplicationUid { get; }

    /// <summary>
    /// Gets the PlanIt areaId for the council that issued <see cref="ApplicationUid"/>.
    /// PlanIt uids are only unique within a council, so the saved-application identity
    /// is (UserId, ApplicationUid, AuthorityId). Without this, decision-update dispatch
    /// fires the wrong council's payload to users who saved a same-uid app in a
    /// different council (bd tc-th98).
    /// </summary>
    public int AuthorityId { get; }

    public DateTimeOffset SavedAt { get; }

    /// <summary>
    /// Gets the embedded snapshot of the planning application at save time. Renders the
    /// saved-list without an N-fan-out cross-partition hydration (see bd tc-udby). Null only
    /// for legacy rows persisted before the snapshot column existed; the list handler
    /// hydrates and persists those once on first read (lazy backfill).
    /// </summary>
    public PlanningApplication? Application { get; }

    public static SavedApplication Create(string userId, string applicationUid, int authorityId, DateTimeOffset now)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        ArgumentException.ThrowIfNullOrWhiteSpace(applicationUid);
        return new SavedApplication(userId, applicationUid, authorityId, now, application: null);
    }

    public static SavedApplication Create(string userId, PlanningApplication application, DateTimeOffset now)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        ArgumentNullException.ThrowIfNull(application);

        // Key on the canonical {areaId}/{name} uid rather than the raw PlanIt uid
        // string. The raw uid is client-supplied on the save path and may arrive in a
        // stale format; the canonical uid is a deterministic server-side function of
        // (AreaId, Name), so every re-save of the same application lands on the same
        // {userId}:{applicationUid} Cosmos doc id and the upsert is idempotent (bd tc-o88i).
        return new SavedApplication(userId, application.CanonicalUid, application.AreaId, now, application);
    }

    /// <summary>
    /// Returns a new <see cref="SavedApplication"/> with the embedded snapshot replaced.
    /// Used by the refresh-on-tap path (see bd tc-udby) so that opening an item silently
    /// updates its saved-list snapshot from the master applications container.
    /// </summary>
    /// <param name="freshApplication">The latest application snapshot. Its canonical uid must match this saved record's uid.</param>
    /// <returns>A new instance with the same identity but the fresh snapshot.</returns>
    public SavedApplication WithFreshSnapshot(PlanningApplication freshApplication)
    {
        ArgumentNullException.ThrowIfNull(freshApplication);

        // Compare on the canonical {areaId}/{name} uid, the same key Create stamps onto
        // ApplicationUid. The raw Uid field is not the saved-record identity (bd tc-o88i).
        if (!string.Equals(freshApplication.CanonicalUid, this.ApplicationUid, StringComparison.Ordinal))
        {
            throw new ArgumentException(
                $"Snapshot canonical uid '{freshApplication.CanonicalUid}' does not match saved record uid '{this.ApplicationUid}'.",
                nameof(freshApplication));
        }

        return new SavedApplication(this.UserId, this.ApplicationUid, this.AuthorityId, this.SavedAt, freshApplication);
    }

    /// <summary>
    /// Returns a new <see cref="SavedApplication"/> with the supplied snapshot embedded
    /// while preserving this record's existing <see cref="ApplicationUid"/> key.
    /// </summary>
    /// <remarks>
    /// Used by the lazy-backfill path for legacy uid-only rows persisted before either the
    /// embedded snapshot column or the canonical-uid scheme existed. Unlike
    /// <see cref="WithFreshSnapshot"/> it does NOT require the snapshot's canonical uid to
    /// match the record key — a legacy row's key is a raw PlanIt uid by definition.
    /// Re-keying such rows to the canonical form is the dedicated migration's job
    /// (bd tc-sqr3); backfill must rewrite in place so it never orphans the Cosmos doc
    /// or leaves a duplicate (bd tc-o88i).
    /// </remarks>
    /// <param name="snapshot">The application snapshot to embed.</param>
    /// <returns>A new instance with the same identity but the embedded snapshot.</returns>
    public SavedApplication WithEmbeddedSnapshot(PlanningApplication snapshot)
    {
        ArgumentNullException.ThrowIfNull(snapshot);
        return new SavedApplication(this.UserId, this.ApplicationUid, this.AuthorityId, this.SavedAt, snapshot);
    }
}
