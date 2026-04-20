namespace TownCrier.Application.UserProfiles;

/// <summary>
/// Scans all user profiles for inactivity older than the retention window and
/// deletes them via <see cref="DeleteUserProfileCommandHandler"/> so the full
/// erasure cascade (watch zones, saved applications, device registrations,
/// notifications, decision alerts, Auth0) runs for each dormant account.
///
/// Designed to be invoked once per day by the worker. Keeping the retention
/// window as a constant here (not configuration) ensures the privacy policy's
/// "12 months of inactivity" promise is enforced uniformly in code.
/// </summary>
public sealed class DormantAccountCleanupCommandHandler
{
    private const int RetentionMonths = 12;

    private readonly IUserProfileRepository repository;
    private readonly DeleteUserProfileCommandHandler deleteHandler;

    public DormantAccountCleanupCommandHandler(
        IUserProfileRepository repository,
        DeleteUserProfileCommandHandler deleteHandler)
    {
        this.repository = repository;
        this.deleteHandler = deleteHandler;
    }

    public async Task<DormantAccountCleanupResult> HandleAsync(
        DormantAccountCleanupCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var cutoff = command.Now.AddMonths(-RetentionMonths);
        var dormant = await this.repository.GetDormantAsync(cutoff, ct).ConfigureAwait(false);

        var deleted = 0;
        foreach (var profile in dormant)
        {
            try
            {
                await this.deleteHandler
                    .HandleAsync(new DeleteUserProfileCommand(profile.UserId), ct)
                    .ConfigureAwait(false);
                deleted++;
            }
            catch (UserProfileNotFoundException)
            {
                // Tolerate the race where another caller deleted the profile
                // between the dormant scan and the cascade delete.
            }
        }

        return new DormantAccountCleanupResult(deleted);
    }
}
