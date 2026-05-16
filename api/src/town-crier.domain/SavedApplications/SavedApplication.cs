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
        return new SavedApplication(userId, application.Uid, application.AreaId, now, application);
    }

    /// <summary>
    /// Returns a new <see cref="SavedApplication"/> with the embedded snapshot replaced.
    /// Used by the refresh-on-tap path (see bd tc-udby) so that opening an item silently
    /// updates its saved-list snapshot from the master applications container.
    /// </summary>
    /// <param name="freshApplication">The latest application snapshot. Must share this saved record's uid.</param>
    /// <returns>A new instance with the same identity but the fresh snapshot.</returns>
    public SavedApplication WithFreshSnapshot(PlanningApplication freshApplication)
    {
        ArgumentNullException.ThrowIfNull(freshApplication);

        if (!string.Equals(freshApplication.Uid, this.ApplicationUid, StringComparison.Ordinal))
        {
            throw new ArgumentException(
                $"Snapshot uid '{freshApplication.Uid}' does not match saved record uid '{this.ApplicationUid}'.",
                nameof(freshApplication));
        }

        return new SavedApplication(this.UserId, this.ApplicationUid, this.AuthorityId, this.SavedAt, freshApplication);
    }
}
