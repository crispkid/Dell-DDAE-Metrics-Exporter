package collector

import "github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"

type validationError string

func (e validationError) Error() string                     { return string(e) + " validation failed" }
func (e validationError) FailureClass() observability.Class { return observability.ClassValidation }
