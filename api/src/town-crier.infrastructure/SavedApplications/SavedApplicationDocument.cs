using TownCrier.Domain.SavedApplications;

namespace TownCrier.Infrastructure.SavedApplications;

internal sealed class SavedApplicationDocument
{
    public required string Id { get; init; }

    public required string UserId { get; init; }

    public required string ApplicationUid { get; init; }

    public required DateTimeOffset SavedAt { get; init; }

    public static SavedApplicationDocument FromDomain(SavedApplication savedApplication)
    {
        ArgumentNullException.ThrowIfNull(savedApplication);

        return new SavedApplicationDocument
        {
            Id = MakeId(savedApplication.UserId, savedApplication.ApplicationUid),
            UserId = savedApplication.UserId,
            ApplicationUid = savedApplication.ApplicationUid,
            SavedAt = savedApplication.SavedAt,
        };
    }

    public SavedApplication ToDomain()
    {
        return SavedApplication.Create(this.UserId, this.ApplicationUid, this.SavedAt);
    }

    internal static string MakeId(string userId, string applicationUid) => $"{userId}:{applicationUid}";
}
