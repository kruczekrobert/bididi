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
	timeout, destination, observer, err := f.prepareMessageAssertion(timeoutValue, destination)
	if err != nil {
		return err
	}
	expected, err := f.parseExpectedMessages(body)
	if err != nil {
		return err
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
		f.bufferObservedMessage(destination, message)
	}
}

func (f *FeaturesContext) assertMessagesInOrder(timeoutValue, destination string, body *godog.DocString) error {
	timeout, destination, observer, err := f.prepareMessageAssertion(timeoutValue, destination)
	if err != nil {
		return err
	}
	expected, err := f.parseExpectedMessages(body)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		if f.messagesMatchInOrder(expected, f.observedMessages[destination]) {
			return nil
		}
		message, err := observer.Next(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return f.messageOrderMismatchError(destination, timeout, body.Content)
			}
			return fmt.Errorf("read messages from %q: %w", destination, err)
		}
		f.bufferObservedMessage(destination, message)
	}
}

func (f *FeaturesContext) assertMessagesAbsent(timeoutValue, destination string, body *godog.DocString) error {
	timeout, destination, observer, err := f.prepareMessageAssertion(timeoutValue, destination)
	if err != nil {
		return err
	}
	forbidden, err := f.parseExpectedMessages(body)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		if index, pattern, found := f.findMatchingMessage(forbidden, f.observedMessages[destination]); found {
			return fmt.Errorf("messages from %q contained forbidden JSON message %d at observed message %d: id=%q data=%s", destination, pattern+1, index+1, f.observedMessages[destination][index].message.ID, f.observedMessages[destination][index].message.Data)
		}
		message, err := observer.Next(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil
			}
			return fmt.Errorf("read messages from %q: %w", destination, err)
		}
		f.bufferObservedMessage(destination, message)
	}
}

func (f *FeaturesContext) extractVariableFromMessage(path string, messageNumber int, destination, variableName string) error {
	destination, err := f.resolveMessageDestination(destination)
	if err != nil {
		return fmt.Errorf("message destination: %w", err)
	}
	if _, exists := f.messageObservers[destination]; !exists {
		return fmt.Errorf("messages from %q are not observed; use the observe step before publishing", destination)
	}
	if messageNumber < 1 || messageNumber > len(f.observedMessages[destination]) {
		return fmt.Errorf("message %d from %q is not available; observed messages: %d", messageNumber, destination, len(f.observedMessages[destination]))
	}

	return f.extractVariableFromObservedMessage(path, messageNumber-1, destination, variableName)
}

func (f *FeaturesContext) extractVariableFromMatchingMessage(timeoutValue, path, destination, variableName string, body *godog.DocString) error {
	timeout, destination, observer, err := f.prepareMessageAssertion(timeoutValue, destination)
	if err != nil {
		return err
	}
	expected, err := f.parseExpectedMessage(body)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		if index, found := f.findFirstMatchingObservedMessage(expected, f.observedMessages[destination]); found {
			return f.extractVariableFromObservedMessage(path, index, destination, variableName)
		}
		message, err := observer.Next(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return f.matchingMessageExtractionError(destination, timeout, body.Content, path)
			}
			return fmt.Errorf("read messages from %q: %w", destination, err)
		}
		f.bufferObservedMessage(destination, message)
	}
}

func (f *FeaturesContext) extractVariableFromObservedMessage(path string, index int, destination, variableName string) error {
	messageNumber := index + 1
	observed := f.observedMessages[destination][index]
	if !observed.valid {
		return fmt.Errorf("message %d from %q is not valid JSON: %s", messageNumber, destination, observed.message.Data)
	}
	value, err := f.jsonComparator.extractValueByPath(observed.json, path)
	if err != nil {
		return fmt.Errorf("failed to extract value from message %d from %q at path %s: %w%s", messageNumber, destination, path, err, f.wildcardProcessor.GetVariablesDebugInfo())
	}
	actualVariableName, typePrefix := f.parseVariableNameWithPrefix(variableName)
	f.wildcardProcessor.SetVariable(actualVariableName, f.convertValueWithTypePrefix(value, typePrefix))
	return nil
}

func (f *FeaturesContext) assertMessageCount(timeoutValue, destination string, expectedCount int, body *godog.DocString) error {
	if expectedCount < 0 {
		return fmt.Errorf("expected message count must not be negative")
	}
	timeout, destination, observer, err := f.prepareMessageAssertion(timeoutValue, destination)
	if err != nil {
		return err
	}
	expected, err := f.parseExpectedMessage(body)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		message, err := observer.Next(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				actualCount := f.matchingMessageCount(expected, f.observedMessages[destination])
				if actualCount != expectedCount {
					return fmt.Errorf("messages from %q had count %d matching JSON within %s, expected exactly %d\nMatching JSON:\n%s", destination, actualCount, timeout, expectedCount, body.Content)
				}
				return nil
			}
			return fmt.Errorf("read messages from %q: %w", destination, err)
		}
		f.bufferObservedMessage(destination, message)
	}
}

