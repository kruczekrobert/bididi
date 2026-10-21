package bdd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cucumber/godog"
)

type httpStepContextKey struct{}

// Godog binds only supplied arguments, so optional arguments come from the current step.
func httpStepArguments(ctx context.Context) (*godog.Table, *godog.DocString) {
	step, _ := ctx.Value(httpStepContextKey{}).(*godog.Step)
	if step == nil || step.Argument == nil {
		return nil, nil
	}
	return step.Argument.DataTable, step.Argument.DocString
}

type httpTableRow struct {
	typeName string
	name     string
	value    string
}

func parseHTTPTable(table *godog.Table, allowed map[string]bool) ([]httpTableRow, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, nil
	}
	columns := table.Rows[0].Cells
	if len(columns) != 3 || columns[0].Value != "type" || columns[1].Value != "name" || columns[2].Value != "value" {
		return nil, fmt.Errorf("HTTP table columns must be type, name, value")
	}

	rows := make([]httpTableRow, 0, len(table.Rows)-1)
	for i, row := range table.Rows[1:] {
		if len(row.Cells) != 3 {
			return nil, fmt.Errorf("HTTP table row %d must have three cells", i+2)
		}
		entry := httpTableRow{row.Cells[0].Value, row.Cells[1].Value, row.Cells[2].Value}
		if !allowed[entry.typeName] {
			return nil, fmt.Errorf("HTTP table row %d has unsupported type %q", i+2, entry.typeName)
		}
		if (entry.typeName == "header" || entry.typeName == "response-header" || entry.typeName == "query") && entry.name == "" {
			return nil, fmt.Errorf("HTTP table row %d requires a name", i+2)
		}
		if entry.typeName != "header" && entry.typeName != "response-header" && entry.typeName != "query" && entry.name != "" {
			return nil, fmt.Errorf("HTTP table row %d must have an empty name", i+2)
		}
		rows = append(rows, entry)
	}
	return rows, nil
}

func (f *FeaturesContext) resolveHTTPText(value string) (string, error) {
	resolved, err := f.wildcardProcessor.SubstituteWildcards(value)
	if err != nil {
		return "", err
	}
	if resolved == nil {
		return "", fmt.Errorf("HTTP table value %q resolved to null", value)
	}
	return fmt.Sprint(resolved), nil
}

func (f *FeaturesContext) compareHTTPText(expected, actual string) error {
	resolved, err := f.wildcardProcessor.ResolveWildcards(expected)
	if err != nil {
		return err
	}
	if resolved == nil {
		return fmt.Errorf("HTTP table expectation %q resolved to null", expected)
	}
	return f.jsonComparator.compareValues(fmt.Sprint(resolved), actual, "")
}

func (f *FeaturesContext) compareHTTPStatus(expected string, actual int) error {
	resolved, err := f.wildcardProcessor.ResolveWildcards(expected)
	if err != nil {
		return err
	}
	value := fmt.Sprint(resolved)
	if status, err := strconv.Atoi(value); err == nil {
		if status != actual {
			return fmt.Errorf("expected response status %d, got %d", status, actual)
		}
		return nil
	}
	return f.jsonComparator.compareValues(value, float64(actual), "")
}
