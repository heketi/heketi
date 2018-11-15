//
// Copyright (c) 2016 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package kubeexec

import (
	"os"
	"testing"

	restclient "k8s.io/client-go/rest"

	"github.com/heketi/heketi/executors/cmdexec"
	"github.com/heketi/heketi/pkg/logging"
	"github.com/heketi/tests"
)

func init() {
	inClusterConfig = func() (*restclient.Config, error) {
		return &restclient.Config{}, nil
	}
	logger.SetLevel(logging.LEVEL_NOLOG)
}

func TestNewKubeExecutor(t *testing.T) {
	config := &KubeConfig{
		CmdConfig: cmdexec.CmdConfig{
			Fstab: "myfstab",
		},
		Namespace: "mynamespace",
	}

	k, err := NewKubeExecutor(config)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, k.Fstab == "myfstab")
	tests.Assert(t, k.Throttlemap != nil)
	tests.Assert(t, k.config != nil)
}

func TestNewKubeExecutorNoNamespace(t *testing.T) {
	// this test only works correctly if the test is _not_ run
	// in a k8s type environment. It will fail if run w/in a pod.
	// Since we're trying to run tests inside openshift, disable it for now.
	t.Skipf("This is a silly test")

	config := &KubeConfig{
		CmdConfig: cmdexec.CmdConfig{
			Fstab: "myfstab",
		},
	}

	k, err := NewKubeExecutor(config)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, k == nil)
}

func TestNewKubeExecutorRebalanceOnExpansion(t *testing.T) {

	// This tests access to configurations
	// from the sshconfig exector

	config := &KubeConfig{
		CmdConfig: cmdexec.CmdConfig{
			Fstab: "myfstab",
		},
		Namespace: "mynamespace",
	}

	k, err := NewKubeExecutor(config)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, k.Fstab == "myfstab")
	tests.Assert(t, k.Throttlemap != nil)
	tests.Assert(t, k.config != nil)
	tests.Assert(t, k.RebalanceOnExpansion() == false)

	config = &KubeConfig{
		CmdConfig: cmdexec.CmdConfig{
			Fstab:                "myfstab",
			RebalanceOnExpansion: true,
		},
		Namespace: "mynamespace",
	}

	k, err = NewKubeExecutor(config)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, k.Fstab == "myfstab")
	tests.Assert(t, k.Throttlemap != nil)
	tests.Assert(t, k.config != nil)
	tests.Assert(t, k.RebalanceOnExpansion() == true)
}

func TestKubeExecutorEnvVariables(t *testing.T) {

	// set environment
	err := os.Setenv("HEKETI_SNAPSHOT_LIMIT", "999")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	defer os.Unsetenv("HEKETI_SNAPSHOT_LIMIT")

	err = os.Setenv("HEKETI_FSTAB", "anotherfstab")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	defer os.Unsetenv("HEKETI_FSTAB")

	config := &KubeConfig{
		CmdConfig: cmdexec.CmdConfig{
			Fstab: "myfstab",
		},
		Namespace: "mynamespace",
	}

	k, err := NewKubeExecutor(config)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, k.Throttlemap != nil)
	tests.Assert(t, k.config != nil)
	tests.Assert(t, k.Fstab == "anotherfstab")
	tests.Assert(t, k.SnapShotLimit() == 999)

}
