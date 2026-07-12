// Package servicebus is a thin, consumer-facing wrapper over the official
// azservicebus SDK for the Town Crier poll-trigger queue. It exposes only the
// operations the worker's poll modes need — probe the queue depth, publish a
// single scheduled trigger, receive-and-delete one trigger, and (for the
// bootstrap reconciler, GH#938 PR2) peek the queue, cancel a scheduled
// message, and drain the dead-letter sub-queue — and never leaks SDK types
// past its methods. Authentication is the pinned user-assigned managed
// identity (AZURE_CLIENT_ID); there is no SAS / connection-string path,
// mirroring the Cosmos identity model (see internal/platform/cosmos.go).
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

// QueueDepth is a snapshot of the trigger queue's active, scheduled and
// dead-lettered message counts. The bootstrapper's fork-guard (GH#938 PR2)
// thresholds on TriggerCount (active+scheduled): 0 seeds, 1 is a healthy
// single chain, >1 is a fork that must be collapsed. DeadLetterMessageCount is
// tracked separately — dead letters are corpses, not live triggers, and are
// handled by an unconditional drain rather than folded into the fork count.
type QueueDepth struct {
	ActiveMessageCount     int64
	ScheduledMessageCount  int64
	DeadLetterMessageCount int64
}

// TriggerCount returns the number of live (active+scheduled) trigger
// messages, excluding dead letters — the count the bootstrap reconciler
// thresholds on (GH#938 PR2).
func (d QueueDepth) TriggerCount() int64 {
	return d.ActiveMessageCount + d.ScheduledMessageCount
}

