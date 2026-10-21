package bdd

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestVariableOperations(t *testing.T) {
	// Create a new FeaturesContext for testing
	f := newFeaturesContext()
	
	// Set up a test variable
	f.wildcardProcessor.SetVariable("test_text", "hello world")
	
	// Test equals operation
	err := f.checkVariable("test_text", "equals", "hello world")
	assert.NoError(t, err, "equals operation should pass")
	
	// Test that equals fails when it should
	err = f.checkVariable("test_text", "equals", "goodbye world")
	assert.Error(t, err, "equals operation should fail with wrong value")
	assert.Contains(t, err.Error(), "does not match expected value")
	
	// Test not equals operation
	err = f.checkVariable("test_text", "not equals", "goodbye world")
	assert.NoError(t, err, "not equals operation should pass")
	
	// Test that not equals fails when it should
	err = f.checkVariable("test_text", "not equals", "hello world")
	assert.Error(t, err, "not equals operation should fail with same value")
	assert.Contains(t, err.Error(), "does match expected value")
	
	// Test contains operation
	err = f.checkVariable("test_text", "contains", "hello")
	assert.NoError(t, err, "contains operation should pass")
	
	// Test that contains fails when it should
	err = f.checkVariable("test_text", "contains", "goodbye")
	assert.Error(t, err, "contains operation should fail when substring is not present")
	assert.Contains(t, err.Error(), "does not contain expected value")
	
	// Test not contains operation
	err = f.checkVariable("test_text", "not contains", "goodbye")
	assert.NoError(t, err, "not contains operation should pass")
	
	// Test that not contains fails when it should
	err = f.checkVariable("test_text", "not contains", "hello")
	assert.Error(t, err, "not contains operation should fail when substring is present")
	assert.Contains(t, err.Error(), "should not contain expected value")
}
