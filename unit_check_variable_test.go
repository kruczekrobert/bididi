package bdd

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckVariable(t *testing.T) {
	f := NewFeaturesContext(t)

	f.wildcardProcessor.SetVariable("test_string", "hello world")
	f.wildcardProcessor.SetVariable("test_integer", 42)
	f.wildcardProcessor.SetVariable("test_float", 3.14)
	f.wildcardProcessor.SetVariable("test_boolean", true)

	err := f.checkVariable("test_string", "equals", "hello world")
	assert.NoError(t, err)

	err = f.checkVariable("test_integer", "equals", "42")
	assert.NoError(t, err)

	err = f.checkVariable("test_float", "equals", "3.14")
	assert.NoError(t, err)

	err = f.checkVariable("test_boolean", "equals", "true")
	assert.NoError(t, err)

	err = f.checkVariable("test_string", "not equals", "goodbye world")
	assert.NoError(t, err)

	err = f.checkVariable("test_string", "not equals", "hello world")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does match expected value")

	err = f.checkVariable("test_string", "equals", "goodbye world")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match expected value")

	f.wildcardProcessor.SetVariable("empty", nil)
	assert.NoError(t, f.checkVariable("empty", "equals", "$NULL$"))
	assert.Error(t, f.checkVariable("empty", "not equals", "$NULL$"))
	assert.NoError(t, f.checkVariable("empty", "not equals", "hello"))
	assert.NoError(t, f.checkVariable("test_string", "not equals", "$NULL$"))
	f.wildcardProcessor.SetVariable("numeric_name", 42)
	assert.Error(t, f.checkVariable("$VAR(numeric_name)$", "equals", "42"))
}
