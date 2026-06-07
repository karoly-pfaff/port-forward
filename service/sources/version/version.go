package version

// Version is the current Portier service version.
// Build scripts inject this at compile time:
//
//	go build -ldflags "-X portier/service/sources/version.Version=1.1.0" ...
var Version = "dev"
