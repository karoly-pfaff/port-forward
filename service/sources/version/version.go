package version

// Version is the current Portier service version.
// This source default is bumped per release (kept in lockstep with the CLI
// version package and the package.json version). Build scripts may also inject
// it at compile time:
//
//	go build -ldflags "-X portier/service/sources/version.Version=1.7.0" ...
var Version = "1.11.0"
