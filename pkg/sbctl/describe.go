package sbctl

import (
	"os"

	"github.com/pkg/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	describecli "k8s.io/kubectl/pkg/cmd/describe"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"
)

func Describe(f cmdutil.Factory, args []string) error {
	o := &describecli.DescribeOptions{
		FilenameOptions: &resource.FilenameOptions{},
		DescriberSettings: &describe.DescriberSettings{
			ShowEvents: true,
			ChunkSize:  cmdutil.DefaultChunkSize,
		},
		BuilderArgs: args,
		NewBuilder:  f.NewBuilder,

		CmdParent: "kubectl",
		IOStreams: genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	}
	err := o.Run()
	if err != nil {
		return errors.Wrap(err, "failed to run describer")
	}
	return nil
}
