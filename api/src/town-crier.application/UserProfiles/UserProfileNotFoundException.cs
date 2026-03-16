namespace TownCrier.Application.UserProfiles;

public sealed class UserProfileNotFoundException : Exception
{
    public UserProfileNotFoundException()
        : base("User profile not found.")
    {
    }

    public UserProfileNotFoundException(string message)
        : base(message)
    {
    }

    public UserProfileNotFoundException(string message, Exception innerException)
        : base(message, innerException)
    {
    }

    public string? UserId { get; private set; }

    public static UserProfileNotFoundException ForUser(string userId)
    {
        return new UserProfileNotFoundException($"User profile not found for user '{userId}'.")
        {
            UserId = userId,
        };
    }
}