// IsEmpty reports whether the queue has no live (active+scheduled) trigger —
// the signal the bootstrapper uses to decide a reseed is needed. A queue
// holding only dead letters still counts as empty of live triggers.
func (d QueueDepth) IsEmpty() bool {
	return d.TriggerCount() == 0
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
		ActiveMessageCount:     int64(resp.ActiveMessageCount),
		ScheduledMessageCount:  int64(resp.ScheduledMessageCount),
		DeadLetterMessageCount: int64(resp.DeadLetterMessageCount),
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

// MessageState mirrors the subset of azservicebus.MessageState the bootstrap
// reconciler needs to distinguish, without leaking the SDK type past this
// package boundary.
type MessageState int

const (
	// MessageStateActive indicates the message is available for receipt now.
	MessageStateActive MessageState = iota
	// MessageStateScheduled indicates the message is not yet visible; it will
	// enqueue at ScheduledEnqueueTime.
	MessageStateScheduled
	// MessageStateDeferred indicates the message was explicitly deferred. The
	// trigger queue never defers messages, so the reconciler treats this the
	// same as active (a stray message to discard) rather than as a case worth
	// distinguishing.
	MessageStateDeferred
)

// PeekedMessage is a snapshot of one message's state on the trigger queue, as
// seen by PeekMessages — never locked or removed. SequenceNumber is the handle
// CancelScheduled needs to cancel it. ScheduledEnqueueTime is only meaningful
// when State is MessageStateScheduled.
type PeekedMessage struct {
	SequenceNumber       int64
	State                MessageState
	ScheduledEnqueueTime time.Time
}

// peekBatchSize bounds one PeekMessages call. The trigger queue is designed to
// carry exactly one live message (ADR 0024); this ceiling is generous headroom
// for a fork, not a hard limit a healthy queue could ever approach.
const peekBatchSize = 100

// PeekMessages returns a snapshot of up to peekBatchSize messages currently on
// the trigger queue — active or scheduled, with sequence numbers and (for
// scheduled messages) their activation time — without locking or removing
// them. The bootstrap reconciler uses it to decide which trigger to keep when
// the queue has forked (GH#938 PR2).
func (c *Client) PeekMessages(ctx context.Context) (_ []PeekedMessage, err error) {
	receiver, err := c.sbClient.NewReceiverForQueue(c.queueName, nil)
	if err != nil {
		return nil, fmt.Errorf("build peek receiver: %w", err)
	}
	defer func() {
		if closeErr := receiver.Close(ctx); closeErr != nil && err == nil {
			err = fmt.Errorf("close peek receiver: %w", closeErr)
		}
	}()

	msgs, err := receiver.PeekMessages(ctx, peekBatchSize, nil)
	if err != nil {
		return nil, fmt.Errorf("peek trigger queue: %w", err)
	}

	peeked := make([]PeekedMessage, 0, len(msgs))
	for _, m := range msgs {
		pm := PeekedMessage{}
		if m.SequenceNumber != nil {
			pm.SequenceNumber = *m.SequenceNumber
		}
		switch m.State {
		case azservicebus.MessageStateScheduled:
			pm.State = MessageStateScheduled
			if m.ScheduledEnqueueTime != nil {
				pm.ScheduledEnqueueTime = *m.ScheduledEnqueueTime
			}
		case azservicebus.MessageStateDeferred:
			pm.State = MessageStateDeferred
		default:
			pm.State = MessageStateActive
		}
		peeked = append(peeked, pm)
	}
	return peeked, nil
}

// CancelScheduled cancels one or more scheduled messages on the trigger queue
// by sequence number, as identified via PeekMessages. The bootstrap reconciler
// uses it to collapse a forked chain down to the single scheduled trigger it
// decided to keep (GH#938 PR2). A nil-error no-op when sequenceNumbers is empty.
func (c *Client) CancelScheduled(ctx context.Context, sequenceNumbers []int64) (err error) {
	if len(sequenceNumbers) == 0 {
		return nil
	}

	sender, err := c.sbClient.NewSender(c.queueName, nil)
	if err != nil {
		return fmt.Errorf("build cancel sender: %w", err)
	}
	defer func() {
		if closeErr := sender.Close(ctx); closeErr != nil && err == nil {
			err = fmt.Errorf("close cancel sender: %w", closeErr)
		}
	}()

	if err := sender.CancelScheduledMessages(ctx, sequenceNumbers, nil); err != nil {
		return fmt.Errorf("cancel scheduled triggers: %w", err)
	}
	return nil
}

// deadLetterDrainBatch bounds one dead-letter ReceiveMessages call. A handful
// of dead-lettered corpses is the expected case (GH#938's 2026-07-12 incident
// left 4); this is generous headroom, not a hard ceiling — DrainDeadLetters
// loops until the sub-queue reports empty regardless of how many batches that
// takes.
const deadLetterDrainBatch = 32

// DrainDeadLetters receives and completes every message on the trigger
// queue's dead-letter sub-queue, returning the number drained. It loops until a
// receive attempt within receiveWaitTimeout returns no messages (an empty
// DLQ), so a healthy queue with no dead letters returns (0, nil) promptly. The
// bootstrap reconciler drains the DLQ on every cycle it runs, regardless of the
// active/scheduled trigger count — dead-lettered corpses never self-clear
// (GH#938 PR2).
func (c *Client) DrainDeadLetters(ctx context.Context) (drained int, err error) {
	receiver, err := c.sbClient.NewReceiverForQueue(c.queueName, &azservicebus.ReceiverOptions{
		SubQueue: azservicebus.SubQueueDeadLetter,
	})
	if err != nil {
		return 0, fmt.Errorf("build dead-letter receiver: %w", err)
	}
	defer func() {
		if closeErr := receiver.Close(ctx); closeErr != nil && err == nil {
			err = fmt.Errorf("close dead-letter receiver: %w", closeErr)
		}
	}()

	for {
		waitCtx, cancel := context.WithTimeout(ctx, receiveWaitTimeout)
		msgs, recvErr := receiver.ReceiveMessages(waitCtx, deadLetterDrainBatch, nil)
		cancel()
		if recvErr != nil {
			if ctx.Err() != nil {
				return drained, ctx.Err()
			}
			if errors.Is(recvErr, context.DeadlineExceeded) {
				return drained, nil
			}
			return drained, fmt.Errorf("receive dead letters: %w", recvErr)
		}
		if len(msgs) == 0 {
			return drained, nil
		}
		for _, msg := range msgs {
			if completeErr := receiver.CompleteMessage(ctx, msg, nil); completeErr != nil {
				return drained, fmt.Errorf("complete dead letter: %w", completeErr)
			}
			drained++
		}
	}
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
