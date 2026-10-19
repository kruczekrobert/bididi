package bdd

import "testing"

func NewFeaturesContext(_ *testing.T) FeaturesContext {
	return *newFeaturesContext()
}
