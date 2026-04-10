namespace TownCrier.Infrastructure.Tests.Notifications;

public sealed class NotificationEmailSentTests
{
    [Test]
    public async Task Should_HaveEmailSentFalse_When_NewlyCreated()
    {
        // Arrange & Act
        var notification = new NotificationBuilder().Build();

        // Assert
        await Assert.That(notification.EmailSent).IsFalse();
    }

    [Test]
    public async Task Should_HaveEmailSentTrue_When_MarkEmailSentCalled()
    {
        // Arrange
        var notification = new NotificationBuilder().Build();

        // Act
        notification.MarkEmailSent();

        // Assert
        await Assert.That(notification.EmailSent).IsTrue();
    }
}
