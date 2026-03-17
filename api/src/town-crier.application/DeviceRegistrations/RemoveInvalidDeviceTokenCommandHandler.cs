namespace TownCrier.Application.DeviceRegistrations;

public sealed class RemoveInvalidDeviceTokenCommandHandler
{
    private readonly IDeviceRegistrationRepository repository;

    public RemoveInvalidDeviceTokenCommandHandler(IDeviceRegistrationRepository repository)
    {
        this.repository = repository;
    }

    public async Task HandleAsync(RemoveInvalidDeviceTokenCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        await this.repository.DeleteByTokenAsync(command.Token, ct).ConfigureAwait(false);
    }
}
