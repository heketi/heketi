//
// Copyright (c) 2016 The heketi Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package sshexec

import (
	"strings"
	"testing"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

func TestSshExecBrickCreate(t *testing.T) {

	f := NewFakeSsh()
	defer tests.Patch(&sshNew,
		func(logger *utils.Logger, user string, file string) (Ssher, error) {
			return f, nil
		}).Restore()

	config := &SshConfig{
		PrivateKeyFile: "xkeyfile",
		User:           "xuser",
		Port:           "100",
		CLICommandConfig: CLICommandConfig{
			Fstab: "/my/fstab",
		},
	}

	s, err := NewSshExecutor(config)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)

	// Create a Brick
	b := &executors.BrickRequest{
		VgId:             "xvgid",
		Name:             "id",
		TpSize:           100,
		Size:             10,
		PoolMetadataSize: 5,
	}

	// Mock ssh function
	f.FakeConnectAndExec = func(host string,
		commands []string,
		timeoutMinutes int,
		useSudo bool) ([]string, error) {

		tests.Assert(t, host == "myhost:100", host)
		tests.Assert(t, len(commands) == 6)

		for i, cmd := range commands {
			cmd = strings.Trim(cmd, " ")
			switch i {
			case 0:
				tests.Assert(t,
					cmd == "mkdir -p /var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case 1:
				tests.Assert(t,
					cmd == "lvcreate --poolmetadatasize 5K "+
						"-c 256K -L 100K -T vg_xvgid/tp_id -V 10K -n brick_id", cmd)

			case 2:
				tests.Assert(t,
					cmd == "mkfs.xfs -i size=512 "+
						"-n size=8192 /dev/vg_xvgid/brick_id", cmd)

			case 3:
				tests.Assert(t,
					cmd == "echo \"/dev/vg_xvgid/brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id "+
						"xfs rw,inode64,noatime,nouuid 1 2\" | "+
						"tee -a /my/fstab > /dev/null", cmd)

			case 4:
				tests.Assert(t,
					cmd == "mount -o rw,inode64,noatime,nouuid "+
						"/dev/vg_xvgid/brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case 5:
				tests.Assert(t,
					cmd == "mkdir "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id/brick", cmd)
			}
		}

		return nil, nil
	}

	// Create Brick
	_, err = s.BrickCreate("myhost", b)
	tests.Assert(t, err == nil, err)

}

func TestSshExecBrickCreateSudo(t *testing.T) {

	f := NewFakeSsh()
	defer tests.Patch(&sshNew,
		func(logger *utils.Logger, user string, file string) (Ssher, error) {
			return f, nil
		}).Restore()

	config := &SshConfig{
		PrivateKeyFile: "xkeyfile",
		User:           "xuser",
		Port:           "100",
		CLICommandConfig: CLICommandConfig{
			Fstab: "/my/fstab",
			Sudo:  true,
		},
	}

	s, err := NewSshExecutor(config)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)

	// Create a Brick
	b := &executors.BrickRequest{
		VgId:             "xvgid",
		Name:             "id",
		TpSize:           100,
		Size:             10,
		PoolMetadataSize: 5,
	}

	// Mock ssh function
	f.FakeConnectAndExec = func(host string,
		commands []string,
		timeoutMinutes int,
		useSudo bool) ([]string, error) {

		tests.Assert(t, host == "myhost:100", host)
		tests.Assert(t, len(commands) == 6)
		tests.Assert(t, useSudo == true)

		for i, cmd := range commands {
			cmd = strings.Trim(cmd, " ")
			switch i {
			case 0:
				tests.Assert(t,
					cmd == "mkdir -p /var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case 1:
				tests.Assert(t,
					cmd == "lvcreate --poolmetadatasize 5K "+
						"-c 256K -L 100K -T vg_xvgid/tp_id -V 10K -n brick_id", cmd)

			case 2:
				tests.Assert(t,
					cmd == "mkfs.xfs -i size=512 "+
						"-n size=8192 /dev/vg_xvgid/brick_id", cmd)

			case 3:
				tests.Assert(t,
					cmd == "echo \"/dev/vg_xvgid/brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id "+
						"xfs rw,inode64,noatime,nouuid 1 2\" | "+
						"tee -a /my/fstab > /dev/null", cmd)

			case 4:
				tests.Assert(t,
					cmd == "mount -o rw,inode64,noatime,nouuid "+
						"/dev/vg_xvgid/brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case 5:
				tests.Assert(t,
					cmd == "mkdir "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id/brick", cmd)
			}
		}

		return nil, nil
	}

	// Create Brick
	_, err = s.BrickCreate("myhost", b)
	tests.Assert(t, err == nil, err)

}

func TestSshExecBrickDestroy(t *testing.T) {

	f := NewFakeSsh()
	defer tests.Patch(&sshNew,
		func(logger *utils.Logger, user string, file string) (Ssher, error) {
			return f, nil
		}).Restore()

	config := &SshConfig{
		PrivateKeyFile: "xkeyfile",
		User:           "xuser",
		Port:           "100",
		CLICommandConfig: CLICommandConfig{
			Fstab: "/my/fstab",
		},
	}

	s, err := NewSshExecutor(config)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)

	// Create a Brick
	b := &executors.BrickRequest{
		VgId:             "xvgid",
		Name:             "id",
		TpSize:           100,
		Size:             10,
		PoolMetadataSize: 5,
	}

	// Mock ssh function
	f.FakeConnectAndExec = func(host string,
		commands []string,
		timeoutMinutes int,
		useSudo bool) ([]string, error) {

		tests.Assert(t, host == "myhost:100", host)

		for _, cmd := range commands {
			cmd = strings.Trim(cmd, " ")
			switch {
			case strings.Contains(cmd, "umount"):
				tests.Assert(t,
					cmd == "umount "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case strings.Contains(cmd, "lvremove"):
				tests.Assert(t,
					cmd == "lvremove -f vg_xvgid/tp_id", cmd)

			case strings.Contains(cmd, "rmdir"):
				tests.Assert(t,
					cmd == "rmdir "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case strings.Contains(cmd, "sed"):
				tests.Assert(t,
					cmd == "sed -i.save "+
						"\"/brick_id/d\" /my/fstab", cmd)
			}
		}

		return nil, nil
	}

	// Create Brick
	err = s.BrickDestroy("myhost", b)
	tests.Assert(t, err == nil, err)
}
