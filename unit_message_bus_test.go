package bdd

import (
	"context"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testMessageObserver struct {
	messages chan Message
}

func newTestMessageContext(messages ...Message) (*FeaturesContext, *testMessageObserver) {
	f := newFeaturesContext()
	observer := &testMessageObserver{messages: make(chan Message, len(messages))}
	for _, message := range messages {
		observer.messages <- message
	}
	f.messageObservers["events"] = observer
	return f, observer
}

func (o *testMessageObserver) Next(ctx context.Context) (Message, error) {
	select {
	case message := <-o.messages:
		return message, nil
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}

func (o *testMessageObserver) Close() error { return nil }

func TestAssertMessagesAbsentWaitsForTheWholeWindow(t *testing.T) {
	f, _ := newTestMessageContext(Message{ID: "1", Data: []byte(`{"type":"OrderCreated"}`)})
	started := time.Now()

	err := f.assertMessagesAbsent("30ms", "events", &godog.DocString{Content: `[{"type":"PaymentFailed"}]`})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(started), 20*time.Millisecond)
	assert.Len(t, f.observedMessages["events"], 1)
}

func TestAssertMessagesAbsentRejectsAForbiddenBufferedMessage(t *testing.T) {
	f, _ := newTestMessageContext()
	f.bufferObservedMessage("events", Message{ID: "1", Data: []byte(`{"type":"PaymentFailed"}`)})

	err := f.assertMessagesAbsent("1s", "events", &godog.DocString{Content: `[{"type":"PaymentFailed"}]`})

	require.ErrorContains(t, err, "contained forbidden JSON message 1 at observed message 1")
}

func TestAssertMessagesAbsentCatchesAForbiddenMessageNearTheEndOfTheWindow(t *testing.T) {
	f, observer := newTestMessageContext()
	go func() {
		time.Sleep(20 * time.Millisecond)
		observer.messages <- Message{ID: "1", Data: []byte(`{"type":"PaymentFailed"}`)}
	}()

	err := f.assertMessagesAbsent("100ms", "events", &godog.DocString{Content: `[{"type":"PaymentFailed"}]`})

	require.ErrorContains(t, err, "contained forbidden JSON message 1 at observed message 1")
}

func TestExtractVariableFromMessageUsesOneBasedArrivalOrder(t *testing.T) {
	f, _ := newTestMessageContext()
	f.bufferObservedMessage("events", Message{ID: "1", Data: []byte(`{"type":"OrderCreated","payload":{"id":41}}`)})
	f.bufferObservedMessage("events", Message{ID: "2", Data: []byte(`{"type":"InvoiceRequested","payload":{"id":42}}`)})

	require.NoError(t, f.extractVariableFromMessage("payload.id", 2, "events", "int:event_id"))
	value, exists := f.wildcardProcessor.GetVariable("event_id")
	require.True(t, exists)
	assert.Equal(t, 42, value)

	err := f.extractVariableFromMessage("payload.id", 3, "events", "missing")
	require.ErrorContains(t, err, `message 3 from "events" is not available; observed messages: 2`)
}

func TestAssertMessageCountDeduplicatesStableIDsAndWaitsForTheWholeWindow(t *testing.T) {
	f, _ := newTestMessageContext(
		Message{ID: "1", Data: []byte(`{"type":"OrderCreated"}`)},
		Message{ID: "1", Data: []byte(`{"type":"OrderCreated"}`)},
		Message{ID: "2", Data: []byte(`{"type":"OrderCreated"}`)},
		Message{ID: "3", Data: []byte(`{"type":"InvoiceRequested"}`)},
	)
	started := time.Now()

	err := f.assertMessageCount("30ms", "events", 2, &godog.DocString{Content: `{"type":"OrderCreated"}`})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(started), 20*time.Millisecond)
	assert.Len(t, f.observedMessages["events"], 3)
}

func TestAssertMessageCountReportsTheExactCount(t *testing.T) {
	f, _ := newTestMessageContext(Message{ID: "1", Data: []byte(`{"type":"OrderCreated"}`)})

	err := f.assertMessageCount("10ms", "events", 2, &godog.DocString{Content: `{"type":"OrderCreated"}`})

	require.ErrorContains(t, err, `had count 1 matching JSON`)
	require.ErrorContains(t, err, `expected exactly 2`)
}

func TestAssertMessagesInOrderAllowsUnrelatedMessages(t *testing.T) {
	f, _ := newTestMessageContext(
		Message{ID: "1", Data: []byte(`{"type":"Unrelated"}`)},
		Message{ID: "2", Data: []byte(`{"type":"OrderCreated","order_id":"42"}`)},
		Message{ID: "3", Data: []byte(`{"type":"AuditRecorded"}`)},
		Message{ID: "4", Data: []byte(`{"type":"InvoiceRequested","order_id":"42"}`)},
	)

	err := f.assertMessagesInOrder("100ms", "events", &godog.DocString{Content: `[
      {"type":"OrderCreated","order_id":"42"},
      {"type":"InvoiceRequested","order_id":"42"}
    ]`})

	require.NoError(t, err)
	assert.Len(t, f.observedMessages["events"], 4)
}

func TestAssertMessagesInOrderRejectsReversedEvents(t *testing.T) {
	f, _ := newTestMessageContext(
		Message{ID: "1", Data: []byte(`{"type":"InvoiceRequested","order_id":"42"}`)},
		Message{ID: "2", Data: []byte(`{"type":"OrderCreated","order_id":"42"}`)},
	)

	err := f.assertMessagesInOrder("20ms", "events", &godog.DocString{Content: `[
      {"type":"OrderCreated","order_id":"42"},
      {"type":"InvoiceRequested","order_id":"42"}
    ]`})

	require.ErrorContains(t, err, "did not contain the expected JSON messages in order")
}

func TestExtractVariableFromFirstMatchingMessageWaitsAndIgnoresUnrelatedMessages(t *testing.T) {
	f, _ := newTestMessageContext(
		Message{ID: "1", Data: []byte(`{"type":"Unrelated","payload":{"order_id":"wrong"}}`)},
		Message{ID: "2", Data: []byte(`{"type":"OrderCreated","correlation_id":"trace-42","payload":{"order_id":"42"}}`)},
	)
	f.wildcardProcessor.SetVariable("correlation_id", "trace-42")

	err := f.extractVariableFromMatchingMessage(
		"100ms",
		"payload.order_id",
		"events",
		"str:order_id",
		&godog.DocString{Content: `{"type":"OrderCreated","correlation_id":"$VAR(correlation_id)$"}`},
	)

	require.NoError(t, err)
	value, exists := f.wildcardProcessor.GetVariable("order_id")
	require.True(t, exists)
	assert.Equal(t, "42", value)
}

func TestExtractVariableFromFirstMatchingMessageReportsTimeout(t *testing.T) {
	f, _ := newTestMessageContext(Message{ID: "1", Data: []byte(`{"type":"Unrelated"}`)})

	err := f.extractVariableFromMatchingMessage(
		"20ms",
		"payload.order_id",
		"events",
		"order_id",
		&godog.DocString{Content: `{"type":"OrderCreated"}`},
	)

	require.ErrorContains(t, err, `no message from "events" matching JSON was available`)
}
