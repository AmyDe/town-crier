using TownCrier.Application.OfferCodes;

namespace TownCrier.Application.Tests.OfferCodes;

public sealed class FakeOfferCodeGenerator : IOfferCodeGenerator
{
    private readonly Queue<string> codes;

    public FakeOfferCodeGenerator(params string[] codes)
    {
        this.codes = new Queue<string>(codes);
    }

    public string Generate()
    {
        if (this.codes.Count == 0)
        {
            throw new InvalidOperationException("FakeOfferCodeGenerator exhausted.");
        }

        return this.codes.Dequeue();
    }
}
