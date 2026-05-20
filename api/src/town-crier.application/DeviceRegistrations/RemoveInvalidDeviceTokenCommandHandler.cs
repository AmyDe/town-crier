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

        // Orphan-row note: if a token was previously registered under a different userId
        // (e.g. device reinstalled and signed in with a new account), only the current
        // user's row is removed here. The prior user's orphaned row is collected by the
        // APNs invalid-token callback or by the 180-day TTL. Accepted per GH#395.
        await this.repository.DeleteByTokenAsync(command.UserId, command.Token, ct).ConfigureAwait(false);
    }
}
