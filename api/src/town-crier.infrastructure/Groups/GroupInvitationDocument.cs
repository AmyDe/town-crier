using TownCrier.Domain.Groups;

namespace TownCrier.Infrastructure.Groups;

internal sealed class GroupInvitationDocument
{
    private const string IdPrefix = "inv:";

    public required string Id { get; init; }

    public required string Type { get; init; }

    public required string OwnerId { get; init; }

    public required string GroupId { get; init; }

    public required string InviteeEmail { get; init; }

    public required string InvitedByUserId { get; init; }

    public required string Status { get; init; }

    public required DateTimeOffset CreatedAt { get; init; }

    public required DateTimeOffset ExpiresAt { get; init; }

    public static string ToDocumentId(string domainId) => IdPrefix + domainId;

    public static GroupInvitationDocument FromDomain(GroupInvitation invitation)
    {
        ArgumentNullException.ThrowIfNull(invitation);

        return new GroupInvitationDocument
        {
            Id = ToDocumentId(invitation.Id),
            Type = "invitation",
            OwnerId = invitation.InvitedByUserId,
            GroupId = invitation.GroupId,
            InviteeEmail = invitation.InviteeEmail,
            InvitedByUserId = invitation.InvitedByUserId,
            Status = invitation.Status.ToString(),
            CreatedAt = invitation.CreatedAt,
            ExpiresAt = invitation.ExpiresAt,
        };
    }

    public GroupInvitation ToDomain()
    {
        var domainId = this.Id.StartsWith(IdPrefix, StringComparison.Ordinal)
            ? this.Id[IdPrefix.Length..]
            : this.Id;

        var status = Enum.Parse<InvitationStatus>(this.Status);
        return GroupInvitation.Reconstitute(
            domainId,
            this.GroupId,
            this.InviteeEmail,
            this.InvitedByUserId,
            status,
            this.CreatedAt,
            this.ExpiresAt);
    }
}
