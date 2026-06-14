// Package servicebus is a thin, consumer-facing wrapper over the official
// azservicebus SDK for the Town Crier poll-trigger queue. It exposes only the
// two operations the worker's poll modes need — probe the queue depth and
// publish a single scheduled trigger — and never leaks SDK types past its
// methods. Authentication is the pinned user-assigned managed identity
// (AZURE_CLIENT_ID); there is no SAS / connection-string path, mirroring the
// Cosmos identity model (see internal/platform/cosmos.go).
package servicebus

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

// serviceBusSuffix is the public Azure Service Bus DNS suffix. A bare namespace
// name has it appended; a value already carrying it is left unchanged so the
// FQDN is never doubled (the .NET worker hit NXDOMAIN when it doubled the
// suffix — see ServiceBusServiceExtensions.cs).
const serviceBusSuffix = ".servicebus.windows.net"

// Sentinel errors for construction-time validation.
var (
	ErrMissingNamespace = errors.New("service bus namespace is required")
	ErrMissingQueue     = errors.New("service bus queue name is required")
)

// QueueDepth is a snapshot of the trigger queue's active and scheduled message
// counts. It mirrors .NET's PollTriggerQueueDepth: the bootstrapper seeds a new
// trigger only when both counts are zero (IsEmpty).
type QueueDepth struct {
	ActiveMessageCount    int64
	ScheduledMessageCount int64
}

// IsEmpty reports whether the queue has no active and no scheduled messages —
// the signal the bootstrapper uses to decide a reseed is needed.
func (d QueueDepth) IsEmpty() bool {
	return d.ActiveMessageCount == 0 && d.ScheduledMessageCount == 0
}

// Client wraps the azservicebus data-plane and management clients for one
// queue. Construct it with NewClient and release it with Close.
type Client struct {
	queueName string
	sbClient  *azservicebus.Client
	admin     *admin.Client
}

// NewClient builds a Service Bus client for the given namespace and queue,
// authenticated by the pinned user-assigned managed identity (azureClientID).
// When azureClientID is empty the SDK falls back to the ambient managed
// identity. The namespace may be a bare name or a full FQDN; either is accepted.
// Connections open lazily on first call, preserving cold-start latency.
func NewClient(namespace, queueName, azureClientID string) (*Client, error) {
	if namespace == "" {
		return nil, ErrMissingNamespace
	}
	if queueName == "" {
		return nil, ErrMissingQueue
	}

	credOpts := &azidentity.ManagedIdentityCredentialOptions{}
	if azureClientID != "" {
		credOpts.ID = azidentity.ClientID(azureClientID)
	}
	cred, err := azidentity.NewManagedIdentityCredential(credOpts)
	if err != nil {
		return nil, fmt.Errorf("build managed-identity credential: %w", err)
	}

	fqdn := normalizeFQDN(namespace)

	sbClient, err := azservicebus.NewClient(fqdn, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("build service bus client: %w", err)
	}

	adminClient, err := admin.NewClient(fqdn, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("build service bus admin client: %w", err)
	}

	return &Client{
		queueName: queueName,
		sbClient:  sbClient,
		admin:     adminClient,
	}, nil
}

// QueueName returns the trigger queue this client targets.
func (c *Client) QueueName() string { return c.queueName }

// QueueDepth reads the queue's active and scheduled message counts via the
// Service Bus management API. The bootstrapper uses this to decide whether the
// adaptive polling chain is alive (non-empty) or needs reseeding (empty).
func (c *Client) QueueDepth(ctx context.Context) (QueueDepth, error) {
	resp, err := c.admin.GetQueueRuntimeProperties(ctx, c.queueName, nil)
	if err != nil {
		return QueueDepth{}, fmt.Errorf("read queue runtime properties: %w", err)
	}
	if resp == nil {
		return QueueDepth{}, fmt.Errorf("queue %q not found", c.queueName)
	}
	return QueueDepth{
		ActiveMessageCount:    int64(resp.ActiveMessageCount),
		ScheduledMessageCount: int64(resp.ScheduledMessageCount),
	}, nil
}

// PublishAt publishes one poll-trigger message scheduled to enqueue at
// scheduledEnqueueTime. The body carries only a diagnostic timestamp — the
// message is a "run once now" tick, matching .NET's PollTriggerPayload. A
// scheduled enqueue (server-side) defers delivery without holding a goroutine.
func (c *Client) PublishAt(ctx context.Context, scheduledEnqueueTime time.Time, body []byte) error {
	sender, err := c.sbClient.NewSender(c.queueName, nil)
	if err != nil {
		return fmt.Errorf("build sender: %w", err)
	}
	defer func() { _ = sender.Close(ctx) }()

	enqueue := scheduledEnqueueTime.UTC()
	msg := &azservicebus.Message{
		Body:                 body,
		ContentType:          ptr("application/json"),
		ScheduledEnqueueTime: &enqueue,
	}
	if err := sender.SendMessage(ctx, msg, nil); err != nil {
		return fmt.Errorf("publish poll trigger: %w", err)
	}
	return nil
}

// Close releases the underlying SDK clients.
func (c *Client) Close(ctx context.Context) error {
	if c.sbClient == nil {
		return nil
	}
	return c.sbClient.Close(ctx)
}

// normalizeFQDN appends the Service Bus DNS suffix to a bare namespace name,
// leaving a value that already carries it (case-insensitive) unchanged.
func normalizeFQDN(namespace string) string {
	if strings.HasSuffix(strings.ToLower(namespace), serviceBusSuffix) {
		return namespace
	}
	return namespace + serviceBusSuffix
}

func ptr[T any](v T) *T { return &v }
