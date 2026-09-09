package config

// LoadIsolated uses the normal validators without reading process environment.
// Diagnostic callers supply only their separately validated field allowlist.
func LoadIsolated(values map[string]string) (Config, error) {
	return load(func(name string) (string, bool) { v, ok := values[name]; return v, ok }, readSecretFile)
}
