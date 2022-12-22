package main

import (
	"github.com/replicatedhq/sbctl/cli"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.InitAndExecute(version)
}
