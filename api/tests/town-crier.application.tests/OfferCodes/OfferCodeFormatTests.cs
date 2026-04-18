using TownCrier.Application.OfferCodes;

namespace TownCrier.Application.Tests.OfferCodes;

public sealed class OfferCodeFormatTests
{
    [Test]
    [Arguments("A7KMZQR3FNXP", "A7KMZQR3FNXP")]   // already canonical
    [Arguments("a7kmzqr3fnxp", "A7KMZQR3FNXP")]   // lowercase
    [Arguments("A7KM-ZQR3-FNXP", "A7KMZQR3FNXP")]   // hyphens
    [Arguments("  A7KM ZQR3 FNXP ", "A7KMZQR3FNXP")]   // whitespace + spaces
    public async Task Normalize_Should_StripSeparatorsAndUppercase(string input, string expected)
    {
        var result = OfferCodeFormat.Normalize(input);
        await Assert.That(result).IsEqualTo(expected);
    }

    [Test]
    [Arguments("")]
    [Arguments("SHORT")]
    [Arguments("A7KMZQR3FNXPEXTRA")]
    [Arguments("A7KMZQR3FNXI")]  // excluded letter I
    public async Task Normalize_Should_Throw_When_InputInvalid(string input)
    {
        var act = () => OfferCodeFormat.Normalize(input);
        await Assert.That(act).Throws<InvalidOfferCodeFormatException>();
    }

    [Test]
    public async Task Format_Should_InsertHyphensEveryFourChars()
    {
        var display = OfferCodeFormat.Format("A7KMZQR3FNXP");
        await Assert.That(display).IsEqualTo("A7KM-ZQR3-FNXP");
    }

    [Test]
    public async Task IsValidCanonical_Should_ReturnTrue_For12AlphabetChars()
    {
        await Assert.That(OfferCodeFormat.IsValidCanonical("A7KMZQR3FNXP")).IsTrue();
    }

    [Test]
    [Arguments("a7kmzqr3fnxp")]
    [Arguments("A7KM-ZQR3-FNXP")]
    [Arguments("A7KMZQR3FNXI")]
    public async Task IsValidCanonical_Should_ReturnFalse_ForNonCanonical(string input)
    {
        await Assert.That(OfferCodeFormat.IsValidCanonical(input)).IsFalse();
    }
}
