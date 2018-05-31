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
	"strconv"
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

	// make local vars with more accurate names to cut down on name confusion
	// and make future refactoring easier
	brickPath := brick.Path
	mountPath := utils.BrickMountFromPath(brickPath)

	// Create command set to execute on the node
	devnode := utils.BrickDevNode(brick.VgId, brick.Name)
	commands := []string{

		// Create a directory
		fmt.Sprintf("mkdir -p %v", mountPath),

		// Setup the LV
		fmt.Sprintf("lvcreate --autobackup=%v --poolmetadatasize %vK --chunksize 256K --size %vK --thin %v/%v --virtualsize %vK --name %v",
			// backup LVM metadata
			utils.BoolToYN(s.BackupLVM),

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
		fmt.Sprintf("awk \"BEGIN {print \\\"%v %v xfs rw,inode64,noatime,nouuid 1 2\\\" >> \\\"%v\\\"}\"",
			devnode,
			mountPath,
			s.Fstab),

		// Mount
		fmt.Sprintf("mount -o rw,inode64,noatime,nouuid %v %v", devnode, mountPath),

		// Create a directory inside the formated volume for GlusterFS
		fmt.Sprintf("mkdir %v", brickPath),
	}

	// Only set the GID if the value is other than root(gid 0).
	// When no gid is set, root is the only one that can write to the volume
	if 0 != brick.Gid {
		commands = append(commands, []string{
			// Set GID on brick
			fmt.Sprintf("chown :%v %v", brick.Gid, brickPath),

			// Set writable by GID and UID
			fmt.Sprintf("chmod 2775 %v", brickPath),
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
		Path: brickPath,
	}
	return b, nil
}

func (s *CmdExecutor) BrickDestroy(host string,
	brick *executors.BrickRequest) (bool, error) {

	godbc.Require(brick != nil)
	godbc.Require(host != "")
	godbc.Require(brick.Name != "")
	godbc.Require(brick.VgId != "")
	godbc.Require(brick.Path != "")

	var (
		umountErr      error
		spaceReclaimed bool
	)

	// Cloned bricks do not follow 'our' VG/LV naming, detect it.
	commands := []string{
		fmt.Sprintf("mount | grep -w %v | cut -d\" \" -f1", brick.Path),
	}
	output, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if output == nil || err != nil {
		return spaceReclaimed, fmt.Errorf("No brick mounted on %v, unable to proceed with removing", brick.Path)
	}
	dev := output[0]
	// detect the thinp LV used by this brick (in "vg_.../tp_..." format)
	commands = []string{
		fmt.Sprintf("lvs --noheadings --separator=/ -ovg_name,pool_lv %v", dev),
	}
	output, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}
	tp := output[0]

	// Try to unmount first
	commands = []string{
		fmt.Sprintf("umount %v", brick.Path),
	}
	_, umountErr = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if umountErr != nil {
		logger.Err(err)
	}

	// remove brick from fstab before we start deleting LVM items.
	// if heketi or the node was terminated while these steps are being
	// performed we'll orphan storage but the node should still be
	// bootable. If we remove LVM stuff first but leave an entry in
	// fstab referencing it, we could end up with a non-booting system.
	// Even if we failed to umount the brick, remove it from fstab
	// so that it does not get mounted again on next reboot.
	err = s.removeBrickFromFstab(host, brick)

	// if either umount or fstab remove failed there's no point in
	// continuing. We'll need either automated or manual recovery
	// in the future, but we need to know something went wrong.
	if err != nil {
		logger.Err(err)
		return spaceReclaimed, err
	}
	if umountErr != nil {
		return spaceReclaimed, umountErr
	}

	// Remove the LV (by device name)
	commands = []string{
		fmt.Sprintf("lvremove --autobackup=%v -f %v", utils.BoolToYN(s.BackupLVM), dev),
	}
	_, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
	} else {
		// no space freed when tp sticks around
		spaceReclaimed = false
	}

	// Detect the number of bricks using the thin-pool
	commands = []string{
		fmt.Sprintf("lvs --noheadings --options=thin_count %v", tp),
	}
	output, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
		return spaceReclaimed, fmt.Errorf("Unable to determine number of logical volumes in "+
			"thin pool %v on host %v", tp, host)
	}
	thin_count, err := strconv.Atoi(strings.TrimSpace(output[0]))
	if err != nil {
		return spaceReclaimed, fmt.Errorf("Failed to convert number of logical volumes in thin pool %v on host %v: %v", tp, host, err)
	}

	// If there is no brick left in the thin-pool, it can be removed
	if thin_count == 0 {
		commands = []string{
			fmt.Sprintf("lvremove --autobackup=%v -f %v", utils.BoolToYN(s.BackupLVM), tp),
		}
		_, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
		if err != nil {
			logger.Err(err)
		} else {
			spaceReclaimed = true
		}
	}

	// Now cleanup the mount point
	commands = []string{
		fmt.Sprintf("rmdir %v", brick.Path),
	}
	_, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}

	return spaceReclaimed, nil
}

func (s *CmdExecutor) removeBrickFromFstab(
	host string, brick *executors.BrickRequest) error {

	// If the brick.Path contains "(/var)?/run/gluster/", there is no entry in fstab as GlusterD manages it.
	if strings.HasPrefix(brick.Path, "/run/gluster/") || strings.HasPrefix(brick.Path, "/var/run/gluster/") {
		return nil
	}
	commands := []string{
		fmt.Sprintf("sed -i.save \"/%v/d\" %v",
			utils.BrickIdToName(brick.Name),
			s.Fstab),
	}
	_, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}
	return err
}
