package version

// Version is set via ldflags at build time:
// go build -ldflags "-X github.com/teamcutter/chatr/internal/version.Version=v0.1.0"
var Version = "dev"
