package version

import "runtime/debug"

var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

func String() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
