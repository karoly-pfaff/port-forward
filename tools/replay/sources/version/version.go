package version

// Version is the current Portier replay tool version. It tracks the overall
// Portier release version (bumped alongside the other version surfaces).
// Build scripts may inject this at compile time:
//
//	go build -ldflags "-X portier/replay/sources/version.Version=1.13.0" ...
var Version = "1.17.0"
