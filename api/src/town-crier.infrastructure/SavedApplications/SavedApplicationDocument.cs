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
            SavedAt = savedApplication.SavedAt,
            Application = savedApplication.Application is null
                ? null
                : SavedApplicationSnapshotDocument.FromDomain(savedApplication.Application),
        };
    }

    public SavedApplication ToDomain()
    {
        return this.Application is null
            ? SavedApplication.Create(this.UserId, this.ApplicationUid, this.SavedAt)
            : SavedApplication.Create(this.UserId, this.Application.ToDomain(), this.SavedAt);
    }

    internal static string MakeId(string userId, string applicationUid) => $"{userId}:{applicationUid}";
}
