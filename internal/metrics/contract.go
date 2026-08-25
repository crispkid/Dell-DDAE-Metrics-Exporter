package metrics

import "github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"

var nodeStateValues = []string{
	"maintenance_mode", "scheduling_disabled", "not_ready", "ready",
	"restarting", "shutting_down", "powered_off", "powering_on", "unknown",
}

var failureClasses = []observability.Class{
	observability.ClassAuth, observability.ClassTLS, observability.ClassTimeout,
	observability.ClassTransport, observability.ClassHTTP, observability.ClassDecode,
	observability.ClassValidation, observability.ClassKafkaAuth,
	observability.ClassKafkaTimeout, observability.ClassKafkaRejected,
	observability.ClassBufferFull, observability.ClassInternal,
}
