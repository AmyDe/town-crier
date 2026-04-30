using TownCrier.Domain.Notifications;

namespace TownCrier.Domain.Tests.Notifications;

public sealed class UkPlanningVocabularyTests
{
    [Test]
    [Arguments("Permitted", "Approved")]
    [Arguments("Conditions", "Approved with conditions")]
    [Arguments("Rejected", "Refused")]
    [Arguments("Appealed", "Refusal appealed")]
    [Arguments("permitted", "Approved")]
    [Arguments("REJECTED", "Refused")]
    public async Task Should_MapPlanItStateToUkDisplayString_When_StateIsADecision(
        string planItAppState, string expectedDisplay)
    {
        // Act
        var result = UkPlanningVocabulary.GetDisplayString(planItAppState);

        // Assert
        await Assert.That(result).IsEqualTo(expectedDisplay);
    }

    [Test]
    [Arguments("Undecided")]
    [Arguments("Withdrawn")]
    [Arguments("Unresolved")]
    [Arguments("Referred")]
    [Arguments("")]
    [Arguments(null)]
    public async Task Should_ReturnNull_When_StateIsNotADecision(string? planItAppState)
    {
        // Act
        var result = UkPlanningVocabulary.GetDisplayString(planItAppState);

        // Assert
        await Assert.That(result).IsNull();
    }
}
