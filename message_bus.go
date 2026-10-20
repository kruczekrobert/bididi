package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

type MessageBus interface {
	Publish(ctx context.Context, destination string, data []byte) error
	Observe(ctx context.Context, destination string) (MessageObserver, error)
}

type MessageObserver interface {
	Next(ctx context.Context) (Message, error)
	Close() error
}

type Message struct {
	ID   string
	Data []byte
}

type observedMessage struct {
	message Message
	json    interface{}
	valid   bool
}

func (f *FeaturesContext) afterScenario(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
	return ctx, f.closeMessageObservers()
}

func (f *FeaturesContext) closeMessageObservers() error {
	var closeErrors []error
	for destination, observer := range f.messageObservers {
		if err := observer.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close message observer for %q: %w", destination, err))
		}
	}
	f.messageObservers = make(map[string]MessageObserver)
	f.observedMessages = make(map[string][]observedMessage)
	return errors.Join(closeErrors...)
}

func (f *FeaturesContext) resolveMessageDestination(value string) (string, error) {
	resolved, err := f.wildcardProcessor.ResolveWildcards(value)
	if err != nil {
		return "", err
	}
	destination, ok := resolved.(string)
	if !ok || destination == "" {
		return "", fmt.Errorf("message destination must be a non-empty string")
	}
	return destination, nil
}

func (f *FeaturesContext) observeMessages(destination string) error {
	if f.messageBus == nil {
		return fmt.Errorf("message bus not configured")
	}
	destination, err := f.resolveMessageDestination(destination)
	if err != nil {
		return fmt.Errorf("message destination: %w", err)
	}
	if _, exists := f.messageObservers[destination]; exists {
		return fmt.Errorf("already observing messages from %q", destination)
	}
	observer, err := f.messageBus.Observe(context.Background(), destination)
	if err != nil {
		return fmt.Errorf("observe messages from %q: %w", destination, err)
	}
	if observer == nil {
		return fmt.Errorf("message bus returned no observer for %q", destination)
	}
	f.messageObservers[destination] = observer
	return nil
}

func (f *FeaturesContext) publishJSONMessage(destination string, body *godog.DocString) error {
	if f.messageBus == nil {
		return fmt.Errorf("message bus not configured")
	}
	if body == nil {
		return fmt.Errorf("JSON message requires a body DocString")
	}
	destination, err := f.resolveMessageDestination(destination)
	if err != nil {
		return fmt.Errorf("message destination: %w", err)
	}
	var payload interface{}
	if err := json.Unmarshal([]byte(body.Content), &payload); err != nil {
		return fmt.Errorf("invalid message JSON: %w", err)
	}
	processed, err := f.wildcardProcessor.SubstituteWildcards(payload)
	if err != nil {
		return fmt.Errorf("resolve message JSON: %w", err)
	}
	data, err := json.Marshal(processed)
	if err != nil {
		return fmt.Errorf("encode message JSON: %w", err)
	}
	if err := f.messageBus.Publish(context.Background(), destination, data); err != nil {
		return fmt.Errorf("publish message to %q: %w", destination, err)
	}
	return nil
}

func (f *FeaturesContext) assertMessages(timeoutValue, destination string, body *godog.DocString) error {
	if body == nil {
		return fmt.Errorf("expected JSON messages require a body DocString")
	}
	timeout, err := time.ParseDuration(timeoutValue)
	if err != nil || timeout <= 0 {
		return fmt.Errorf("invalid message timeout %q: expected a positive Go duration", timeoutValue)
	}
	destination, err = f.resolveMessageDestination(destination)
	if err != nil {
		return fmt.Errorf("message destination: %w", err)
	}
	observer, exists := f.messageObservers[destination]
	if !exists {
		return fmt.Errorf("messages from %q are not observed; use the observe step before publishing", destination)
	}
	processedJSON, err := f.jsonComparator.ProcessWildcardsInJSONString(body.Content)
	if err != nil {
		return fmt.Errorf("resolve expected message JSON: %w", err)
	}
	var expected []interface{}
	if err := json.Unmarshal([]byte(processedJSON), &expected); err != nil {
		return fmt.Errorf("expected messages must be a JSON array: %w", err)
	}
	if len(expected) == 0 {
		return fmt.Errorf("expected messages must be a non-empty JSON array")
	}
	for i, item := range expected {
		if _, ok := item.(map[string]interface{}); !ok {
			return fmt.Errorf("expected message %d must be a JSON object", i+1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		if f.messagesMatch(expected, f.observedMessages[destination]) {
			return nil
		}
		message, err := observer.Next(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return f.messageMismatchError(destination, timeout, body.Content)
			}
			return fmt.Errorf("read messages from %q: %w", destination, err)
		}
		if message.ID != "" && hasObservedMessageID(f.observedMessages[destination], message.ID) {
			continue
		}
		message.Data = bytes.Clone(message.Data)
		observed := observedMessage{message: message}
		observed.valid = json.Unmarshal(message.Data, &observed.json) == nil
		f.observedMessages[destination] = append(f.observedMessages[destination], observed)
	}
}

func hasObservedMessageID(messages []observedMessage, id string) bool {
	for _, message := range messages {
		if message.message.ID == id {
			return true
		}
	}
	return false
}

func (f *FeaturesContext) messagesMatch(expected []interface{}, observed []observedMessage) bool {
	used := make([]bool, len(observed))
	var match func(int) bool
	match = func(i int) bool {
		if i == len(expected) {
			return true
		}
		for j, message := range observed {
			if used[j] || !message.valid || f.matchMessageJSON(expected[i], message.json, "") != nil {
				continue
			}
			used[j] = true
			if match(i + 1) {
				return true
			}
			used[j] = false
		}
		return false
	}
	return match(0)
}

func (f *FeaturesContext) matchMessageJSON(expected, actual interface{}, path string) error {
	fields, ok := expected.(map[string]interface{})
	if !ok {
		return f.jsonComparator.compareValues(expected, actual, path)
	}
	actualFields, ok := actual.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected JSON object at %q, got %T", path, actual)
	}
	for key, value := range fields {
		actualValue, exists := actualFields[key]
		if !exists {
			return fmt.Errorf("missing field %q at %q", key, path)
		}
		if err := f.matchMessageJSON(value, actualValue, path+"."+key); err != nil {
			return err
		}
	}
	return nil
}

func (f *FeaturesContext) messageMismatchError(destination string, timeout time.Duration, expected string) error {
	var result strings.Builder
	fmt.Fprintf(&result, "messages from %q did not match within %s\nExpected JSON messages:\n%s\nObserved messages (%d):", destination, timeout, expected, len(f.observedMessages[destination]))
	if len(f.observedMessages[destination]) == 0 {
		result.WriteString(" none")
	}
	for _, observed := range f.observedMessages[destination] {
		fmt.Fprintf(&result, "\n  id=%q data=%s", observed.message.ID, observed.message.Data)
	}
	return errors.New(result.String())
}
