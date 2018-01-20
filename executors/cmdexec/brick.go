//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmdexec

import (
	"fmt"
	"strings"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/lpabon/godbc"
)

func (s *CmdExecutor) BrickCreate(host string,
	brick *executors.BrickRequest) (*executors.BrickInfo, error) {

	godbc.Require(brick != nil)
	godbc.Require(host != "")
	godbc.Require(brick.Name != "")
	godbc.Require(brick.Size > 0)
	godbc.Require(brick.TpSize >= brick.Size)
	godbc.Require(brick.VgId != "")
	godbc.Require(brick.Path != "")
	godbc.Require(s.Fstab != "")

	// Create command set to execute on the node
	devnode := utils.BrickDevNode(brick.VgId, brick.Name)
	commands := []string{

		// Create a directory
		fmt.Sprintf("mkdir -p %v", brick.Path),

		// Setup the LV
		fmt.Sprintf("lvcreate --poolmetadatasize %vK -c 256K -L %vK -T %v/%v -V %vK -n %v",
			// MetadataSize
			brick.PoolMetadataSize,

			//Thin Pool Size
			brick.TpSize,

			// volume group
			utils.VgIdToName(brick.VgId),

			// ThinP name
			utils.BrickIdToThinPoolName(brick.Name),

			// Allocation size
			brick.Size,

			// Logical Vol name
			utils.BrickIdToName(brick.Name)),

		// Format
		fmt.Sprintf("mkfs.xfs -i size=512 -n size=8192 %v", devnode),

		// Fstab
		fmt.Sprintf("echo \"%v %v xfs rw,inode64,noatime,nouuid 1 2\" | tee -a %v > /dev/null ",
			devnode,
			brick.Path,
			s.Fstab),

		// Mount
		fmt.Sprintf("mount -o rw,inode64,noatime,nouuid %v %v", devnode, brick.Path),

		// Create a directory inside the formated volume for GlusterFS
		fmt.Sprintf("mkdir %v/brick", brick.Path),
	}

	// Only set the GID if the value is other than root(gid 0).
	// When no gid is set, root is the only one that can write to the volume
	if 0 != brick.Gid {
		commands = append(commands, []string{
			// Set GID on brick
			fmt.Sprintf("chown :%v %v/brick", brick.Gid, brick.Path),

			// Set writable by GID and UID
			fmt.Sprintf("chmod 2775 %v/brick", brick.Path),
		}...)
	}

	// Execute commands
	_, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		// Cleanup
		s.BrickDestroy(host, brick)
		return nil, err
	}

	// Save brick location
	b := &executors.BrickInfo{
		Path: fmt.Sprintf("%v/brick", brick.Path),
	}
	return b, nil
}

func (s *CmdExecutor) BrickDestroy(host string,
	brick *executors.BrickRequest) error {

	godbc.Require(brick != nil)
	godbc.Require(host != "")
	godbc.Require(brick.Name != "")
	godbc.Require(brick.VgId != "")

	mp := utils.BrickMountPoint(brick.VgId, brick.Name)
	// Try to unmount first
	commands := []string{
		fmt.Sprintf("umount %v", mp),
	}
	_, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}

	// Now try to remove the LV
	commands = []string{
		fmt.Sprintf("lvremove -f %v", utils.BrickThinLvName(brick.VgId, brick.Name)),
	}
	_, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}

	// Now cleanup the mount point
	commands = []string{
		fmt.Sprintf("rmdir %v", mp),
	}
	_, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}

	// Remove from fstab
	commands = []string{
		fmt.Sprintf("sed -i.save \"/%v/d\" %v",
			utils.BrickIdToName(brick.Name),
			s.Fstab),
	}
	_, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}

	return nil
}

func (s *CmdExecutor) BrickDestroyCheck(host string,
	brick *executors.BrickRequest) error {
	godbc.Require(brick != nil)
	godbc.Require(host != "")
	godbc.Require(brick.Name != "")
	godbc.Require(brick.VgId != "")

	err := s.checkThinPoolUsage(host, brick)
	if err != nil {
		return err
	}

	return nil
}

// Determine if any other logical volumes are using the thin pool.
// If they are, then either a clone volume or a snapshot is using that storage,
// and we cannot delete the brick.
func (s *CmdExecutor) checkThinPoolUsage(host string,
	brick *executors.BrickRequest) error {

	// Sample output:
	// 		# lvs --options=lv_name,thin_count --separator=: | grep "tp_"
	// 		tp_a17c621ade79017b48cc0042bea86510:2
	// 		tp_8d4e0849a5c90608a543928961bd2387:1
	//		tp_3b9b3e07f06b93d94006ef272d3c10eb:2

	tp := utils.BrickIdToThinPoolName(brick.Name)
	commands := []string{
		fmt.Sprintf("lvs --options=lv_name,thin_count --separator=:"),
	}

	// Send command
	output, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
		return fmt.Errorf("Unable to determine number of logical volumes in "+
			"thin pool %v on host %v", tp, host)
	}

	// Determine if do not have only one LV in the thin pool,
	// we cannot delete the brick
	lvs := strings.Index(output[0], tp+":1")
	if lvs == -1 {
		return fmt.Errorf("Cannot delete thin pool %v on %v because it "+
			"is used by [%v] snapshot(s) or cloned volume(s)",
			tp,
			host,
			lvs)
	}

	return nil
}
