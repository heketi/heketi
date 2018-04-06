//
// Copyright (c) 2016 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmdexec

import (
	"strings"
	"testing"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

func TestSshExecBrickCreate(t *testing.T) {
	f := NewCommandFaker()
	s, err := NewFakeExecutor(f)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	s.portStr = "100"

	// Create a Brick
	b := &executors.BrickRequest{
		VgId:             "xvgid",
		Name:             "id",
		TpSize:           100,
		Size:             10,
		PoolMetadataSize: 5,
		Path:             utils.BrickPath("xvgid", "id"),
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
						"-n size=8192 /dev/mapper/vg_xvgid-brick_id", cmd)

			case 3:
				tests.Assert(t,
					cmd == "awk \"BEGIN {print \\\"/dev/mapper/vg_xvgid-brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id "+
						"xfs rw,inode64,noatime,nouuid 1 2\\\" "+
						">> \\\"/my/fstab\\\"}\"", cmd)

			case 4:
				tests.Assert(t,
					cmd == "mount -o rw,inode64,noatime,nouuid "+
						"/dev/mapper/vg_xvgid-brick_id "+
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

func TestSshExecBrickCreateWithGid(t *testing.T) {
	f := NewCommandFaker()
	s, err := NewFakeExecutor(f)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	s.portStr = "100"

	// Create a Brick
	b := &executors.BrickRequest{
		VgId:             "xvgid",
		Name:             "id",
		TpSize:           100,
		Size:             10,
		PoolMetadataSize: 5,
		Gid:              1234,
		Path:             utils.BrickPath("xvgid", "id"),
	}

	// Mock ssh function
	f.FakeConnectAndExec = func(host string,
		commands []string,
		timeoutMinutes int,
		useSudo bool) ([]string, error) {

		tests.Assert(t, host == "myhost:100", host)
		tests.Assert(t, len(commands) == 8)

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
						"-n size=8192 /dev/mapper/vg_xvgid-brick_id", cmd)

			case 3:
				tests.Assert(t,
					cmd == "awk \"BEGIN {print \\\"/dev/mapper/vg_xvgid-brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id "+
						"xfs rw,inode64,noatime,nouuid 1 2\\\" "+
						">> \\\"/my/fstab\\\"}\"", cmd)

			case 4:
				tests.Assert(t,
					cmd == "mount -o rw,inode64,noatime,nouuid "+
						"/dev/mapper/vg_xvgid-brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case 5:
				tests.Assert(t,
					cmd == "mkdir "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id/brick", cmd)

			case 6:
				tests.Assert(t,
					cmd == "chown :1234 "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id/brick", cmd)

			case 7:
				tests.Assert(t,
					cmd == "chmod 2775 "+
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
	f := NewCommandFaker()
	s, err := NewFakeExecutor(f)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	s.useSudo = true
	s.portStr = "100"

	// Create a Brick
	b := &executors.BrickRequest{
		VgId:             "xvgid",
		Name:             "id",
		TpSize:           100,
		Size:             10,
		PoolMetadataSize: 5,
		Path:             utils.BrickPath("xvgid", "id"),
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
						"-n size=8192 /dev/mapper/vg_xvgid-brick_id", cmd)

			case 3:
				tests.Assert(t,
					cmd == "awk \"BEGIN {print \\\"/dev/mapper/vg_xvgid-brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id "+
						"xfs rw,inode64,noatime,nouuid 1 2\\\" "+
						">> \\\"/my/fstab\\\"}\"", cmd)

			case 4:
				tests.Assert(t,
					cmd == "mount -o rw,inode64,noatime,nouuid "+
						"/dev/mapper/vg_xvgid-brick_id "+
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
	f := NewCommandFaker()
	s, err := NewFakeExecutor(f)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	s.portStr = "100"

	// Create a Brick
	b := &executors.BrickRequest{
		VgId:             "xvgid",
		Name:             "id",
		TpSize:           100,
		Size:             10,
		PoolMetadataSize: 5,
		Path:             strings.TrimSuffix(utils.BrickPath("xvgid", "id"), "/brick"),
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
			case strings.HasPrefix(cmd, "mount"):
				tests.Assert(t,
					cmd == "mount | grep -w "+b.Path+" | cut -d\" \" -f1", cmd)
				// return the device that was mounted
				output := [2]string{"/dev/vg_xvgid/brick_id", ""}
				return output[0:1], nil

			case strings.Contains(cmd, "lvs") && strings.Contains(cmd, "vg_name"):
				tests.Assert(t,
					cmd == "lvs --noheadings --separator=/ "+
						"-ovg_name,pool_lv /dev/vg_xvgid/brick_id", cmd)
				// return the device that was mounted
				output := [2]string{"vg_xvgid/tp_id", ""}
				return output[0:1], nil

			case strings.Contains(cmd, "lvs") && strings.Contains(cmd, "thin_count"):
				tests.Assert(t,
					cmd == "lvs --noheadings --options=thin_count vg_xvgid/tp_id", cmd)
				// return the number of thin-p users
				output := [2]string{"0", ""}
				return output[0:1], nil

			case strings.Contains(cmd, "umount"):
				tests.Assert(t,
					cmd == "umount "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case strings.Contains(cmd, "lvremove"):
				tests.Assert(t,
					cmd == "lvremove -f vg_xvgid/tp_id" ||
						cmd == "lvremove -f /dev/vg_xvgid/brick_id", cmd)

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
	_, err = s.BrickDestroy("myhost", b)
	tests.Assert(t, err == nil, err)
}