func (f *FeaturesContext) prepareMessageAssertion(timeoutValue, destination string) (time.Duration, string, MessageObserver, error) {
	timeout, err := time.ParseDuration(timeoutValue)
	if err != nil || timeout <= 0 {
		return 0, "", nil, fmt.Errorf("invalid message timeout %q: expected a positive Go duration", timeoutValue)
	}
	destination, err = f.resolveMessageDestination(destination)
	if err != nil {
		return 0, "", nil, fmt.Errorf("message destination: %w", err)
	}
	observer, exists := f.messageObservers[destination]
	if !exists {
		return 0, "", nil, fmt.Errorf("messages from %q are not observed; use the observe step before publishing", destination)
	}
	return timeout, destination, observer, nil
}

func (f *FeaturesContext) parseExpectedMessages(body *godog.DocString) ([]interface{}, error) {
	if body == nil {
		return nil, fmt.Errorf("expected JSON messages require a body DocString")
	}
	processedJSON, err := f.jsonComparator.ProcessWildcardsInJSONString(body.Content)
	if err != nil {
		return nil, fmt.Errorf("resolve expected message JSON: %w", err)
	}
	var expected []interface{}
	if err := json.Unmarshal([]byte(processedJSON), &expected); err != nil {
		return nil, fmt.Errorf("expected messages must be a JSON array: %w", err)
	}
	if len(expected) == 0 {
		return nil, fmt.Errorf("expected messages must be a non-empty JSON array")
	}
	for i, item := range expected {
		if _, ok := item.(map[string]interface{}); !ok {
			return nil, fmt.Errorf("expected message %d must be a JSON object", i+1)
		}
	}
	return expected, nil
}

func (f *FeaturesContext) parseExpectedMessage(body *godog.DocString) (interface{}, error) {
	if body == nil {
		return nil, fmt.Errorf("expected message JSON requires a body DocString")
	}
	processedJSON, err := f.jsonComparator.ProcessWildcardsInJSONString(body.Content)
	if err != nil {
		return nil, fmt.Errorf("resolve expected message JSON: %w", err)
	}
	var expected interface{}
	if err := json.Unmarshal([]byte(processedJSON), &expected); err != nil {
		return nil, fmt.Errorf("expected message must be a JSON object: %w", err)
	}
	if _, ok := expected.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("expected message must be a JSON object")
	}
	return expected, nil
}

func (f *FeaturesContext) bufferObservedMessage(destination string, message Message) bool {
	if message.ID != "" && hasObservedMessageID(f.observedMessages[destination], message.ID) {
		return false
	}
	message.Data = bytes.Clone(message.Data)
	observed := observedMessage{message: message}
	observed.valid = json.Unmarshal(message.Data, &observed.json) == nil
	f.observedMessages[destination] = append(f.observedMessages[destination], observed)
	return true
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

func (f *FeaturesContext) messagesMatchInOrder(expected []interface{}, observed []observedMessage) bool {
	expectedIndex := 0
	for _, message := range observed {
		if !message.valid || f.matchMessageJSON(expected[expectedIndex], message.json, "") != nil {
			continue
		}
		expectedIndex++
		if expectedIndex == len(expected) {
			return true
		}
	}
	return false
}

func (f *FeaturesContext) findFirstMatchingObservedMessage(expected interface{}, observed []observedMessage) (int, bool) {
	for i, message := range observed {
		if message.valid && f.matchMessageJSON(expected, message.json, "") == nil {
			return i, true
		}
	}
	return 0, false
}

func (f *FeaturesContext) findMatchingMessage(expected []interface{}, observed []observedMessage) (int, int, bool) {
	for i, message := range observed {
		if !message.valid {
			continue
		}
		for j, pattern := range expected {
			if f.matchMessageJSON(pattern, message.json, "") == nil {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}

func (f *FeaturesContext) matchingMessageCount(expected interface{}, observed []observedMessage) int {
	count := 0
	for _, message := range observed {
		if message.valid && f.matchMessageJSON(expected, message.json, "") == nil {
			count++
		}
	}
	return count
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

func (f *FeaturesContext) messageOrderMismatchError(destination string, timeout time.Duration, expected string) error {
	var result strings.Builder
	fmt.Fprintf(&result, "messages from %q did not contain the expected JSON messages in order within %s\nExpected ordered JSON messages:\n%s\nObserved messages (%d):", destination, timeout, expected, len(f.observedMessages[destination]))
	f.appendObservedMessages(&result, destination)
	return errors.New(result.String())
}

func (f *FeaturesContext) matchingMessageExtractionError(destination string, timeout time.Duration, expected, path string) error {
	var result strings.Builder
	fmt.Fprintf(&result, "no message from %q matching JSON was available within %s for extraction at path %q\nMatching JSON:\n%s\nObserved messages (%d):", destination, timeout, path, expected, len(f.observedMessages[destination]))
	f.appendObservedMessages(&result, destination)
	return errors.New(result.String())
}

func (f *FeaturesContext) appendObservedMessages(result *strings.Builder, destination string) {
	if len(f.observedMessages[destination]) == 0 {
		result.WriteString(" none")
	}
	for _, observed := range f.observedMessages[destination] {
		fmt.Fprintf(result, "\n  id=%q data=%s", observed.message.ID, observed.message.Data)
	}
}
