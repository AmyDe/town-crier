namespace TownCrier.Domain.DecisionAlerts;

public sealed class DecisionAlert
{
    private DecisionAlert(
        string id,
        string userId,
        string applicationUid,
        string applicationName,
        string applicationAddress,
        string decision,
        bool pushSent,
        DateTimeOffset createdAt)
    {
        this.Id = id;
        this.UserId = userId;
        this.ApplicationUid = applicationUid;
        this.ApplicationName = applicationName;
        this.ApplicationAddress = applicationAddress;
        this.Decision = decision;
        this.PushSent = pushSent;
        this.CreatedAt = createdAt;
    }

    public string Id { get; }

    public string UserId { get; }

    public string ApplicationUid { get; }

    public string ApplicationName { get; }

    public string ApplicationAddress { get; }

    public string Decision { get; }

    public bool PushSent { get; private set; }

    public DateTimeOffset CreatedAt { get; }

    public static DecisionAlert Create(
        string userId,
        string applicationUid,
        string applicationName,
        string applicationAddress,
        string decision,
        DateTimeOffset now)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        ArgumentException.ThrowIfNullOrWhiteSpace(applicationUid);
        ArgumentException.ThrowIfNullOrWhiteSpace(applicationName);

        return new DecisionAlert(
            id: Guid.NewGuid().ToString(),
            userId: userId,
            applicationUid: applicationUid,
            applicationName: applicationName,
            applicationAddress: applicationAddress,
            decision: decision,
            pushSent: false,
            createdAt: now);
    }

    public void MarkPushSent()
    {
        this.PushSent = true;
    }
}
