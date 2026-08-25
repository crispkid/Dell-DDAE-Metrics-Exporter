package ddae

const (
	pingPath        = "/rest/v1/ping"
	clustersPath    = "/rest/v1/ddae-clusters"
	nodesPath       = "/rest/v1/infrastructure-nodes"
	lockPath        = "/rest/v1/system-lock"
	powerPath       = "/rest/v1/system-shutdown"
	alertListPath   = "/rest/v1/serviceability-issues"
	alertDetailPath = "/rest/v1/serviceability-issues/"
	tokenPath       = "/auth/realms/ddae/protocol/openid-connect/token"
)

type Operation struct {
	Collector string
	Method    string
	Path      string
}

var approvedOperations = [...]Operation{
	{Collector: "ping", Method: "GET", Path: pingPath},
	{Collector: "clusters", Method: "GET", Path: clustersPath},
	{Collector: "nodes", Method: "GET", Path: nodesPath},
	{Collector: "lock", Method: "GET", Path: lockPath},
	{Collector: "power", Method: "GET", Path: powerPath},
	{Collector: "alert_list", Method: "GET", Path: alertListPath},
	{Collector: "alert_detail", Method: "GET", Path: alertDetailPath + "{id}"},
}

func ApprovedOperations() []Operation {
	result := make([]Operation, len(approvedOperations))
	copy(result, approvedOperations[:])
	return result
}
