package cmds

import (
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/client/api/go-client"
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
	db                      string
	app                     *glusterfs.App
	Ts                      *httptest.Server
	Url                     string
)

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
	db := tests.Tempfile()
	//	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	//	defer app.Close()

	// Setup the server
	Ts := setupHeketiServer(app)
	Url = Ts.URL
	//	defer ts.Close()
	//	server := ts.URL
	//  defaultflags :=
	var test_version = []struct {
		input []string
		err   string
	}{
		{[]string{"--version", "--server", Ts.URL, "--user", "admin", "--secret",
			TEST_ADMIN_KEY, "-v"}, ""},
		{[]string{"--veri"}, "unknown flag: --veri"},
	}
	for _, test_arg := range test_version {
		c.SetArgs(test_arg.input)
		err := c.Execute()
		if err != nil {
			diff = strings.Compare(err.Error(), test_arg.err)
			if diff != 0 {
				t.Error("Expected ", test_arg.err, ",Got ", err.Error())
			}
		}
	}
	c.ResetFlags()
}

func TestClusterCreate(t *testing.T) {
	c := cmds.ClusterCreateCommand
	err := c.RunE(c, nil)
	if err != nil {
		t.Error("Expected Nothing, Got ", err.Error())
	}
}

func TestClusterList(t *testing.T) {
	c := cmds.ClusterListCommand
	err := c.RunE(c, nil)
	if err != nil {
		t.Error("Expected Nothing, Got ", err.Error())

	}
}

func TestClusterDelete(t *testing.T) {
	heketi := client.NewClient(Url, "admin", TEST_ADMIN_KEY)
	cluster, _ := heketi.ClusterCreate()
	clusterid := cluster.Id
	var testCluDel = []struct {
		input []string
		err   string
	}{
		{[]string{"badid"}, "404 page not found"},
		{nil, "Cluster id missing"},
		{[]string{clusterid}, ""},
		{[]string{clusterid}, "Id not found"},
	}
	c := cmds.ClusterDeleteCommand
	for _, test_clu := range testCluDel {
		err := c.RunE(c, test_clu.input)
		if err != nil {
			if !strings.Contains(err.Error(), test_clu.err) {
				t.Error("Expected " + test_clu.err + ", Got" + err.Error())
			}
		} else if test_clu.err != "" {
			t.Error("Expected " + test_clu.err + ", Got Nothing")
		}
	}
}

func TestClusterInfo(t *testing.T) {
	heketi := client.NewClient(Url, "admin", TEST_ADMIN_KEY)
	cluster, _ := heketi.ClusterCreate()
	clusterid := cluster.Id
	cluster_d, _ := heketi.ClusterCreate()
	clusterid_d := cluster_d.Id
	heketi.ClusterDelete(clusterid_d)
	var testCluInfo = []struct {
		input []string
		err   string
	}{
		{[]string{"badid"}, "404 page not found"},
		{nil, "Cluster id missing"},
		{[]string{clusterid}, ""},
		{[]string{clusterid_d}, "Id not found"},
	}
	c := cmds.ClusterInfoCommand
	for _, test_clu := range testCluInfo {
		err := c.RunE(c, test_clu.input)
		if err != nil {
			diff := strings.Contains(err.Error(), test_clu.err)
			if diff != true {
				t.Error("Expected "+test_clu.err+", Got", err.Error())
			}
		} else if test_clu.err != "" {
			t.Error("Expected " + test_clu.err + ", Got Nothing")
		}
	}
}

func TestVolumeList(t *testing.T) {
	c := cmds.VolumeListCommand
	err := c.RunE(c, nil)
	if err != nil {
		t.Error("Expected Nothing, Got ", err.Error())

	}
}
