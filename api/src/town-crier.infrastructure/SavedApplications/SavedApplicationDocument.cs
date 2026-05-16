using System.Diagnostics.CodeAnalysis;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.SavedApplications;
using TownCrier.Infrastructure.PlanningApplications;

namespace TownCrier.Infrastructure.SavedApplications;

internal sealed class SavedApplicationDocument
{
    public required string Id { get; init; }

    public required string UserId { get; init; }

    public required string ApplicationUid { get; init; }

    /// <summary>
    /// Gets the PlanIt areaId for the council that issued <see cref="ApplicationUid"/>.
    /// Nullable so legacy rows persisted before this column was added hydrate via
    /// the embedded snapshot's <c>application.areaId</c> fallback in <see cref="ToDomain"/>.
    /// New rows always populate this projected column so that the dispatch query can
    /// filter on (uid, authorityId) without cracking the embedded snapshot (bd tc-th98).
    /// </summary>
    public int? AuthorityId { get; init; }

    public required DateTimeOffset SavedAt { get; init; }

    /// <summary>
    /// Gets the embedded planning-application snapshot. Renders the saved-list with one
    /// partitioned query, eliminating the cross-partition fan-out that blew the
    /// per-second RU budget on Cosmos serverless (bd tc-udby). Null only for
    /// legacy rows persisted before the snapshot column existed; the list handler
    /// hydrates and persists those once on first read (lazy backfill).
    /// </summary>
    public SavedApplicationSnapshotDocument? Application { get; init; }

    public static SavedApplicationDocument FromDomain(SavedApplication savedApplication)
    {
        ArgumentNullException.ThrowIfNull(savedApplication);

        return new SavedApplicationDocument
        {
            Id = MakeId(savedApplication.UserId, savedApplication.ApplicationUid),
            UserId = savedApplication.UserId,
            ApplicationUid = savedApplication.ApplicationUid,
            AuthorityId = savedApplication.AuthorityId,
            SavedAt = savedApplication.SavedAt,
            Application = savedApplication.Application is null
                ? null
                : SavedApplicationSnapshotDocument.FromDomain(savedApplication.Application),
        };
    }

    public SavedApplication ToDomain()
    {
        // Coalesce the projected authorityId from the embedded snapshot for legacy
        // rows persisted before the column was added. Backfill semantics — same
        // pattern as the snapshot null check below (bd tc-th98).
        var authorityId = this.AuthorityId ?? this.Application?.AreaId ?? 0;

        return this.Application is null
            ? SavedApplication.Create(this.UserId, this.ApplicationUid, authorityId, this.SavedAt)
            : SavedApplication.Create(this.UserId, this.Application.ToDomain(), this.SavedAt);
    }

    internal static string MakeId(string userId, string applicationUid) => $"{userId}:{applicationUid}";
}
