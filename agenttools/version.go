package agenttools

import "runtime/debug"

// modulePath is the Go module of this package.
const modulePath = "github.com/boring-design/elastic-fruit-cli"

// version is set at release build time with
// -ldflags "-X github.com/boring-design/elastic-fruit-cli/agenttools.version=<version>".
var version = ""

// Version returns the version of the efc tools: the release version, the
// module version when another program builds them in, or "dev".
func Version() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if info.Main.Path == modulePath && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	for _, dependency := range info.Deps {
		if dependency.Path == modulePath {
			return dependency.Version
		}
	}
	return "dev"
}
