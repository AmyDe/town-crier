namespace TownCrier.Application.UserProfiles;

public sealed class AutoGrantOptions
{
    public string ProDomains { get; set; } = string.Empty;

    public bool IsProDomain(string? email)
    {
        if (string.IsNullOrWhiteSpace(email) || string.IsNullOrWhiteSpace(this.ProDomains))
        {
            return false;
        }

        var atIndex = email.IndexOf('@', StringComparison.Ordinal);
        if (atIndex < 0)
        {
            return false;
        }

        var emailDomain = email[(atIndex + 1)..];

        var domains = this.ProDomains.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);

        return Array.Exists(domains, d => emailDomain.Equals(d, StringComparison.OrdinalIgnoreCase));
    }
}
