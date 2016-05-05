package cmds

import (
	"bytes"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/client/cli/go/cmds"
	"github.com/heketi/heketi/middleware"
	"github.com/heketi/tests"
)

var (
	diff                    int
	HEKETI_CLI_TEST_VERSION           = "testing"
	sout                    io.Writer = os.Stdout
	serr                    io.Writer = os.Stderr
	TEST_ADMIN_KEY                    = "adminkey"
)
var test_version = []struct {
	input  []string
	output string
}{
	{[]string{"--version", "--server", "http://localhost:8080", "-v"}, ""},
	{[]string{"--veri"}, "unknown flag: --veri"},
}

var testNoFlag = []struct {
	input []string
}{
	{[]string{}},
}

func setupHeketiServer(app *glusterfs.App) *httptest.Server {
	router := mux.NewRouter()
	app.SetRoutes(router)
	n := negroni.New()

	jwtconfig := &middleware.JwtAuthConfig{}
	jwtconfig.Admin.PrivateKey = TEST_ADMIN_KEY
	jwtconfig.User.PrivateKey = "userkey"

	// Setup middleware
	n.Use(middleware.NewJwtAuth(jwtconfig))
	n.UseFunc(app.Auth)
	n.UseHandler(router)

	// Create server
	return httptest.NewServer(n)
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
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()
	//	server := ts.URL
	//  defaultflags :=
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

func TestClusterDelete(t *testing.T) {
	//heketi := client.NewClient(options.Url, options.User, options.Key)
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
