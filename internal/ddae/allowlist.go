package ddae

import (
	"fmt"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
)

const (
	pingSuffix                    = "/ping"
	clustersSuffix                = "/ddae-clusters"
	nodesSuffix                   = "/infrastructure-nodes"
	lockSuffix                    = "/system-lock"
	powerSuffix                   = "/system-shutdown"
	alertListSuffix               = "/serviceability-issues"
	alertDetailSuffix             = "/serviceability-issues/"
	serviceabilityLogListSuffix   = "/serviceability-events"
	serviceabilityLogDetailSuffix = "/serviceability-events/"
	tokenPath                     = "/auth/realms/ddae/protocol/openid-connect/token"

	// Default paths remain available to package tests and compatibility
	// assertions. Runtime clients use their immutable configured route set.
	pingPath                    = config.DefaultDDAEPingPathPrefix + pingSuffix
	clustersPath                = config.DefaultDDAEAPIPathPrefix + clustersSuffix
	nodesPath                   = config.DefaultDDAEAPIPathPrefix + nodesSuffix
	lockPath                    = config.DefaultDDAEAPIPathPrefix + lockSuffix
	powerPath                   = config.DefaultDDAEAPIPathPrefix + powerSuffix
	alertListPath               = config.DefaultDDAEAPIPathPrefix + alertListSuffix
	alertDetailPath             = config.DefaultDDAEAPIPathPrefix + alertDetailSuffix
	serviceabilityLogListPath   = config.DefaultDDAEAPIPathPrefix + serviceabilityLogListSuffix
	serviceabilityLogDetailPath = config.DefaultDDAEAPIPathPrefix + serviceabilityLogDetailSuffix
)

type Operation struct {
	Collector string
	Method    string
	Path      string
}

type routeSet struct {
	ping                    string
	clusters                string
	nodes                   string
	lock                    string
	power                   string
	alertList               string
	alertDetail             string
	serviceabilityLogList   string
	serviceabilityLogDetail string
}

func routeSetForPrefixes(pingPrefix, apiPrefix string) (routeSet, error) {
	if err := config.ValidateDDAEPathPrefix(pingPrefix); err != nil {
		return routeSet{}, fmt.Errorf("DDAE Ping path prefix is invalid: %w", err)
	}
	if err := config.ValidateDDAEPathPrefix(apiPrefix); err != nil {
		return routeSet{}, fmt.Errorf("DDAE API path prefix is invalid: %w", err)
	}
	return routeSet{
		ping:                    pingPrefix + pingSuffix,
		clusters:                apiPrefix + clustersSuffix,
		nodes:                   apiPrefix + nodesSuffix,
		lock:                    apiPrefix + lockSuffix,
		power:                   apiPrefix + powerSuffix,
		alertList:               apiPrefix + alertListSuffix,
		alertDetail:             apiPrefix + alertDetailSuffix,
		serviceabilityLogList:   apiPrefix + serviceabilityLogListSuffix,
		serviceabilityLogDetail: apiPrefix + serviceabilityLogDetailSuffix,
	}, nil
}

func (routes routeSet) operations() []Operation {
	return []Operation{
		{Collector: "ping", Method: "GET", Path: routes.ping},
		{Collector: "clusters", Method: "GET", Path: routes.clusters},
		{Collector: "nodes", Method: "GET", Path: routes.nodes},
		{Collector: "lock", Method: "GET", Path: routes.lock},
		{Collector: "power", Method: "GET", Path: routes.power},
		{Collector: "alert_list", Method: "GET", Path: routes.alertList},
		{Collector: "alert_detail", Method: "GET", Path: routes.alertDetail + "{id}"},
		{Collector: "serviceability_log_list", Method: "GET", Path: routes.serviceabilityLogList},
		{Collector: "serviceability_log_detail", Method: "GET", Path: routes.serviceabilityLogDetail + "{id}"},
	}
}

// ApprovedOperations returns a fresh copy of the default compiled GET
// allowlist. Runtime-specific policy tests can use ApprovedOperationsForPrefixes.
func ApprovedOperations() []Operation {
	operations, err := ApprovedOperationsForPrefixes(
		config.DefaultDDAEPingPathPrefix,
		config.DefaultDDAEAPIPathPrefix,
	)
	if err != nil {
		panic("invalid compiled DDAE path-prefix defaults")
	}
	return operations
}

// ApprovedOperationsForPrefixes returns the exact immutable operation paths
// that a client using the validated prefixes will send.
func ApprovedOperationsForPrefixes(pingPrefix, apiPrefix string) ([]Operation, error) {
	routes, err := routeSetForPrefixes(pingPrefix, apiPrefix)
	if err != nil {
		return nil, err
	}
	return routes.operations(), nil
}
