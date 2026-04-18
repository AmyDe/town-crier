using TownCrier.Application.OfferCodes;

namespace TownCrier.Application.Tests.OfferCodes;

public sealed class OfferCodeGeneratorTests
{
    [Test]
    public async Task Generate_Should_Return12CharCanonicalCode()
    {
        var generator = new OfferCodeGenerator();

        var code = generator.Generate();

        await Assert.That(code).HasLength().EqualTo(OfferCodeFormat.CanonicalLength);
        await Assert.That(OfferCodeFormat.IsValidCanonical(code)).IsTrue();
    }

    [Test]
    public async Task Generate_Should_ReturnDifferentCodesEachCall()
    {
        var generator = new OfferCodeGenerator();

        var codes = Enumerable.Range(0, 100).Select(_ => generator.Generate()).ToHashSet();

        await Assert.That(codes).HasCount().EqualTo(100);
    }
}
