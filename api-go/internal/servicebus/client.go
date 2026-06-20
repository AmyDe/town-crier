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

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// serviceBusSuffix is the public Azure Service Bus DNS suffix. A bare namespace
// name has it appended; a value already carrying it is left unchanged so the
// FQDN is never doubled.
const serviceBusSuffix = ".servicebus.windows.net"

// Sentinel errors for construction-time validation.
var (
	ErrMissingNamespace = errors.New("service bus namespace is required")
	ErrMissingQueue     = errors.New("service bus queue name is required")
)

// QueueDepth is a snapshot of the trigger queue's active and scheduled message
// counts. The bootstrapper seeds a new trigger only when both counts are zero
// (IsEmpty).
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
// message is a "run once now" tick. A scheduled enqueue (server-side) defers
// delivery without holding a goroutine.
func (c *Client) PublishAt(ctx context.Context, scheduledEnqueueTime time.Time, body []byte) (err error) {
	sender, err := c.sbClient.NewSender(c.queueName, nil)
	if err != nil {
		return fmt.Errorf("build sender: %w", err)
	}
	// Surface a close failure only when the publish itself succeeded, so a
	// genuine send error is never masked by a teardown error.
	defer func() {
		if closeErr := sender.Close(ctx); closeErr != nil && err == nil {
			err = fmt.Errorf("close sender: %w", closeErr)
		}
	}()

	enqueue := scheduledEnqueueTime.UTC()
	msg := &azservicebus.Message{
		Body:                 body,
		ContentType:          platform.Ptr("application/json"),
		ScheduledEnqueueTime: &enqueue,
	}
	if err := sender.SendMessage(ctx, msg, nil); err != nil {
		return fmt.Errorf("publish poll trigger: %w", err)
	}
	return nil
}

// receiveWaitTimeout bounds a single receive-and-delete attempt so an empty
// queue returns promptly rather than blocking until a message arrives (5s).
const receiveWaitTimeout = 5 * time.Second

// ReceiveTrigger destructively receives one poll-trigger message in
// receive-and-delete mode (ADR 0024 amendment): the message is removed from the
// queue the instant it is received — there is no lock, no Complete, no Abandon.
// It reports whether a message was consumed (false when the queue is empty
// within the receive window). The body is discarded: the trigger is a "run once"
// tick, so its presence is all the orchestrator needs.
//
// A receiver is opened per call and closed after; the orchestrator runs once per
// process so there is no hot-path receiver to pool. The wait is bounded by
// receiveWaitTimeout so an empty queue does not block the cycle.
func (c *Client) ReceiveTrigger(ctx context.Context) (received bool, err error) {
	receiver, err := c.sbClient.NewReceiverForQueue(c.queueName, &azservicebus.ReceiverOptions{
		ReceiveMode: azservicebus.ReceiveModeReceiveAndDelete,
	})
	if err != nil {
		return false, fmt.Errorf("build receiver: %w", err)
	}
	defer func() {
		if closeErr := receiver.Close(ctx); closeErr != nil && err == nil {
			err = fmt.Errorf("close receiver: %w", closeErr)
		}
	}()

	waitCtx, cancel := context.WithTimeout(ctx, receiveWaitTimeout)
	defer cancel()

	msgs, err := receiver.ReceiveMessages(waitCtx, 1, nil)
	if err != nil {
		// A timeout waiting for a message means an empty queue, not a failure: the
		// deadline-exceeded is scoped to waitCtx. A caller-cancelled parent ctx is
		// a genuine error — distinguish the two via the parent ctx.
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return false, nil
		}
		return false, fmt.Errorf("receive poll trigger: %w", err)
	}
	return len(msgs) > 0, nil
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
