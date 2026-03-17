using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.DeviceRegistrations;

public sealed class RegisterDeviceTokenCommandHandler
{
    private readonly IDeviceRegistrationRepository repository;
    private readonly TimeProvider timeProvider;

    public RegisterDeviceTokenCommandHandler(
        IDeviceRegistrationRepository repository,
        TimeProvider timeProvider)
    {
        this.repository = repository;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(RegisterDeviceTokenCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var existing = await this.repository.GetByTokenAsync(command.Token, ct).ConfigureAwait(false);

        if (existing is not null)
        {
            existing.RefreshRegistration(this.timeProvider.GetUtcNow());
            await this.repository.SaveAsync(existing, ct).ConfigureAwait(false);
            return;
        }

        var registration = DeviceRegistration.Create(
            command.UserId,
            command.Token,
            command.Platform,
            this.timeProvider.GetUtcNow());

        await this.repository.SaveAsync(registration, ct).ConfigureAwait(false);
    }
}
