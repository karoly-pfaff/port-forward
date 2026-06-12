package version

// Version is the current Portier CLI version.
// Build scripts inject this at compile time:
//
//	go build -ldflags "-X portier/cli/sources/version.Version=1.4.0" ...
var Version = "1.8.0"
