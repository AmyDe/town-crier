namespace TownCrier.Infrastructure.ServiceBus;

/// <summary>
/// Configuration for the Service Bus ARM management-plane client. The management
/// plane is addressed by full ARM resource path (subscription + resource group +
/// namespace + queue), not by data-plane FQDN — this is a different surface to
/// <see cref="ServiceBusRestOptions"/>.
/// </summary>
public sealed class ServiceBusManagementOptions
{
    public required string SubscriptionId { get; init; }

    public required string ResourceGroup { get; init; }

    /// <summary>
    /// Gets the Service Bus namespace name. Accepts either the bare name
    /// ("sb-town-crier-prod") or the data-plane FQDN
    /// ("sb-town-crier-prod.servicebus.windows.net") — the client strips the
    /// suffix before building the ARM path.
    /// </summary>
    public required string Namespace { get; init; }
}
