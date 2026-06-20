package version

// Version is set at build time via ldflags. Defaults to "dev" for
// plain `go build` without flags.
var Base = "dev"

// String returns the version string.
func String() string {
	return Base
}
