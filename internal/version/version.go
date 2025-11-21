package version

// Version is the current version of Terraform Mirror
const Version = "1.0.0-alpha"

// BuildTime is set during build via -ldflags
var BuildTime string

// GitCommit is set during build via -ldflags
var GitCommit string
