/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"flag"
	"os"
	"time"

	"github.com/onsi/ginkgo/config"
	"github.com/spf13/viper"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

type TestContextType struct {
	KubeConfig         string
	KubeContext        string
	KubeAPIContentType string
	KubeVolumeDir      string
	CertDir            string
	Host               string
	// TODO: Deprecating this over time... instead just use gobindata_util.go , see #23987.
	RepoRoot string

	Provider       string
	CloudConfig    CloudConfig
	KubectlPath    string
	OutputDir      string
	ReportDir      string
	ReportPrefix   string
	Prefix         string
	MinStartupPods int
	// Timeout for waiting for system pods to be running
	SystemPodsStartupTimeout time.Duration
	UpgradeTarget            string
	UpgradeImage             string
	PrometheusPushGateway    string
	ContainerRuntime         string
	MasterOSDistro           string
	NodeOSDistro             string
	VerifyServiceAccount     bool
	DeleteNamespace          bool
	DeleteNamespaceOnFailure bool
	AllowedNotReadyNodes     int
	CleanStart               bool
	// If set to 'true' or 'all' framework will start a goroutine monitoring resource usage of system add-ons.
	// It will read the data every 30 seconds from all Nodes and print summary during afterEach. If set to 'master'
	// only master Node will be monitored.
	GatherKubeSystemResourceUsageData string
	GatherLogsSizes                   bool
	GatherMetricsAfterTest            bool
	// Currently supported values are 'hr' for human-readable and 'json'. It's a comma separated list.
	OutputPrintType string
	// CreateTestingNS is responsible for creating namespace used for executing e2e tests.
	// It accepts namespace base name, which will be prepended with e2e prefix, kube client
	// and labels to be applied to a namespace.
	CreateTestingNS CreateTestingNSFn
	// If set to true test will dump data about the namespace in which test was running.
	DumpLogsOnFailure bool
	// If the garbage collector is enabled in the kube-apiserver and kube-controller-manager.
	GarbageCollectorEnabled bool
	// FeatureGates is a set of key=value pairs that describe feature gates for alpha/experimental features.
	FeatureGates string
	// Node e2e specific test context
	NodeTestContextType

	// Viper-only parameters.  These will in time replace all flags.

	// Example: Create a file 'e2e.json' with the following:
	// 	"Cadvisor":{
	// 		"MaxRetries":"6"
	// 	}

	Viper    string
	Cadvisor struct {
		MaxRetries      int
		SleepDurationMS int
	}

	LoggingSoak struct {
		Scale                    int
		MilliSecondsBetweenWaves int
	}
}

// NodeTestContextType is part of TestContextType, it is shared by all node e2e test.
type NodeTestContextType struct {
	// Name of the node to run tests on (node e2e suite only).
	NodeName string
	// DisableKubenet disables kubenet when starting kubelet.
	DisableKubenet bool
	// Whether to enable the QoS Cgroup Hierarchy or not
	CgroupsPerQOS bool
	// How the kubelet should interface with the cgroup hierarchy (cgroupfs or systemd)
	CgroupDriver string
	// The hard eviction thresholds
	EvictionHard string
	// ManifestPath is the static pod manifest path.
	ManifestPath string
	// PrepullImages indicates whether node e2e framework should prepull images.
	PrepullImages bool
	// RuntimeIntegrationType indicates how runtime is integrated with Kubelet. This is mainly used for CRI validation test.
	RuntimeIntegrationType string
	// ContainerRuntimeEndpoint is the endpoint of remote container runtime grpc server. This is mainly used for Remote CRI
	// validation test.
	ContainerRuntimeEndpoint string
	// MounterPath is the path to the program to run to perform a mount
	MounterPath string
	// MounterRootfsPath is the path to the root filesystem for the program used to perform a mount in kubelet
	MounterRootfsPath string
}

