package cmds

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/heketi/heketi/client/cli/go/cmds"
)

var (
	diff                    int
	HEKETI_CLI_TEST_VERSION           = "testing"
	sout                    io.Writer = os.Stdout
	serr                    io.Writer = os.Stderr
)
var test_version = []struct {
	input  []string
	output string
}{
	{[]string{"--version", "-v"}, ""},
	{[]string{"--veri"}, "unknown flag: --veri"},
}

var testNoFlag = []struct {
	input []string
}{
	{[]string{}},
}

func TestVersion(t *testing.T) {
	c := cmds.NewHeketiCli(HEKETI_CLI_TEST_VERSION, sout, serr)

	buf := new(bytes.Buffer)
	for _, test_arg := range test_version {
		c.SetArgs(test_arg.input)
		c.SetOutput(buf)
		output := c.Execute()
		if output != nil {
			diff = strings.Compare(output.Error(), test_arg.output)
			if diff != 0 {
				t.Error("Expected ", test_arg.output, ",Got ", output.Error())
			}
		}
	}
	c.ResetFlags()
}

func TestClusterCreate(t *testing.T) {
	c := cmds.ClusterCreateCommand
	c.Root().ResetFlags()
	buf := new(bytes.Buffer)
	for _, test_clu := range testNoFlag {
		c.SetOutput(buf)
		output := c.RunE(c, test_clu.input)
		if output != nil {
			t.Error("Expected Nothing, Got ", output.Error())
		}
	}
}

func TestClusterList(t *testing.T) {
	c := cmds.ClusterListCommand
	c.Root().ResetFlags()
	buf := new(bytes.Buffer)
	for _, test_clu := range testNoFlag {
		c.SetOutput(buf)
		output := c.RunE(c, test_clu.input)
		if output != nil {
			t.Error("Expected Nothing, Got ", output.Error())
		}
	}
}

func TestVolumeList(t *testing.T) {
	c := cmds.VolumeListCommand
	c.Root().ResetFlags()
	buf := new(bytes.Buffer)
	for _, test_clu := range testNoFlag {
		c.SetOutput(buf)
		output := c.RunE(c, test_clu.input)
		if output != nil {
			t.Error("Expected Nothing, Got ", output.Error())
		}
	}
}
