package metrics

import (
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

type testApp struct {
	topologyInfo *api.TopologyInfoResponse
}

func (t *testApp) SetRoutes(router *mux.Router) error {
	return nil
}

func (t *testApp) TopologyInfo() (*api.TopologyInfoResponse, error) {
	return t.topologyInfo, nil
}

func (t *testApp) Close() {}

func (t *testApp) Auth(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
}

func TestMetricsEndpoint(t *testing.T) {
	ta := &testApp{
		topologyInfo: &api.TopologyInfoResponse{
			ClusterList: []api.Cluster{
				{
					Id: "t1",
				},
			},
		},
	}

	ts := httptest.NewServer(NewMetricsHandler(ta))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	match, err := regexp.Match("gluster_nodes_count{cluster=\"t1\"} 0", body)
	if !match || err != nil {
		t.Fatal("gluster_nodes_count{cluster=\"t1\"} 0 should be present in the metrics output")
	}
}