type CloudConfig struct {
	ProjectID         string
	Zone              string
	Cluster           string
	MasterName        string
	NodeInstanceGroup string
	NumNodes          int
	ClusterTag        string
	Network           string

	Provider cloudprovider.Interface
}

var TestContext TestContextType
var federatedKubeContext string

// Register flags common to all e2e test suites.
func RegisterCommonFlags() {
	// Turn on verbose by default to get spec names
	config.DefaultReporterConfig.Verbose = true

	// Turn on EmitSpecProgress to get spec progress (especially on interrupt)
	config.GinkgoConfig.EmitSpecProgress = true

	// Randomize specs as well as suites
	config.GinkgoConfig.RandomizeAllSpecs = true

	flag.StringVar(&TestContext.GatherKubeSystemResourceUsageData, "gather-resource-usage", "false", "If set to 'true' or 'all' framework will be monitoring resource usage of system all add-ons in (some) e2e tests, if set to 'master' framework will be monitoring master node only, if set to 'none' of 'false' monitoring will be turned off.")
	flag.BoolVar(&TestContext.GatherLogsSizes, "gather-logs-sizes", false, "If set to true framework will be monitoring logs sizes on all machines running e2e tests.")
	flag.BoolVar(&TestContext.GatherMetricsAfterTest, "gather-metrics-at-teardown", false, "If set to true framwork will gather metrics from all components after each test.")
	flag.StringVar(&TestContext.OutputPrintType, "output-print-type", "hr", "Comma separated list: 'hr' for human readable summaries 'json' for JSON ones.")
	flag.BoolVar(&TestContext.DumpLogsOnFailure, "dump-logs-on-failure", true, "If set to true test will dump data about the namespace in which test was running.")
	flag.BoolVar(&TestContext.DeleteNamespace, "delete-namespace", true, "If true tests will delete namespace after completion. It is only designed to make debugging easier, DO NOT turn it off by default.")
	flag.BoolVar(&TestContext.DeleteNamespaceOnFailure, "delete-namespace-on-failure", true, "If true, framework will delete test namespace on failure. Used only during test debugging.")
	flag.IntVar(&TestContext.AllowedNotReadyNodes, "allowed-not-ready-nodes", 0, "If non-zero, framework will allow for that many non-ready nodes when checking for all ready nodes.")
	flag.StringVar(&TestContext.Host, "host", "http://127.0.0.1:8080", "The host, or apiserver, to connect to")
	flag.StringVar(&TestContext.ReportPrefix, "report-prefix", "", "Optional prefix for JUnit XML reports. Default is empty, which doesn't prepend anything to the default name.")
	flag.StringVar(&TestContext.ReportDir, "report-dir", "", "Path to the directory where the JUnit XML reports should be saved. Default is empty, which doesn't generate these reports.")
	flag.StringVar(&TestContext.FeatureGates, "feature-gates", "", "A set of key=value pairs that describe feature gates for alpha/experimental features.")
	flag.StringVar(&TestContext.Viper, "viper-config", "e2e", "The name of the viper config i.e. 'e2e' will read values from 'e2e.json' locally.  All e2e parameters are meant to be configurable by viper.")
}

