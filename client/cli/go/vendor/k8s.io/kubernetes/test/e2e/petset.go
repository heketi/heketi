/*
Copyright 2014 The Kubernetes Authors.

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

package e2e

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	inf "gopkg.in/inf.v0"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/apps"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/controller/petset"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	utilyaml "k8s.io/kubernetes/pkg/util/yaml"
	"k8s.io/kubernetes/pkg/watch"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	statefulsetPoll = 10 * time.Second
	// Some pets install base packages via wget
	statefulsetTimeout = 10 * time.Minute
	// Timeout for pet pods to change state
	petPodTimeout           = 5 * time.Minute
	zookeeperManifestPath   = "test/e2e/testing-manifests/petset/zookeeper"
	mysqlGaleraManifestPath = "test/e2e/testing-manifests/petset/mysql-galera"
	redisManifestPath       = "test/e2e/testing-manifests/petset/redis"
	// Should the test restart statefulset clusters?
	// TODO: enable when we've productionzed bringup of pets in this e2e.
	restartCluster = false

	// Timeout for reads from databases running on pets.
	readTimeout = 60 * time.Second
)

// Time: 25m, slow by design.
// GCE Quota requirements: 3 pds, one per pet manifest declared above.
// GCE Api requirements: nodes and master need storage r/w permissions.
var _ = framework.KubeDescribe("StatefulSet [Slow] [Feature:PetSet]", func() {
	f := framework.NewDefaultFramework("statefulset")
	var ns string
	var c clientset.Interface

	BeforeEach(func() {
		c = f.ClientSet
		ns = f.Namespace.Name
	})

	framework.KubeDescribe("Basic StatefulSet functionality", func() {
		psName := "pet"
		labels := map[string]string{
			"foo": "bar",
			"baz": "blah",
		}
		headlessSvcName := "test"

		BeforeEach(func() {
			By("creating service " + headlessSvcName + " in namespace " + ns)
			headlessService := createServiceSpec(headlessSvcName, "", true, labels)
			_, err := c.Core().Services(ns).Create(headlessService)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if CurrentGinkgoTestDescription().Failed {
				dumpDebugInfo(c, ns)
			}
			framework.Logf("Deleting all statefulset in ns %v", ns)
			deleteAllStatefulSets(c, ns)
		})

		It("should provide basic identity [Feature:StatefulSet]", func() {
			By("creating statefulset " + psName + " in namespace " + ns)
			petMounts := []api.VolumeMount{{Name: "datadir", MountPath: "/data/"}}
			podMounts := []api.VolumeMount{{Name: "home", MountPath: "/home"}}
			ps := newStatefulSet(psName, ns, headlessSvcName, 3, petMounts, podMounts, labels)
			setInitializedAnnotation(ps, "false")

			_, err := c.Apps().StatefulSets(ns).Create(ps)
			Expect(err).NotTo(HaveOccurred())

			pst := statefulSetTester{c: c}

			By("Saturating pet set " + ps.Name)
			pst.saturate(ps)

			By("Verifying statefulset mounted data directory is usable")
			ExpectNoError(pst.checkMount(ps, "/data"))

			By("Verifying statefulset provides a stable hostname for each pod")
			ExpectNoError(pst.checkHostname(ps))

			cmd := "echo $(hostname) > /data/hostname; sync;"
			By("Running " + cmd + " in all pets")
			ExpectNoError(pst.execInPets(ps, cmd))

			By("Restarting statefulset " + ps.Name)
			pst.restart(ps)
			pst.saturate(ps)

			By("Verifying statefulset mounted data directory is usable")
			ExpectNoError(pst.checkMount(ps, "/data"))

			cmd = "if [ \"$(cat /data/hostname)\" = \"$(hostname)\" ]; then exit 0; else exit 1; fi"
			By("Running " + cmd + " in all pets")
			ExpectNoError(pst.execInPets(ps, cmd))
		})

		It("should handle healthy pet restarts during scale [Feature:PetSet]", func() {
			By("creating statefulset " + psName + " in namespace " + ns)

			petMounts := []api.VolumeMount{{Name: "datadir", MountPath: "/data/"}}
			podMounts := []api.VolumeMount{{Name: "home", MountPath: "/home"}}
			ps := newStatefulSet(psName, ns, headlessSvcName, 2, petMounts, podMounts, labels)
			setInitializedAnnotation(ps, "false")
			_, err := c.Apps().StatefulSets(ns).Create(ps)
			Expect(err).NotTo(HaveOccurred())

			pst := statefulSetTester{c: c}

			pst.waitForRunning(1, ps)

			By("Marking pet at index 0 as healthy.")
			pst.setHealthy(ps)

			By("Waiting for pet at index 1 to enter running.")
			pst.waitForRunning(2, ps)

			// Now we have 1 healthy and 1 unhealthy pet. Deleting the healthy pet should *not*
			// create a new pet till the remaining pet becomes healthy, which won't happen till
			// we set the healthy bit.

			By("Deleting healthy pet at index 0.")
			pst.deletePetAtIndex(0, ps)

			By("Confirming pet at index 0 is not recreated.")
			pst.confirmPetCount(1, ps, 10*time.Second)

			By("Deleting unhealthy pet at index 1.")
			pst.deletePetAtIndex(1, ps)

			By("Confirming all pets in statefulset are created.")
			pst.saturate(ps)
		})
	})

	framework.KubeDescribe("Deploy clustered applications [Slow] [Feature:PetSet]", func() {
		AfterEach(func() {
			if CurrentGinkgoTestDescription().Failed {
				dumpDebugInfo(c, ns)
			}
			framework.Logf("Deleting all statefulset in ns %v", ns)
			deleteAllStatefulSets(c, ns)
		})

		It("should creating a working zookeeper cluster [Feature:PetSet]", func() {
			pst := &statefulSetTester{c: c}
			pet := &zookeeperTester{tester: pst}
			By("Deploying " + pet.name())
			ps := pet.deploy(ns)

			By("Creating foo:bar in member with index 0")
			pet.write(0, map[string]string{"foo": "bar"})

			if restartCluster {
				By("Restarting pet set " + ps.Name)
				pst.restart(ps)
				pst.waitForRunning(ps.Spec.Replicas, ps)
			}

			By("Reading value under foo from member with index 2")
			if err := pollReadWithTimeout(pet, 2, "foo", "bar"); err != nil {
				framework.Failf("%v", err)
			}
		})

		It("should creating a working redis cluster [Feature:PetSet]", func() {
			pst := &statefulSetTester{c: c}
			pet := &redisTester{tester: pst}
			By("Deploying " + pet.name())
			ps := pet.deploy(ns)

			By("Creating foo:bar in member with index 0")
			pet.write(0, map[string]string{"foo": "bar"})

			if restartCluster {
				By("Restarting pet set " + ps.Name)
				pst.restart(ps)
				pst.waitForRunning(ps.Spec.Replicas, ps)
			}

			By("Reading value under foo from member with index 2")
			if err := pollReadWithTimeout(pet, 2, "foo", "bar"); err != nil {
				framework.Failf("%v", err)
			}
		})

		It("should creating a working mysql cluster [Feature:PetSet]", func() {
			pst := &statefulSetTester{c: c}
			pet := &mysqlGaleraTester{tester: pst}
			By("Deploying " + pet.name())
			ps := pet.deploy(ns)

			By("Creating foo:bar in member with index 0")
			pet.write(0, map[string]string{"foo": "bar"})

			if restartCluster {
				By("Restarting pet set " + ps.Name)
				pst.restart(ps)
				pst.waitForRunning(ps.Spec.Replicas, ps)
			}

			By("Reading value under foo from member with index 2")
			if err := pollReadWithTimeout(pet, 2, "foo", "bar"); err != nil {
				framework.Failf("%v", err)
			}
		})
	})
})

var _ = framework.KubeDescribe("Pet set recreate [Slow] [Feature:PetSet]", func() {
	f := framework.NewDefaultFramework("pet-set-recreate")
	var c clientset.Interface
	var ns string

	labels := map[string]string{
		"foo": "bar",
		"baz": "blah",
	}
	headlessSvcName := "test"
	podName := "test-pod"
	statefulSetName := "web"
	petPodName := "web-0"

	BeforeEach(func() {
		framework.SkipUnlessProviderIs("gce", "vagrant")
		By("creating service " + headlessSvcName + " in namespace " + f.Namespace.Name)
		headlessService := createServiceSpec(headlessSvcName, "", true, labels)
		_, err := f.ClientSet.Core().Services(f.Namespace.Name).Create(headlessService)
		framework.ExpectNoError(err)
		c = f.ClientSet
		ns = f.Namespace.Name
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			dumpDebugInfo(c, ns)
		}
		By("Deleting all statefulset in ns " + ns)
		deleteAllStatefulSets(c, ns)
	})

	It("should recreate evicted statefulset", func() {
		By("looking for a node to schedule pet set and pod")
		nodes := framework.GetReadySchedulableNodesOrDie(f.ClientSet)
		node := nodes.Items[0]

		By("creating pod with conflicting port in namespace " + f.Namespace.Name)
		conflictingPort := api.ContainerPort{HostPort: 21017, ContainerPort: 21017, Name: "conflict"}
		pod := &api.Pod{
			ObjectMeta: api.ObjectMeta{
				Name: podName,
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					{
						Name:  "nginx",
						Image: "gcr.io/google_containers/nginx-slim:0.7",
						Ports: []api.ContainerPort{conflictingPort},
					},
				},
				NodeName: node.Name,
			},
		}
		pod, err := f.ClientSet.Core().Pods(f.Namespace.Name).Create(pod)
		framework.ExpectNoError(err)

		By("creating statefulset with conflicting port in namespace " + f.Namespace.Name)
		ps := newStatefulSet(statefulSetName, f.Namespace.Name, headlessSvcName, 1, nil, nil, labels)
		petContainer := &ps.Spec.Template.Spec.Containers[0]
		petContainer.Ports = append(petContainer.Ports, conflictingPort)
		ps.Spec.Template.Spec.NodeName = node.Name
		_, err = f.ClientSet.Apps().StatefulSets(f.Namespace.Name).Create(ps)
		framework.ExpectNoError(err)

		By("waiting until pod " + podName + " will start running in namespace " + f.Namespace.Name)
		if err := f.WaitForPodRunning(podName); err != nil {
			framework.Failf("Pod %v did not start running: %v", podName, err)
		}

		var initialPetPodUID types.UID
		By("waiting until pet pod " + petPodName + " will be recreated and deleted at least once in namespace " + f.Namespace.Name)
		w, err := f.ClientSet.Core().Pods(f.Namespace.Name).Watch(api.SingleObject(api.ObjectMeta{Name: petPodName}))
		framework.ExpectNoError(err)
		// we need to get UID from pod in any state and wait until pet set controller will remove pod atleast once
		_, err = watch.Until(petPodTimeout, w, func(event watch.Event) (bool, error) {
			pod := event.Object.(*api.Pod)
			switch event.Type {
			case watch.Deleted:
				framework.Logf("Observed delete event for pet pod %v in namespace %v", pod.Name, pod.Namespace)
				if initialPetPodUID == "" {
					return false, nil
				}
				return true, nil
			}
			framework.Logf("Observed pet pod in namespace: %v, name: %v, uid: %v, status phase: %v. Waiting for statefulset controller to delete.",
				pod.Namespace, pod.Name, pod.UID, pod.Status.Phase)
			initialPetPodUID = pod.UID
			return false, nil
		})
		if err != nil {
			framework.Failf("Pod %v expected to be re-created atleast once", petPodName)
		}

		By("removing pod with conflicting port in namespace " + f.Namespace.Name)
		err = f.ClientSet.Core().Pods(f.Namespace.Name).Delete(pod.Name, api.NewDeleteOptions(0))
		framework.ExpectNoError(err)

		By("waiting when pet pod " + petPodName + " will be recreated in namespace " + f.Namespace.Name + " and will be in running state")
		// we may catch delete event, thats why we are waiting for running phase like this, and not with watch.Until
		Eventually(func() error {
			petPod, err := f.ClientSet.Core().Pods(f.Namespace.Name).Get(petPodName)
			if err != nil {
				return err
			}
			if petPod.Status.Phase != api.PodRunning {
				return fmt.Errorf("Pod %v is not in running phase: %v", petPod.Name, petPod.Status.Phase)
			} else if petPod.UID == initialPetPodUID {
				return fmt.Errorf("Pod %v wasn't recreated: %v == %v", petPod.Name, petPod.UID, initialPetPodUID)
			}
			return nil
		}, petPodTimeout, 2*time.Second).Should(BeNil())
	})
})

func dumpDebugInfo(c clientset.Interface, ns string) {
	pl, _ := c.Core().Pods(ns).List(api.ListOptions{LabelSelector: labels.Everything()})
	for _, p := range pl.Items {
		desc, _ := framework.RunKubectl("describe", "po", p.Name, fmt.Sprintf("--namespace=%v", ns))
		framework.Logf("\nOutput of kubectl describe %v:\n%v", p.Name, desc)

		l, _ := framework.RunKubectl("logs", p.Name, fmt.Sprintf("--namespace=%v", ns), "--tail=100")
		framework.Logf("\nLast 100 log lines of %v:\n%v", p.Name, l)
	}
}

func kubectlExecWithRetries(args ...string) (out string) {
	var err error
	for i := 0; i < 3; i++ {
		if out, err = framework.RunKubectl(args...); err == nil {
			return
		}
		framework.Logf("Retrying %v:\nerror %v\nstdout %v", args, err, out)
	}
	framework.Failf("Failed to execute \"%v\" with retries: %v", args, err)
	return
}

type petTester interface {
	deploy(ns string) *apps.StatefulSet
	write(petIndex int, kv map[string]string)
	read(petIndex int, key string) string
	name() string
}

type zookeeperTester struct {
	ps     *apps.StatefulSet
	tester *statefulSetTester
}

func (z *zookeeperTester) name() string {
	return "zookeeper"
}

func (z *zookeeperTester) deploy(ns string) *apps.StatefulSet {
	z.ps = z.tester.createStatefulSet(zookeeperManifestPath, ns)
	return z.ps
}

func (z *zookeeperTester) write(petIndex int, kv map[string]string) {
	name := fmt.Sprintf("%v-%d", z.ps.Name, petIndex)
	ns := fmt.Sprintf("--namespace=%v", z.ps.Namespace)
	for k, v := range kv {
		cmd := fmt.Sprintf("/opt/zookeeper/bin/zkCli.sh create /%v %v", k, v)
		framework.Logf(framework.RunKubectlOrDie("exec", ns, name, "--", "/bin/sh", "-c", cmd))
	}
}

func (z *zookeeperTester) read(petIndex int, key string) string {
	name := fmt.Sprintf("%v-%d", z.ps.Name, petIndex)
	ns := fmt.Sprintf("--namespace=%v", z.ps.Namespace)
	cmd := fmt.Sprintf("/opt/zookeeper/bin/zkCli.sh get /%v", key)
	return lastLine(framework.RunKubectlOrDie("exec", ns, name, "--", "/bin/sh", "-c", cmd))
}

type mysqlGaleraTester struct {
	ps     *apps.StatefulSet
	tester *statefulSetTester
}

func (m *mysqlGaleraTester) name() string {
	return "mysql: galera"
}

func (m *mysqlGaleraTester) mysqlExec(cmd, ns, podName string) string {
	cmd = fmt.Sprintf("/usr/bin/mysql -u root -B -e '%v'", cmd)
	// TODO: Find a readiness probe for mysql that guarantees writes will
	// succeed and ditch retries. Current probe only reads, so there's a window
	// for a race.
	return kubectlExecWithRetries(fmt.Sprintf("--namespace=%v", ns), "exec", podName, "--", "/bin/sh", "-c", cmd)
}

func (m *mysqlGaleraTester) deploy(ns string) *apps.StatefulSet {
	m.ps = m.tester.createStatefulSet(mysqlGaleraManifestPath, ns)

	framework.Logf("Deployed statefulset %v, initializing database", m.ps.Name)
	for _, cmd := range []string{
		"create database statefulset;",
		"use statefulset; create table pet (k varchar(20), v varchar(20));",
	} {
		framework.Logf(m.mysqlExec(cmd, ns, fmt.Sprintf("%v-0", m.ps.Name)))
	}
	return m.ps
}

func (m *mysqlGaleraTester) write(petIndex int, kv map[string]string) {
	name := fmt.Sprintf("%v-%d", m.ps.Name, petIndex)
	for k, v := range kv {
		cmd := fmt.Sprintf("use  statefulset; insert into pet (k, v) values (\"%v\", \"%v\");", k, v)
		framework.Logf(m.mysqlExec(cmd, m.ps.Namespace, name))
	}
}

func (m *mysqlGaleraTester) read(petIndex int, key string) string {
	name := fmt.Sprintf("%v-%d", m.ps.Name, petIndex)
	return lastLine(m.mysqlExec(fmt.Sprintf("use statefulset; select v from pet where k=\"%v\";", key), m.ps.Namespace, name))
}

type redisTester struct {
	ps     *apps.StatefulSet
	tester *statefulSetTester
}

func (m *redisTester) name() string {
	return "redis: master/slave"
}

func (m *redisTester) redisExec(cmd, ns, podName string) string {
	cmd = fmt.Sprintf("/opt/redis/redis-cli -h %v %v", podName, cmd)
	return framework.RunKubectlOrDie(fmt.Sprintf("--namespace=%v", ns), "exec", podName, "--", "/bin/sh", "-c", cmd)
}

func (m *redisTester) deploy(ns string) *apps.StatefulSet {
	m.ps = m.tester.createStatefulSet(redisManifestPath, ns)
	return m.ps
}

func (m *redisTester) write(petIndex int, kv map[string]string) {
	name := fmt.Sprintf("%v-%d", m.ps.Name, petIndex)
	for k, v := range kv {
		framework.Logf(m.redisExec(fmt.Sprintf("SET %v %v", k, v), m.ps.Namespace, name))
	}
}

func (m *redisTester) read(petIndex int, key string) string {
	name := fmt.Sprintf("%v-%d", m.ps.Name, petIndex)
	return lastLine(m.redisExec(fmt.Sprintf("GET %v", key), m.ps.Namespace, name))
}

func lastLine(out string) string {
	outLines := strings.Split(strings.Trim(out, "\n"), "\n")
	return outLines[len(outLines)-1]
}

func statefulSetFromManifest(fileName, ns string) *apps.StatefulSet {
	var ps apps.StatefulSet
	framework.Logf("Parsing statefulset from %v", fileName)
	data, err := ioutil.ReadFile(fileName)
	Expect(err).NotTo(HaveOccurred())
	json, err := utilyaml.ToJSON(data)
	Expect(err).NotTo(HaveOccurred())

	Expect(runtime.DecodeInto(api.Codecs.UniversalDecoder(), json, &ps)).NotTo(HaveOccurred())
	ps.Namespace = ns
	if ps.Spec.Selector == nil {
		ps.Spec.Selector = &unversioned.LabelSelector{
			MatchLabels: ps.Spec.Template.Labels,
		}
	}
	return &ps
}

// statefulSetTester has all methods required to test a single statefulset.
type statefulSetTester struct {
	c clientset.Interface
}

func (p *statefulSetTester) createStatefulSet(manifestPath, ns string) *apps.StatefulSet {
	mkpath := func(file string) string {
		return filepath.Join(framework.TestContext.RepoRoot, manifestPath, file)
	}
	ps := statefulSetFromManifest(mkpath("petset.yaml"), ns)

	framework.Logf(fmt.Sprintf("creating " + ps.Name + " service"))
	framework.RunKubectlOrDie("create", "-f", mkpath("service.yaml"), fmt.Sprintf("--namespace=%v", ns))

	framework.Logf(fmt.Sprintf("creating statefulset %v/%v with %d replicas and selector %+v", ps.Namespace, ps.Name, ps.Spec.Replicas, ps.Spec.Selector))
	framework.RunKubectlOrDie("create", "-f", mkpath("petset.yaml"), fmt.Sprintf("--namespace=%v", ns))
	p.waitForRunning(ps.Spec.Replicas, ps)
	return ps
}

func (p *statefulSetTester) checkMount(ps *apps.StatefulSet, mountPath string) error {
	for _, cmd := range []string{
		// Print inode, size etc
		fmt.Sprintf("ls -idlh %v", mountPath),
		// Print subdirs
		fmt.Sprintf("find %v", mountPath),
		// Try writing
		fmt.Sprintf("touch %v", filepath.Join(mountPath, fmt.Sprintf("%v", time.Now().UnixNano()))),
	} {
		if err := p.execInPets(ps, cmd); err != nil {
			return fmt.Errorf("failed to execute %v, error: %v", cmd, err)
		}
	}
	return nil
}

func (p *statefulSetTester) execInPets(ps *apps.StatefulSet, cmd string) error {
	podList := p.getPodList(ps)
	for _, pet := range podList.Items {
		stdout, err := framework.RunHostCmd(pet.Namespace, pet.Name, cmd)
		framework.Logf("stdout of %v on %v: %v", cmd, pet.Name, stdout)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *statefulSetTester) checkHostname(ps *apps.StatefulSet) error {
	cmd := "printf $(hostname)"
	podList := p.getPodList(ps)
	for _, pet := range podList.Items {
		hostname, err := framework.RunHostCmd(pet.Namespace, pet.Name, cmd)
		if err != nil {
			return err
		}
		if hostname != pet.Name {
			return fmt.Errorf("unexpected hostname (%s) and stateful pod name (%s) not equal", hostname, pet.Name)
		}
	}
	return nil
}
func (p *statefulSetTester) saturate(ps *apps.StatefulSet) {
	// TODO: Watch events and check that creation timestamps don't overlap
	var i int32
	for i = 0; i < ps.Spec.Replicas; i++ {
		framework.Logf("Waiting for pet at index " + fmt.Sprintf("%v", i+1) + " to enter Running")
		p.waitForRunning(i+1, ps)
		framework.Logf("Marking pet at index " + fmt.Sprintf("%v", i) + " healthy")
		p.setHealthy(ps)
	}
}

func (p *statefulSetTester) deletePetAtIndex(index int, ps *apps.StatefulSet) {
	// TODO: we won't use "-index" as the name strategy forever,
	// pull the name out from an identity mapper.
	name := fmt.Sprintf("%v-%v", ps.Name, index)
	noGrace := int64(0)
	if err := p.c.Core().Pods(ps.Namespace).Delete(name, &api.DeleteOptions{GracePeriodSeconds: &noGrace}); err != nil {
		framework.Failf("Failed to delete pet %v for StatefulSet %v: %v", name, ps.Name, ps.Namespace, err)
	}
}

func (p *statefulSetTester) scale(ps *apps.StatefulSet, count int32) error {
	name := ps.Name
	ns := ps.Namespace
	p.update(ns, name, func(ps *apps.StatefulSet) { ps.Spec.Replicas = count })

	var petList *api.PodList
	pollErr := wait.PollImmediate(statefulsetPoll, statefulsetTimeout, func() (bool, error) {
		petList = p.getPodList(ps)
		if int32(len(petList.Items)) == count {
			return true, nil
		}
		return false, nil
	})
	if pollErr != nil {
		unhealthy := []string{}
		for _, pet := range petList.Items {
			delTs, phase, readiness := pet.DeletionTimestamp, pet.Status.Phase, api.IsPodReady(&pet)
			if delTs != nil || phase != api.PodRunning || !readiness {
				unhealthy = append(unhealthy, fmt.Sprintf("%v: deletion %v, phase %v, readiness %v", pet.Name, delTs, phase, readiness))
			}
		}
		return fmt.Errorf("Failed to scale statefulset to %d in %v. Remaining pods:\n%v", count, statefulsetTimeout, unhealthy)
	}
	return nil
}

func (p *statefulSetTester) restart(ps *apps.StatefulSet) {
	oldReplicas := ps.Spec.Replicas
	ExpectNoError(p.scale(ps, 0))
	p.update(ps.Namespace, ps.Name, func(ps *apps.StatefulSet) { ps.Spec.Replicas = oldReplicas })
}

func (p *statefulSetTester) update(ns, name string, update func(ps *apps.StatefulSet)) {
	for i := 0; i < 3; i++ {
		ps, err := p.c.Apps().StatefulSets(ns).Get(name)
		if err != nil {
			framework.Failf("failed to get statefulset %q: %v", name, err)
		}
		update(ps)
		ps, err = p.c.Apps().StatefulSets(ns).Update(ps)
		if err == nil {
			return
		}
		if !apierrs.IsConflict(err) && !apierrs.IsServerTimeout(err) {
			framework.Failf("failed to update statefulset %q: %v", name, err)
		}
	}
	framework.Failf("too many retries draining statefulset %q", name)
}

func (p *statefulSetTester) getPodList(ps *apps.StatefulSet) *api.PodList {
	selector, err := unversioned.LabelSelectorAsSelector(ps.Spec.Selector)
	ExpectNoError(err)
	podList, err := p.c.Core().Pods(ps.Namespace).List(api.ListOptions{LabelSelector: selector})
	ExpectNoError(err)
	return podList
}

func (p *statefulSetTester) confirmPetCount(count int, ps *apps.StatefulSet, timeout time.Duration) {
	start := time.Now()
	deadline := start.Add(timeout)
	for t := time.Now(); t.Before(deadline); t = time.Now() {
		podList := p.getPodList(ps)
		petCount := len(podList.Items)
		if petCount != count {
			framework.Failf("StatefulSet %v scaled unexpectedly scaled to %d -> %d replicas: %+v", ps.Name, count, len(podList.Items), podList)
		}
		framework.Logf("Verifying statefulset %v doesn't scale past %d for another %+v", ps.Name, count, deadline.Sub(t))
		time.Sleep(1 * time.Second)
	}
}

func (p *statefulSetTester) waitForRunning(numPets int32, ps *apps.StatefulSet) {
	pollErr := wait.PollImmediate(statefulsetPoll, statefulsetTimeout,
		func() (bool, error) {
			podList := p.getPodList(ps)
			if int32(len(podList.Items)) < numPets {
				framework.Logf("Found %d pets, waiting for %d", len(podList.Items), numPets)
				return false, nil
			}
			if int32(len(podList.Items)) > numPets {
				return false, fmt.Errorf("Too many pods scheduled, expected %d got %d", numPets, len(podList.Items))
			}
			for _, p := range podList.Items {
				isReady := api.IsPodReady(&p)
				if p.Status.Phase != api.PodRunning || !isReady {
					framework.Logf("Waiting for pod %v to enter %v - Ready=True, currently %v - Ready=%v", p.Name, api.PodRunning, p.Status.Phase, isReady)
					return false, nil
				}
			}
			return true, nil
		})
	if pollErr != nil {
		framework.Failf("Failed waiting for pods to enter running: %v", pollErr)
	}

	p.waitForStatus(ps, numPets)
}

func (p *statefulSetTester) setHealthy(ps *apps.StatefulSet) {
	podList := p.getPodList(ps)
	markedHealthyPod := ""
	for _, pod := range podList.Items {
		if pod.Status.Phase != api.PodRunning {
			framework.Failf("Found pod in %v cannot set health", pod.Status.Phase)
		}
		if isInitialized(pod) {
			continue
		}
		if markedHealthyPod != "" {
			framework.Failf("Found multiple non-healthy pets: %v and %v", pod.Name, markedHealthyPod)
		}
		p, err := framework.UpdatePodWithRetries(p.c, pod.Namespace, pod.Name, func(up *api.Pod) {
			up.Annotations[petset.StatefulSetInitAnnotation] = "true"
		})
		ExpectNoError(err)
		framework.Logf("Set annotation %v to %v on pod %v", petset.StatefulSetInitAnnotation, p.Annotations[petset.StatefulSetInitAnnotation], pod.Name)
		markedHealthyPod = pod.Name
	}
}

func (p *statefulSetTester) waitForStatus(ps *apps.StatefulSet, expectedReplicas int32) {
	framework.Logf("Waiting for statefulset status.replicas updated to %d", expectedReplicas)

	ns, name := ps.Namespace, ps.Name
	pollErr := wait.PollImmediate(statefulsetPoll, statefulsetTimeout,
		func() (bool, error) {
			psGet, err := p.c.Apps().StatefulSets(ns).Get(name)
			if err != nil {
				return false, err
			}
			if psGet.Status.Replicas != expectedReplicas {
				framework.Logf("Waiting for pet set status to become %d, currently %d", expectedReplicas, psGet.Status.Replicas)
				return false, nil
			}
			return true, nil
		})
	if pollErr != nil {
		framework.Failf("Failed waiting for pet set status.replicas updated to %d: %v", expectedReplicas, pollErr)
	}
}

func deleteAllStatefulSets(c clientset.Interface, ns string) {
	pst := &statefulSetTester{c: c}
	psList, err := c.Apps().StatefulSets(ns).List(api.ListOptions{LabelSelector: labels.Everything()})
	ExpectNoError(err)

	// Scale down each statefulset, then delete it completely.
	// Deleting a pvc without doing this will leak volumes, #25101.
	errList := []string{}
	for _, ps := range psList.Items {
		framework.Logf("Scaling statefulset %v to 0", ps.Name)
		if err := pst.scale(&ps, 0); err != nil {
			errList = append(errList, fmt.Sprintf("%v", err))
		}
		pst.waitForStatus(&ps, 0)
		framework.Logf("Deleting statefulset %v", ps.Name)
		if err := c.Apps().StatefulSets(ps.Namespace).Delete(ps.Name, nil); err != nil {
			errList = append(errList, fmt.Sprintf("%v", err))
		}
	}

	// pvs are global, so we need to wait for the exact ones bound to the statefulset pvcs.
	pvNames := sets.NewString()
	// TODO: Don't assume all pvcs in the ns belong to a statefulset
	pvcPollErr := wait.PollImmediate(statefulsetPoll, statefulsetTimeout, func() (bool, error) {
		pvcList, err := c.Core().PersistentVolumeClaims(ns).List(api.ListOptions{LabelSelector: labels.Everything()})
		if err != nil {
			framework.Logf("WARNING: Failed to list pvcs, retrying %v", err)
			return false, nil
		}
		for _, pvc := range pvcList.Items {
			pvNames.Insert(pvc.Spec.VolumeName)
			// TODO: Double check that there are no pods referencing the pvc
			framework.Logf("Deleting pvc: %v with volume %v", pvc.Name, pvc.Spec.VolumeName)
			if err := c.Core().PersistentVolumeClaims(ns).Delete(pvc.Name, nil); err != nil {
				return false, nil
			}
		}
		return true, nil
	})
	if pvcPollErr != nil {
		errList = append(errList, fmt.Sprintf("Timeout waiting for pvc deletion."))
	}

	pollErr := wait.PollImmediate(statefulsetPoll, statefulsetTimeout, func() (bool, error) {
		pvList, err := c.Core().PersistentVolumes().List(api.ListOptions{LabelSelector: labels.Everything()})
		if err != nil {
			framework.Logf("WARNING: Failed to list pvs, retrying %v", err)
			return false, nil
		}
		waitingFor := []string{}
		for _, pv := range pvList.Items {
			if pvNames.Has(pv.Name) {
				waitingFor = append(waitingFor, fmt.Sprintf("%v: %+v", pv.Name, pv.Status))
			}
		}
		if len(waitingFor) == 0 {
			return true, nil
		}
		framework.Logf("Still waiting for pvs of statefulset to disappear:\n%v", strings.Join(waitingFor, "\n"))
		return false, nil
	})
	if pollErr != nil {
		errList = append(errList, fmt.Sprintf("Timeout waiting for pv provisioner to delete pvs, this might mean the test leaked pvs."))
	}
	if len(errList) != 0 {
		ExpectNoError(fmt.Errorf("%v", strings.Join(errList, "\n")))
	}
}

func ExpectNoError(err error) {
	Expect(err).NotTo(HaveOccurred())
}

func pollReadWithTimeout(pet petTester, petNumber int, key, expectedVal string) error {
	err := wait.PollImmediate(time.Second, readTimeout, func() (bool, error) {
		val := pet.read(petNumber, key)
		if val == "" {
			return false, nil
		} else if val != expectedVal {
			return false, fmt.Errorf("expected value %v, found %v", expectedVal, val)
		}
		return true, nil
	})

	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("timed out when trying to read value for key %v from pet %d", key, petNumber)
	}
	return err
}

func isInitialized(pod api.Pod) bool {
	initialized, ok := pod.Annotations[petset.StatefulSetInitAnnotation]
	if !ok {
		return false
	}
	inited, err := strconv.ParseBool(initialized)
	if err != nil {
		framework.Failf("Couldn't parse statefulset init annotations %v", initialized)
	}
	return inited
}

func dec(i int64, exponent int) *inf.Dec {
	return inf.NewDec(i, inf.Scale(-exponent))
}

func newPVC(name string) api.PersistentVolumeClaim {
	return api.PersistentVolumeClaim{
		ObjectMeta: api.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"volume.alpha.kubernetes.io/storage-class": "anything",
			},
		},
		Spec: api.PersistentVolumeClaimSpec{
			AccessModes: []api.PersistentVolumeAccessMode{
				api.ReadWriteOnce,
			},
			Resources: api.ResourceRequirements{
				Requests: api.ResourceList{
					api.ResourceStorage: *resource.NewQuantity(1, resource.BinarySI),
				},
			},
		},
	}
}

func newStatefulSet(name, ns, governingSvcName string, replicas int32, petMounts []api.VolumeMount, podMounts []api.VolumeMount, labels map[string]string) *apps.StatefulSet {
	mounts := append(petMounts, podMounts...)
	claims := []api.PersistentVolumeClaim{}
	for _, m := range petMounts {
		claims = append(claims, newPVC(m.Name))
	}

	vols := []api.Volume{}
	for _, m := range podMounts {
		vols = append(vols, api.Volume{
			Name: m.Name,
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: fmt.Sprintf("/tmp/%v", m.Name),
				},
			},
		})
	}

	return &apps.StatefulSet{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1beta1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: apps.StatefulSetSpec{
			Selector: &unversioned.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: replicas,
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels:      labels,
					Annotations: map[string]string{},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:         "nginx",
							Image:        "gcr.io/google_containers/nginx-slim:0.7",
							VolumeMounts: mounts,
						},
					},
					Volumes: vols,
				},
			},
			VolumeClaimTemplates: claims,
			ServiceName:          governingSvcName,
		},
	}
}

func setInitializedAnnotation(ss *apps.StatefulSet, value string) {
	ss.Spec.Template.ObjectMeta.Annotations["pod.alpha.kubernetes.io/initialized"] = value
}
