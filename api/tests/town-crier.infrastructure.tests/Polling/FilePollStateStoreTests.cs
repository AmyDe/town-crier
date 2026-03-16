using TownCrier.Infrastructure.Polling;

namespace TownCrier.Infrastructure.Tests.Polling;

public sealed class FilePollStateStoreTests
{
    [Test]
    public async Task Should_ReturnNull_When_FileDoesNotExist()
    {
        // Arrange
        var filePath = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString(), "poll-state.txt");
        var store = new FilePollStateStore(filePath);

        // Act
        var result = await store.GetLastPollTimeAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_PersistAndRetrieve_When_PollTimeIsSaved()
    {
        // Arrange
        var filePath = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString(), "poll-state.txt");
        var store = new FilePollStateStore(filePath);
        var pollTime = new DateTimeOffset(2026, 3, 16, 12, 30, 0, TimeSpan.Zero);

        try
        {
            // Act
            await store.SaveLastPollTimeAsync(pollTime, CancellationToken.None);
            var result = await store.GetLastPollTimeAsync(CancellationToken.None);

            // Assert
            await Assert.That(result).IsEqualTo(pollTime);
        }
        finally
        {
            var directory = Path.GetDirectoryName(filePath);
            if (directory is not null && Directory.Exists(directory))
            {
                Directory.Delete(directory, true);
            }
        }
    }

    [Test]
    public async Task Should_OverwritePreviousValue_When_SavedAgain()
    {
        // Arrange
        var filePath = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString(), "poll-state.txt");
        var store = new FilePollStateStore(filePath);
        var firstPoll = new DateTimeOffset(2026, 3, 16, 10, 0, 0, TimeSpan.Zero);
        var secondPoll = new DateTimeOffset(2026, 3, 16, 10, 15, 0, TimeSpan.Zero);

        try
        {
            // Act
            await store.SaveLastPollTimeAsync(firstPoll, CancellationToken.None);
            await store.SaveLastPollTimeAsync(secondPoll, CancellationToken.None);
            var result = await store.GetLastPollTimeAsync(CancellationToken.None);

            // Assert
            await Assert.That(result).IsEqualTo(secondPoll);
        }
        finally
        {
            var directory = Path.GetDirectoryName(filePath);
            if (directory is not null && Directory.Exists(directory))
            {
                Directory.Delete(directory, true);
            }
        }
    }
}
