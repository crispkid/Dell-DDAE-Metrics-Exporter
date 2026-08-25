package observability

// FailureFields returns the only error-derived attributes allowed at log
// boundaries. It intentionally excludes err.Error(), URLs and response data.
func FailureFields(component string, err error) []any {
	return []any{"component", component, "failure_class", Classify(err)}
}
