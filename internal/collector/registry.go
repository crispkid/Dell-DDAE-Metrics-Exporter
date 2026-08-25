package collector

var requiredCollectorNames = [...]string{"ping", "clusters", "nodes", "lock", "power"}

func RequiredCollectors() []string {
	result := make([]string, len(requiredCollectorNames))
	copy(result, requiredCollectorNames[:])
	return result
}
