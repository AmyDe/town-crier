using TownCrier.Domain.Entitlements;

namespace TownCrier.Web.Endpoints;

[AttributeUsage(AttributeTargets.Method | AttributeTargets.Class)]
internal sealed class RequiresEntitlementAttribute(Entitlement entitlement) : Attribute
{
    public Entitlement Entitlement { get; } = entitlement;
}
