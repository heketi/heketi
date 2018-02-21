package glusterfs

import (
	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

const (
	namespace = "gluster"
)

var (
	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Was the last query of heketi successful.",
		nil, nil,
	)
	clusterCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cluster_count"),
		"Number of clusters at last query.",
		nil, nil,
	)
	volumesCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volumes_count"),
		"How many volumes were up at the last query.",
		[]string{"cluster"}, nil,
	)
	nodesCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "nodes_count"),
		"How many Nodes were up at the last query.",
		[]string{"cluster"}, nil,
	)
	deviceCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "device_count"),
		"How many Devices were up at the last query.",
		[]string{"cluster", "hostname"}, nil,
	)
	deviceSize = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "device_size"),
		"How many Devices were up at the last query.",
		[]string{"cluster", "hostname", "device"}, nil,
	)
	deviceFree = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "device_free"),
		"How many Devices were up at the last query.",
		[]string{"cluster", "hostname", "device"}, nil,
	)
	deviceUsed = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "device_used"),
		"How many Devices were up at the last query.",
		[]string{"cluster", "hostname", "device"}, nil,
	)
	brickCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "brick_count"),
		"Number of bricks at last query.",
		[]string{"cluster", "hostname", "device"}, nil,
	)
)

// Describe all the metrics exported by Heketi exporter. It implements prometheus.Collector.
func (a *App) Describe(ch chan<- *prometheus.Desc) {
	ch <- clusterCount // done
	ch <- volumesCount // done
	ch <- nodesCount   // done
	ch <- deviceCount  // done
	ch <- deviceSize   // done
	ch <- deviceFree   // done
	ch <- deviceUsed   // done
	ch <- brickCount   // done

}

func (a *App) clusterList() (*api.ClusterListResponse, error) {
	list := &api.ClusterListResponse{}

	// Get all the cluster ids from the DB
	err := a.db.View(func(tx *bolt.Tx) error {
		var err error

		list.Clusters, err = ClusterList(tx)
		if err != nil {
			return err
		}

		return nil
	})

	return list, err
}

func (a *App) clusterInfo(id string) (*api.ClusterInfoResponse, error) {
	// Get info from db
	var info *api.ClusterInfoResponse
	err := a.db.View(func(tx *bolt.Tx) error {

		// Create a db entry from the id
		entry, err := NewClusterEntryFromId(tx, id)
		if err == ErrNotFound {
			return err
		} else if err != nil {
			return err
		}

		// Create a response from the db entry
		info, err = entry.NewClusterInfoResponse(tx)
		if err != nil {
			return err
		}
		err = UpdateClusterInfoComplete(tx, info)
		if err != nil {
			return err
		}

		return nil
	})

	return info, err
}

func (a *App) volumeInfo(id string) (*api.VolumeInfoResponse, error) {
	var info *api.VolumeInfoResponse
	err := a.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, id)
		if err == ErrNotFound || !entry.Visible() {
			return ErrNotFound
		} else if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil
	})

	return info, err
}

func (a *App) nodeInfo(id string) (*api.NodeInfoResponse, error) {
	// Get Node information
	var info *api.NodeInfoResponse
	err := a.db.View(func(tx *bolt.Tx) error {
		entry, err := NewNodeEntryFromId(tx, id)
		if err == ErrNotFound {
			return err
		} else if err != nil {
			return err
		}

		info, err = entry.NewInfoReponse(tx)
		if err != nil {
			return err
		}

		return nil
	})

	return info, err
}

func (a *App) topologyInfo() (*api.TopologyInfoResponse, error) {
	topo := &api.TopologyInfoResponse{
		ClusterList: make([]api.Cluster, 0),
	}
	clusterlist, err := a.clusterList()
	if err != nil {
		return nil, err
	}
	for _, cluster := range clusterlist.Clusters {
		clusteri, err := a.clusterInfo(cluster)
		if err != nil {
			return nil, err
		}
		cluster := api.Cluster{
			Id:      clusteri.Id,
			Volumes: make([]api.VolumeInfoResponse, 0),
			Nodes:   make([]api.NodeInfoResponse, 0),
			ClusterFlags: api.ClusterFlags{
				Block: clusteri.Block,
				File:  clusteri.File,
			},
		}
		cluster.Id = clusteri.Id

		// Iterate over the volume list in the cluster
		for _, volumes := range clusteri.Volumes {
			volumesi, err := a.volumeInfo(volumes)
			if err != nil {
				return nil, err
			}
			if volumesi.Cluster == cluster.Id {
				cluster.Volumes = append(cluster.Volumes, *volumesi)
			}
		}

		// Iterate over the nodes in the cluster
		for _, node := range clusteri.Nodes {
			nodei, err := a.nodeInfo(string(node))
			if err != nil {
				return nil, err
			}
			cluster.Nodes = append(cluster.Nodes, *nodei)
		}
		topo.ClusterList = append(topo.ClusterList, cluster)
	}
	return topo, nil

}

// Collect collects all the metrics
func (a *App) Collect(ch chan<- prometheus.Metric) {
	// Collect metrics from volume info
	topinfo, err := a.topologyInfo()
	// Couldn't parse xml, so something is really wrong and up=0
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0.0,
		)
	} else {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 1.0,
		)
	}
	ch <- prometheus.MustNewConstMetric(
		clusterCount, prometheus.GaugeValue, float64(len(topinfo.ClusterList)),
	)
	for _, cluster := range topinfo.ClusterList {
		ch <- prometheus.MustNewConstMetric(
			volumesCount, prometheus.GaugeValue, float64(len(cluster.Volumes)), cluster.Id,
		)
		ch <- prometheus.MustNewConstMetric(
			nodesCount, prometheus.GaugeValue, float64(len(cluster.Nodes)), cluster.Id,
		)
		//	for _, volumes := range cluster.Volumes {
		//			// Not Using for now
		//	}
		for _, nodes := range cluster.Nodes {
			ch <- prometheus.MustNewConstMetric(
				deviceCount, prometheus.GaugeValue, float64(len(nodes.DevicesInfo)), cluster.Id, nodes.Hostnames.Manage[0],
			)
			for _, device := range nodes.DevicesInfo {
				ch <- prometheus.MustNewConstMetric(
					deviceSize, prometheus.GaugeValue, float64(device.Storage.Total), cluster.Id, nodes.Hostnames.Manage[0], device.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					deviceFree, prometheus.GaugeValue, float64(device.Storage.Free), cluster.Id, nodes.Hostnames.Manage[0], device.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					deviceUsed, prometheus.GaugeValue, float64(device.Storage.Used), cluster.Id, nodes.Hostnames.Manage[0], device.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					brickCount, prometheus.GaugeValue, float64(len(device.Bricks)), cluster.Id, nodes.Hostnames.Manage[0], device.Name,
				)
			}
		}
	}
}

func (a *App) Metrics() http.HandlerFunc {
	prometheus.MustRegister(a)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(w, r)
	})
}
