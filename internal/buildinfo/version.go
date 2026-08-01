package buildinfo

// Build-stamped identity, set via -ldflags -X at build time.
var (
	// Version is the release version, e.g. "v1.49.0". Defaults to "dev".
	Version = "dev"
	// GitCommit is the short commit hash the binary was built from.
	GitCommit = "unknown"
	// BuildDate is the RFC3339 timestamp of the build.
	BuildDate = "unknown"
)
