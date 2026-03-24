namespace TownCrier.Domain.Groups;

public sealed class GroupInvitation
{
    private GroupInvitation(
        string id,
        string groupId,
        string inviteeEmail,
        string invitedByUserId,
        InvitationStatus status,
        DateTimeOffset createdAt,
        DateTimeOffset expiresAt)
    {
        this.Id = id;
        this.GroupId = groupId;
        this.InviteeEmail = inviteeEmail;
        this.InvitedByUserId = invitedByUserId;
        this.Status = status;
        this.CreatedAt = createdAt;
        this.ExpiresAt = expiresAt;
    }

    public string Id { get; }

    public string GroupId { get; }

    public string InviteeEmail { get; }

    public string InvitedByUserId { get; }

    public InvitationStatus Status { get; private set; }

    public DateTimeOffset CreatedAt { get; }

    public DateTimeOffset ExpiresAt { get; }

    public static GroupInvitation Create(
        string id,
        string groupId,
        string inviteeEmail,
        string invitedByUserId,
        DateTimeOffset now,
        TimeSpan validityPeriod)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(id);
        ArgumentException.ThrowIfNullOrWhiteSpace(groupId);
        ArgumentException.ThrowIfNullOrWhiteSpace(inviteeEmail);
        ArgumentException.ThrowIfNullOrWhiteSpace(invitedByUserId);

        return new GroupInvitation(
            id,
            groupId,
            inviteeEmail,
            invitedByUserId,
            InvitationStatus.Pending,
            now,
            now + validityPeriod);
    }

    public void Accept(DateTimeOffset now)
    {
        if (this.Status != InvitationStatus.Pending)
        {
            throw new InvalidOperationException("Only pending invitations can be accepted.");
        }

        if (now > this.ExpiresAt)
        {
            throw new InvalidOperationException("This invitation has expired.");
        }

        this.Status = InvitationStatus.Accepted;
    }

    public void Decline()
    {
        if (this.Status != InvitationStatus.Pending)
        {
            throw new InvalidOperationException("Only pending invitations can be declined.");
        }

        this.Status = InvitationStatus.Declined;
    }

    internal static GroupInvitation Reconstitute(
        string id,
        string groupId,
        string inviteeEmail,
        string invitedByUserId,
        InvitationStatus status,
        DateTimeOffset createdAt,
        DateTimeOffset expiresAt)
    {
        return new GroupInvitation(id, groupId, inviteeEmail, invitedByUserId, status, createdAt, expiresAt);
    }
}
