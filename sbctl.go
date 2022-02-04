package main

import (
	"github.com/replicatedhq/sbctl/cli"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	cli.InitAndExecute()
}
