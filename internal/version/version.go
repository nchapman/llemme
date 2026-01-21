// Package version provides version information for lleme.
package version

// Version is the application version, set via ldflags at build time.
var Version = "dev"

// UserAgent returns the User-Agent string for HTTP requests.
func UserAgent() string {
	return "lleme/" + Version
}
