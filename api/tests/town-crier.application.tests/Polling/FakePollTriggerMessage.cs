using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePollTriggerMessage : IPollTriggerMessage
{
    public FakePollTriggerMessage(string id)
    {
        this.Id = id;
    }

    public string Id { get; }
}
