package exampleapp_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/cucumber/godog"
	"github.com/kruczekrobert/bididi"
	exampleapp "github.com/kruczekrobert/bididi/examples/example-app"
	"go.uber.org/fx"
)

func TestFeature(t *testing.T) {
	features := bdd.New()
	defer features.Close()

	var handler http.Handler
	app := fx.New(exampleapp.Module, fx.Populate(&handler))
	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	features.RegisterApp(handler)

	suite := godog.TestSuite{
		Name:                "example-app",
		ScenarioInitializer: features.InitializeScenario,
		Options: &godog.Options{
			Format:   "progress",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("BDD scenarios failed")
	}
}
