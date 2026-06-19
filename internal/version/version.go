package version

// Base and BuildMeta are set at build time via ldflags.
var (
	Base      = "dev"
	BuildMeta = ""
)

// String returns the full version string.
// Tagged builds: "0.1.0", untagged: "0.1.0+a3f2b1c".
func String() string {
	if BuildMeta != "" {
		return Base + "+" + BuildMeta
	}
	return Base
}
