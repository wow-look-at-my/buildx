// cobrautil.go holds the cobrautil code.

package cobrautil

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// HideInheritedFlags hides inherited flags
func HideInheritedFlags(cmd *cobra.Command, hidden ...string) {
	for _, h := range hidden {
		// we can use cmd.SetHelpFunc to override the helper but, it is not enough.
		cmd.Flags().String(h, "", "")
		_ = cmd.Flags().MarkHidden(h)
	}
}

const annotationExperimentalCLI = "experimentalCLI"

func MarkFlagExperimental(f *pflag.Flag) {
	if _, ok := f.Annotations[annotationExperimentalCLI]; ok {
		return
	}
	if f.Annotations == nil {
		f.Annotations = make(map[string][]string)
	}
	f.Annotations[annotationExperimentalCLI] = nil
	f.Usage += " (EXPERIMENTAL)"
}

func MarkFlagsExperimental(fs *pflag.FlagSet, names ...string) {
	for _, name := range names {
		f := fs.Lookup(name)
		if f == nil {
			logrus.Warningf("Unknown flag name %q", name)
			continue
		}
		MarkFlagExperimental(f)
	}
}

func MarkCommandExperimental(c *cobra.Command) {
	if _, ok := c.Annotations[annotationExperimentalCLI]; ok {
		return
	}
	if c.Annotations == nil {
		c.Annotations = make(map[string]string)
	}
	c.Annotations[annotationExperimentalCLI] = ""
	c.Short += " (EXPERIMENTAL)"
}
