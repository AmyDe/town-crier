using TownCrier.Domain.Authorities;

namespace TownCrier.Application.Tests.Authorities;

internal sealed class AuthorityBuilder
{
    private int id = 1;
    private string name = "Test Council";
    private string areaType = "London Borough";
#pragma warning disable S1075
    private string? councilUrl = "https://example.gov.uk";
    private string? planningUrl = "https://example.gov.uk/planning";
#pragma warning restore S1075

    public AuthorityBuilder WithId(int id)
    {
        this.id = id;
        return this;
    }

    public AuthorityBuilder WithName(string name)
    {
        this.name = name;
        return this;
    }

    public AuthorityBuilder WithAreaType(string areaType)
    {
        this.areaType = areaType;
        return this;
    }

    public AuthorityBuilder WithCouncilUrl(string? councilUrl)
    {
        this.councilUrl = councilUrl;
        return this;
    }

    public AuthorityBuilder WithPlanningUrl(string? planningUrl)
    {
        this.planningUrl = planningUrl;
        return this;
    }

    public Authority Build()
    {
        return new Authority(this.id, this.name, this.areaType, this.councilUrl, this.planningUrl);
    }
}