// Register flags specific to the cluster e2e test suite.
func RegisterClusterFlags() {
	flag.BoolVar(&TestContext.VerifyServiceAccount, "e2e-verify-service-account", true, "If true tests will verify the service account before running.")
	flag.StringVar(&TestContext.KubeConfig, clientcmd.RecommendedConfigPathFlag, os.Getenv(clientcmd.RecommendedConfigPathEnvVar), "Path to kubeconfig containing embedded authinfo.")
	flag.StringVar(&TestContext.KubeContext, clientcmd.FlagContext, "", "kubeconfig context to use/override. If unset, will use value from 'current-context'")
	flag.StringVar(&TestContext.KubeAPIContentType, "kube-api-content-type", "application/vnd.kubernetes.protobuf", "ContentType used to communicate with apiserver")
	flag.StringVar(&federatedKubeContext, "federated-kube-context", "federation-cluster", "kubeconfig context for federation-cluster.")

	flag.StringVar(&TestContext.KubeVolumeDir, "volume-dir", "/var/lib/kubelet", "Path to the directory containing the kubelet volumes.")
	flag.StringVar(&TestContext.CertDir, "cert-dir", "", "Path to the directory containing the certs. Default is empty, which doesn't use certs.")
	flag.StringVar(&TestContext.RepoRoot, "repo-root", "../../", "Root directory of kubernetes repository, for finding test files.")
	flag.StringVar(&TestContext.Provider, "provider", "", "The name of the Kubernetes provider (gce, gke, local, vagrant, etc.)")
	flag.StringVar(&TestContext.KubectlPath, "kubectl-path", "kubectl", "The kubectl binary to use. For development, you might use 'cluster/kubectl.sh' here.")
	flag.StringVar(&TestContext.OutputDir, "e2e-output-dir", "/tmp", "Output directory for interesting/useful test data, like performance data, benchmarks, and other metrics.")
	flag.StringVar(&TestContext.Prefix, "prefix", "e2e", "A prefix to be added to cloud resources created during testing.")
	flag.StringVar(&TestContext.ContainerRuntime, "container-runtime", "docker", "The container runtime of cluster VM instances (docker or rkt).")
	flag.StringVar(&TestContext.MasterOSDistro, "master-os-distro", "debian", "The OS distribution of cluster master (debian, trusty, or coreos).")
	flag.StringVar(&TestContext.NodeOSDistro, "node-os-distro", "debian", "The OS distribution of cluster VM instances (debian, trusty, or coreos).")

	// TODO: Flags per provider?  Rename gce-project/gce-zone?
	cloudConfig := &TestContext.CloudConfig
	flag.StringVar(&cloudConfig.MasterName, "kube-master", "", "Name of the kubernetes master. Only required if provider is gce or gke")
	flag.StringVar(&cloudConfig.ProjectID, "gce-project", "", "The GCE project being used, if applicable")
	flag.StringVar(&cloudConfig.Zone, "gce-zone", "", "GCE zone being used, if applicable")
	flag.StringVar(&cloudConfig.Cluster, "gke-cluster", "", "GKE name of cluster being used, if applicable")
	flag.StringVar(&cloudConfig.NodeInstanceGroup, "node-instance-group", "", "Name of the managed instance group for nodes. Valid only for gce, gke or aws. If there is more than one group: comma separated list of groups.")
	flag.StringVar(&cloudConfig.Network, "network", "e2e", "The cloud provider network for this e2e cluster.")
	flag.IntVar(&cloudConfig.NumNodes, "num-nodes", -1, "Number of nodes in the cluster")

	flag.StringVar(&cloudConfig.ClusterTag, "cluster-tag", "", "Tag used to identify resources.  Only required if provider is aws.")
	flag.IntVar(&TestContext.MinStartupPods, "minStartupPods", 0, "The number of pods which we need to see in 'Running' state with a 'Ready' condition of true, before we try running tests. This is useful in any cluster which needs some base pod-based services running before it can be used.")
	flag.DurationVar(&TestContext.SystemPodsStartupTimeout, "system-pods-startup-timeout", 10*time.Minute, "Timeout for waiting for all system pods to be running before starting tests.")
	flag.StringVar(&TestContext.UpgradeTarget, "upgrade-target", "ci/latest", "Version to upgrade to (e.g. 'release/stable', 'release/latest', 'ci/latest', '0.19.1', '0.19.1-669-gabac8c8') if doing an upgrade test.")
	flag.StringVar(&TestContext.UpgradeImage, "upgrade-image", "", "Image to upgrade to (e.g. 'container_vm' or 'gci') if doing an upgrade test.")
	flag.StringVar(&TestContext.PrometheusPushGateway, "prom-push-gateway", "", "The URL to prometheus gateway, so that metrics can be pushed during e2es and scraped by prometheus. Typically something like 127.0.0.1:9091.")
	flag.BoolVar(&TestContext.CleanStart, "clean-start", false, "If true, purge all namespaces except default and system before running tests. This serves to Cleanup test namespaces from failed/interrupted e2e runs in a long-lived cluster.")
	flag.BoolVar(&TestContext.GarbageCollectorEnabled, "garbage-collector-enabled", true, "Set to true if the garbage collector is enabled in the kube-apiserver and kube-controller-manager, then some tests will rely on the garbage collector to delete dependent resources.")
}

