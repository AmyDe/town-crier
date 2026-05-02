using TownCrier.Application.Notifications;

namespace TownCrier.Application.Tests.Notifications;

public sealed class PushSendResultTests
{
    [Test]
    public async Task Should_ExposeInvalidTokens_When_Constructed()
    {
        var result = new PushSendResult(["token-a", "token-b"]);

        await Assert.That(result.InvalidTokens).HasCount().EqualTo(2);
        await Assert.That(result.InvalidTokens[0]).IsEqualTo("token-a");
        await Assert.That(result.InvalidTokens[1]).IsEqualTo("token-b");
    }

    [Test]
    public async Task Should_ReturnEmptyInvalidTokens_When_EmptyIsUsed()
    {
        var result = PushSendResult.Empty;

        await Assert.That(result.InvalidTokens).IsEmpty();
    }
}
