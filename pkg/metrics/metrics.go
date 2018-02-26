package metrics

import (
	"github.com/heketi/heketi/apps"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

type Metrics struct {
	app apps.Application
}

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
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- clusterCount // done
	ch <- volumesCount // done
	ch <- nodesCount   // done
	ch <- deviceCount  // done
	ch <- deviceSize   // done
	ch <- deviceFree   // done
	ch <- deviceUsed   // done
	ch <- brickCount   // done

}

// Collect metrics from heketi app
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	topinfo, err := m.app.TopologyInfo()
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

func NewMetricsHandler(app apps.Application) http.HandlerFunc {
	m := &Metrics{
		app: app,
	}
	prometheus.MustRegister(m)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(w, r)
	})
}