// Register flags specific to the node e2e test suite.
func RegisterNodeFlags() {
	flag.StringVar(&TestContext.NodeName, "node-name", "", "Name of the node to run tests on (node e2e suite only).")
	// TODO(random-liu): Remove kubelet related flags when we move the kubelet start logic out of the test.
	// TODO(random-liu): Find someway to get kubelet configuration, and automatic config and filter test based on the configuration.
	flag.BoolVar(&TestContext.DisableKubenet, "disable-kubenet", false, "If true, start kubelet without kubenet. (default false)")
	flag.StringVar(&TestContext.EvictionHard, "eviction-hard", "memory.available<250Mi,nodefs.available<10%,nodefs.inodesFree<5%", "The hard eviction thresholds. If set, pods get evicted when the specified resources drop below the thresholds.")
	flag.BoolVar(&TestContext.CgroupsPerQOS, "cgroups-per-qos", false, "Enable creation of QoS cgroup hierarchy, if true top level QoS and pod cgroups are created.")
	flag.StringVar(&TestContext.CgroupDriver, "cgroup-driver", "", "Driver that the kubelet uses to manipulate cgroups on the host.  Possible values: 'cgroupfs', 'systemd'")
	flag.StringVar(&TestContext.ManifestPath, "manifest-path", "", "The path to the static pod manifest file.")
	flag.BoolVar(&TestContext.PrepullImages, "prepull-images", true, "If true, prepull images so image pull failures do not cause test failures.")
	flag.StringVar(&TestContext.RuntimeIntegrationType, "runtime-integration-type", "", "Choose the integration path for the container runtime, mainly used for CRI validation.")
	flag.StringVar(&TestContext.ContainerRuntimeEndpoint, "container-runtime-endpoint", "", "The endpoint of remote container runtime grpc server, mainly used for Remote CRI validation.")
	flag.StringVar(&TestContext.MounterPath, "experimental-mounter-path", "", "Path of mounter binary. Leave empty to use the default mount.")
	flag.StringVar(&TestContext.MounterRootfsPath, "experimental-mounter-rootfs-path", "", "Absolute path to root filesystem for the mounter binary.")
}

// overwriteFlagsWithViperConfig finds and writes values to flags using viper as input.
func overwriteFlagsWithViperConfig() {
	viperFlagSetter := func(f *flag.Flag) {
		if viper.IsSet(f.Name) {
			f.Value.Set(viper.GetString(f.Name))
		}
	}
	flag.VisitAll(viperFlagSetter)
}

// ViperizeFlags sets up all flag and config processing. Future configuration info should be added to viper, not to flags.
func ViperizeFlags() {

	// Part 1: Set regular flags.
	// TODO: Future, lets eliminate e2e 'flag' deps entirely in favor of viper only,
	// since go test 'flag's are sort of incompatible w/ flag, glog, etc.
	RegisterCommonFlags()
	RegisterClusterFlags()
	flag.Parse()

	// Part 2: Set Viper provided flags.
	// This must be done after common flags are registered, since Viper is a flag option.
	viper.SetConfigName(TestContext.Viper)
	viper.AddConfigPath(".")
	viper.ReadInConfig()

	// TODO Consider wether or not we want to use overwriteFlagsWithViperConfig().
	viper.Unmarshal(&TestContext)
}
