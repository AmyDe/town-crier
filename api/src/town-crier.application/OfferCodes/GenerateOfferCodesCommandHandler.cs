using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.OfferCodes;

public sealed class GenerateOfferCodesCommandHandler
{
    private const int MaxCount = 1000;
    private const int MaxGenerationAttempts = 5;

    private readonly IOfferCodeRepository repository;
    private readonly IOfferCodeGenerator generator;
    private readonly TimeProvider timeProvider;

    public GenerateOfferCodesCommandHandler(
        IOfferCodeRepository repository,
        IOfferCodeGenerator generator,
        TimeProvider timeProvider)
    {
        this.repository = repository;
        this.generator = generator;
        this.timeProvider = timeProvider;
    }

    public async Task<GenerateOfferCodesResult> HandleAsync(
        GenerateOfferCodesCommand command,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        if (command.Count < 1 || command.Count > MaxCount)
        {
            throw new ArgumentOutOfRangeException(
                nameof(command),
                command.Count,
                $"Count must be between 1 and {MaxCount}.");
        }

        if (command.Tier == SubscriptionTier.Free)
        {
            throw new ArgumentException("Offer codes cannot grant the Free tier.", nameof(command));
        }

        if (command.DurationDays < 1 || command.DurationDays > 365)
        {
            throw new ArgumentOutOfRangeException(
                nameof(command),
                command.DurationDays,
                "DurationDays must be between 1 and 365.");
        }

        var createdAt = this.timeProvider.GetUtcNow();
        var codes = new List<string>(command.Count);

        for (var i = 0; i < command.Count; i++)
        {
            var code = await this.CreateUniqueCodeAsync(command.Tier, command.DurationDays, createdAt, ct)
                .ConfigureAwait(false);
            codes.Add(code.Code);
        }

        return new GenerateOfferCodesResult(codes);
    }

    private async Task<OfferCode> CreateUniqueCodeAsync(
        SubscriptionTier tier,
        int durationDays,
        DateTimeOffset createdAt,
        CancellationToken ct)
    {
        for (var attempt = 0; attempt < MaxGenerationAttempts; attempt++)
        {
            var canonical = this.generator.Generate();
            var offerCode = new OfferCode(canonical, tier, durationDays, createdAt);

            try
            {
                await this.repository.CreateAsync(offerCode, ct).ConfigureAwait(false);
                return offerCode;
            }
            catch (InvalidOperationException)
            {
                // Collision — try again.
            }
        }

        throw new InvalidOperationException(
            $"Could not generate a unique offer code after {MaxGenerationAttempts} attempts.");
    }
}
