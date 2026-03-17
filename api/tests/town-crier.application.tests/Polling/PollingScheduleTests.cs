using TownCrier.Domain.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollingScheduleTests
{
    [Test]
    public async Task Should_AssignHighPriority_When_AuthorityHasManyZones()
    {
        // Arrange
        var zoneCounts = new Dictionary<int, int>
        {
            [100] = 10, // 10 zones
        };

        var config = new PollingScheduleConfig(HighThreshold: 5, LowThreshold: 2);

        // Act
        var schedule = PollingSchedule.Calculate(zoneCounts, config);

        // Assert
        var entry = schedule.GetPriority(100);
        await Assert.That(entry).IsEqualTo(PollingPriority.High);
    }

    [Test]
    public async Task Should_AssignNormalPriority_When_AuthorityHasModerateZones()
    {
        // Arrange
        var zoneCounts = new Dictionary<int, int>
        {
            [100] = 3,
        };

        var config = new PollingScheduleConfig(HighThreshold: 5, LowThreshold: 2);

        // Act
        var schedule = PollingSchedule.Calculate(zoneCounts, config);

        // Assert
        var entry = schedule.GetPriority(100);
        await Assert.That(entry).IsEqualTo(PollingPriority.Normal);
    }

    [Test]
    public async Task Should_AssignLowPriority_When_AuthorityHasFewZones()
    {
        // Arrange
        var zoneCounts = new Dictionary<int, int>
        {
            [100] = 1,
        };

        var config = new PollingScheduleConfig(HighThreshold: 5, LowThreshold: 2);

        // Act
        var schedule = PollingSchedule.Calculate(zoneCounts, config);

        // Assert
        var entry = schedule.GetPriority(100);
        await Assert.That(entry).IsEqualTo(PollingPriority.Low);
    }

    [Test]
    public async Task Should_AssignDifferentPriorities_When_AuthoritiesHaveVaryingZoneCounts()
    {
        // Arrange
        var zoneCounts = new Dictionary<int, int>
        {
            [100] = 10, // High
            [200] = 3,  // Normal
            [300] = 1,  // Low
        };

        var config = new PollingScheduleConfig(HighThreshold: 5, LowThreshold: 2);

        // Act
        var schedule = PollingSchedule.Calculate(zoneCounts, config);

        // Assert
        await Assert.That(schedule.GetPriority(100)).IsEqualTo(PollingPriority.High);
        await Assert.That(schedule.GetPriority(200)).IsEqualTo(PollingPriority.Normal);
        await Assert.That(schedule.GetPriority(300)).IsEqualTo(PollingPriority.Low);
    }

    [Test]
    public async Task Should_ReturnLowPriority_When_AuthorityNotInSchedule()
    {
        // Arrange
        var zoneCounts = new Dictionary<int, int>();
        var config = new PollingScheduleConfig(HighThreshold: 5, LowThreshold: 2);

        // Act
        var schedule = PollingSchedule.Calculate(zoneCounts, config);

        // Assert
        var entry = schedule.GetPriority(999);
        await Assert.That(entry).IsEqualTo(PollingPriority.Low);
    }

    [Test]
    public async Task Should_AssignHighPriority_When_ZoneCountEqualsHighThreshold()
    {
        // Arrange
        var zoneCounts = new Dictionary<int, int>
        {
            [100] = 5,
        };

        var config = new PollingScheduleConfig(HighThreshold: 5, LowThreshold: 2);

        // Act
        var schedule = PollingSchedule.Calculate(zoneCounts, config);

        // Assert
        await Assert.That(schedule.GetPriority(100)).IsEqualTo(PollingPriority.High);
    }

    [Test]
    public async Task Should_AssignNormalPriority_When_ZoneCountEqualsLowThreshold()
    {
        // Arrange
        var zoneCounts = new Dictionary<int, int>
        {
            [100] = 2,
        };

        var config = new PollingScheduleConfig(HighThreshold: 5, LowThreshold: 2);

        // Act
        var schedule = PollingSchedule.Calculate(zoneCounts, config);

        // Assert
        await Assert.That(schedule.GetPriority(100)).IsEqualTo(PollingPriority.Normal);
    }

    [Test]
    public async Task Should_ReturnAllAuthorityIds_When_EnumeratingSchedule()
    {
        // Arrange
        var zoneCounts = new Dictionary<int, int>
        {
            [100] = 10,
            [200] = 3,
            [300] = 1,
        };

        var config = new PollingScheduleConfig(HighThreshold: 5, LowThreshold: 2);

        // Act
        var schedule = PollingSchedule.Calculate(zoneCounts, config);

        // Assert
        var ids = schedule.AuthorityIds;
        await Assert.That(ids).HasCount().EqualTo(3);
        await Assert.That(ids).Contains(100);
        await Assert.That(ids).Contains(200);
        await Assert.That(ids).Contains(300);
    }

    [Test]
    public async Task Should_DetermineIfShouldPoll_When_HighPriorityAndAnyCycle()
    {
        // Arrange
        var zoneCounts = new Dictionary<int, int> { [100] = 10 };
        var config = new PollingScheduleConfig(HighThreshold: 5, LowThreshold: 2);
        var schedule = PollingSchedule.Calculate(zoneCounts, config);

        // Act & Assert — high priority polls every cycle
        await Assert.That(schedule.ShouldPollInCycle(100, 0)).IsTrue();
        await Assert.That(schedule.ShouldPollInCycle(100, 1)).IsTrue();
        await Assert.That(schedule.ShouldPollInCycle(100, 2)).IsTrue();
        await Assert.That(schedule.ShouldPollInCycle(100, 3)).IsTrue();
    }

    [Test]
    public async Task Should_DetermineIfShouldPoll_When_NormalPriority()
    {
        // Arrange
        var zoneCounts = new Dictionary<int, int> { [100] = 3 };
        var config = new PollingScheduleConfig(HighThreshold: 5, LowThreshold: 2);
        var schedule = PollingSchedule.Calculate(zoneCounts, config);

        // Act & Assert — normal priority polls every other cycle
        await Assert.That(schedule.ShouldPollInCycle(100, 0)).IsTrue();
        await Assert.That(schedule.ShouldPollInCycle(100, 1)).IsFalse();
        await Assert.That(schedule.ShouldPollInCycle(100, 2)).IsTrue();
        await Assert.That(schedule.ShouldPollInCycle(100, 3)).IsFalse();
    }

    [Test]
    public async Task Should_DetermineIfShouldPoll_When_LowPriority()
    {
        // Arrange
        var zoneCounts = new Dictionary<int, int> { [100] = 1 };
        var config = new PollingScheduleConfig(HighThreshold: 5, LowThreshold: 2);
        var schedule = PollingSchedule.Calculate(zoneCounts, config);

        // Act & Assert — low priority polls every 4th cycle
        await Assert.That(schedule.ShouldPollInCycle(100, 0)).IsTrue();
        await Assert.That(schedule.ShouldPollInCycle(100, 1)).IsFalse();
        await Assert.That(schedule.ShouldPollInCycle(100, 2)).IsFalse();
        await Assert.That(schedule.ShouldPollInCycle(100, 3)).IsFalse();
        await Assert.That(schedule.ShouldPollInCycle(100, 4)).IsTrue();
    }
}
