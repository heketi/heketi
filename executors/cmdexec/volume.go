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
	"encoding/xml"
	"fmt"
	"time"

	"github.com/heketi/heketi/executors"
	"github.com/lpabon/godbc"
)

func (s *CmdExecutor) VolumeCreate(host string,
	volume *executors.VolumeRequest) (*executors.Volume, error) {

	godbc.Require(volume != nil)
	godbc.Require(host != "")
	godbc.Require(len(volume.Bricks) > 0)
	godbc.Require(volume.Name != "")

	cmd := fmt.Sprintf("gluster --mode=script volume create %v ", volume.Name)

	var (
		inSet     int
		maxPerSet int
	)
	switch volume.Type {
	case executors.DurabilityNone:
		logger.Info("Creating volume %v with no durability", volume.Name)
		inSet = 1
		maxPerSet = 15
	case executors.DurabilityReplica:
		logger.Info("Creating volume %v replica %v", volume.Name, volume.Replica)
		cmd += fmt.Sprintf("replica %v ", volume.Replica)
		if volume.Arbiter {
			cmd += "arbiter 1 "
		}
		inSet = volume.Replica
		maxPerSet = 5
	case executors.DurabilityDispersion:
		logger.Info("Creating volume %v dispersion %v+%v",
			volume.Name, volume.Data, volume.Redundancy)
		cmd += fmt.Sprintf("disperse-data %v redundancy %v ", volume.Data, volume.Redundancy)
		inSet = volume.Data + volume.Redundancy
		maxPerSet = 1
	}

	// There could many, many bricks, which could render a single command
	// line that creates the volume with all the bricks too long.
	// Therefore, we initially create the volume with the first brick set
	// only, and then add each brick set in one subsequent command.

	for _, brick := range volume.Bricks[:inSet] {
		cmd += fmt.Sprintf("%v:%v ", brick.Host, brick.Path)
	}

	commands := []string{cmd}

	commands = append(commands, s.createAddBrickCommands(volume, inSet, inSet, maxPerSet)...)

	commands = append(commands, s.createVolumeOptionsCommand(volume)...)

	commands = append(commands, fmt.Sprintf("gluster --mode=script volume start %v", volume.Name))

	_, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		s.VolumeDestroy(host, volume.Name)
		return nil, err
	}

	return &executors.Volume{}, nil
}

func (s *CmdExecutor) isRemoveBrickComplete(commands []string, host string) bool {
	type RemoveBrickStatusOutput struct {
		OpRet          int    `xml:"opRet"`
		OpErrno        int    `xml:"opErrno"`
		OpErrStr       string `xml:"opErrstr"`
		VolRemoveBrick struct {
			Aggregate struct {
				StatusStr string `xml:"statusStr"`
			} `xml:"aggregate"`
		} `xml:"volRemoveBrick"`
	}
	statusOutputXml, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		logger.LogError("Unable to check remove-brick status")
		return false
	}
	var statusOutput RemoveBrickStatusOutput
	err = xml.Unmarshal([]byte(statusOutputXml[0]), &statusOutput)
	if err != nil {
		logger.LogError("Unable to determine remove-brick status output")
		return false
	}
	logger.Debug("%+v\n", statusOutput)
	if statusOutput.VolRemoveBrick.Aggregate.StatusStr != "completed" {
		return false
	}

	return true
}

func (s *CmdExecutor) cleanupAddBrick(volume *executors.VolumeRequest, host string, inSet, maxPerSet int) error {
	// If add-brick or rebalance has failed, we have to remove the bricks using the following process
	// 1. gluster volume remove-brick VOLNAME BRICKs start
	// 2. gluster volume remove-brick VOLNAME BRICKs status and grep for all completed
	// 3. gluster volume remove-brick VOLNAME BRICKs commit

	type RemoveBrickStartCommitOutput struct {
		OpRet    int    `xml:"opRet"`
		OpErrno  int    `xml:"opErrno"`
		OpErrStr string `xml:"opErrstr"`
	}
	var removeBrickStatusComplete bool
	commands := s.createRemoveBrickCommands(volume, 0, inSet, maxPerSet, "start --xml")
	startOutputXML, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		return fmt.Errorf("Could not revert add-brick")
	}
	var startOutput RemoveBrickStartCommitOutput
	err = xml.Unmarshal([]byte(startOutputXML[0]), &startOutput)
	if err != nil {
		logger.LogError("Unable to determine remove-brick start output")
		return fmt.Errorf("Could not revert add-brick")
	}
	logger.Debug("%+v\n", startOutput)
	if startOutput.OpRet != 0 {
		logger.LogError("failed to start remove-brick, manual cleanup might be required")
		return fmt.Errorf("Could not revert add-brick")
	}

	commands = s.createRemoveBrickCommands(volume, 0, inSet, maxPerSet, "status --xml")
	ticker := time.NewTicker(time.Second * 10)
	select {
	case <-ticker.C:
		removeBrickStatusComplete = s.isRemoveBrickComplete(commands, host)
		if removeBrickStatusComplete {
			ticker.Stop()
			commands := s.createRemoveBrickCommands(volume, 0, inSet, maxPerSet, "commit --xml")
			commitOutputXML, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
			if err != nil {
				return fmt.Errorf("Could not revert add-brick")
			}
			var commitOutput RemoveBrickStartCommitOutput
			err = xml.Unmarshal([]byte(commitOutputXML[0]), &commitOutput)
			if err != nil {
				logger.LogError("Unable to determine remove-brick commit output")
				return fmt.Errorf("Could not revert add-brick")
			}
			logger.Debug("%+v\n", commitOutput)
			if commitOutput.OpRet != 0 {
				return fmt.Errorf("Could not revert add-brick")
			}
			return nil
		}
		logger.Warning("remove-brick status is not complete yet, will recheck after few seconds")
	case <-time.After(20 * time.Minute):
		logger.LogError("remove-brick status did not complete, manual cleanup might be required")
		ticker.Stop()
		return fmt.Errorf("Could not revert add-brick")
	}

	return fmt.Errorf("Could not revert add-brick")
}

func (s *CmdExecutor) VolumeExpand(host string,
	volume *executors.VolumeRequest) (*executors.Volume, error) {

	godbc.Require(volume != nil)
	godbc.Require(host != "")
	godbc.Require(len(volume.Bricks) > 0)
	godbc.Require(volume.Name != "")

	var (
		inSet     int
		maxPerSet int
	)
	switch volume.Type {
	case executors.DurabilityNone:
		inSet = 1
		maxPerSet = 15
	case executors.DurabilityReplica:
		inSet = volume.Replica
		maxPerSet = 5
	case executors.DurabilityDispersion:
		inSet = volume.Data + volume.Redundancy
		maxPerSet = 1
	}

	commands := s.createAddBrickCommands(volume,
		0, // start at the beginning of the brick list
		inSet,
		maxPerSet)
	_, expandErr := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if expandErr != nil {
		return nil, expandErr
	}

	if s.RemoteExecutor.RebalanceOnExpansion() {
		commands = []string{}
		commands = append(commands, fmt.Sprintf("gluster --mode=script volume rebalance %v start", volume.Name))
		_, expandErr = s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
		if expandErr != nil {
			logger.LogError("Unable to rebalance volume %v:%v", volume.Name, expandErr)
			cleanupErr := s.cleanupAddBrick(volume, host, inSet, maxPerSet)
			if cleanupErr != nil && cleanupErr.Error() == "Could not revert add-brick" {
				logger.LogError("Unable to revert add-brick, initiate rebalance to make use of added bricks on volume %v", volume.Name)
				return &executors.Volume{}, nil
			}
			return nil, expandErr
		}
	}

	return &executors.Volume{}, nil
}

func (s *CmdExecutor) VolumeDestroy(host string, volume string) error {
	godbc.Require(host != "")
	godbc.Require(volume != "")

	// First stop the volume, then delete it

	commands := []string{
		fmt.Sprintf("gluster --mode=script volume stop %v force", volume),
	}

	_, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		logger.LogError("Unable to stop volume %v: %v", volume, err)
	}

	commands = []string{
		fmt.Sprintf("gluster --mode=script volume delete %v", volume),
	}

	_, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		return logger.Err(fmt.Errorf("Unable to delete volume %v: %v", volume, err))
	}

	return nil
}

func (s *CmdExecutor) VolumeDestroyCheck(host, volume string) error {
	godbc.Require(host != "")
	godbc.Require(volume != "")

	// Determine if the volume is able to be deleted
	err := s.checkForSnapshots(host, volume)
	if err != nil {
		return err
	}

	return nil
}

func (s *CmdExecutor) createVolumeOptionsCommand(volume *executors.VolumeRequest) []string {
	commands := []string{}
	var cmd string

	// Go through all the Options and create volume set command
	for _, volOption := range volume.GlusterVolumeOptions {
		if volOption != "" {
			cmd = fmt.Sprintf("gluster --mode=script volume set %v %v", volume.Name, volOption)
			commands = append(commands, cmd)
		}

	}
	return commands
}

func (s *CmdExecutor) createAddBrickCommands(volume *executors.VolumeRequest,
	start, inSet, maxPerSet int) []string {

	commands := []string{}
	var cmd string

	// Go through all the bricks and create add-brick commands
	for index, brick := range volume.Bricks[start:] {
		if index%(inSet*maxPerSet) == 0 {
			if cmd != "" {
				// Add add-brick command to the command list
				commands = append(commands, cmd)
			}

			// Create a new add-brick command
			cmd = fmt.Sprintf("gluster --mode=script volume add-brick %v ", volume.Name)
		}

		// Add this brick to the add-brick command
		cmd += fmt.Sprintf("%v:%v ", brick.Host, brick.Path)
	}

	// Add the last add-brick command to the command list
	if cmd != "" {
		commands = append(commands, cmd)
	}

	return commands
}

func (s *CmdExecutor) createRemoveBrickCommands(volume *executors.VolumeRequest,
	start, inSet, maxPerSet int, verb string) []string {

	commands := []string{}
	var cmd string

	// Go through all the bricks and create remove-brick commands
	for index, brick := range volume.Bricks[start:] {
		if index%(inSet*maxPerSet) == 0 {
			if cmd != "" {
				cmd += fmt.Sprintf("%v", verb)
				commands = append(commands, cmd)
			}

			cmd = fmt.Sprintf("gluster --mode=script volume remove-brick %v ", volume.Name)
		}

		cmd += fmt.Sprintf("%v:%v ", brick.Host, brick.Path)
	}

	// Add the last remove-brick command to the command list
	if cmd != "" {
		cmd += fmt.Sprintf("%v", verb)
		commands = append(commands, cmd)
	}

	return commands
}

func (s *CmdExecutor) checkForSnapshots(host, volume string) error {

	// Structure used to unmarshal XML from snapshot gluster cli
	type CliOutput struct {
		SnapList struct {
			Count int `xml:"count"`
		} `xml:"snapList"`
	}

	commands := []string{
		fmt.Sprintf("gluster --mode=script snapshot list %v --xml", volume),
	}

	output, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		return fmt.Errorf("Unable to get snapshot information from volume %v: %v", volume, err)
	}

	var snapInfo CliOutput
	err = xml.Unmarshal([]byte(output[0]), &snapInfo)
	if err != nil {
		return fmt.Errorf("Unable to determine snapshot information from volume %v: %v", volume, err)
	}

	if snapInfo.SnapList.Count > 0 {
		return fmt.Errorf("Unable to delete volume %v because it contains %v snapshots",
			volume, snapInfo.SnapList.Count)
	}

	return nil
}

func (s *CmdExecutor) VolumeInfo(host string, volume string) (*executors.Volume, error) {

	godbc.Require(volume != "")
	godbc.Require(host != "")

	type CliOutput struct {
		OpRet    int               `xml:"opRet"`
		OpErrno  int               `xml:"opErrno"`
		OpErrStr string            `xml:"opErrstr"`
		VolInfo  executors.VolInfo `xml:"volInfo"`
	}

	command := []string{
		fmt.Sprintf("gluster --mode=script volume info %v --xml", volume),
	}

	//Get the xml output of volume info
	output, err := s.RemoteExecutor.RemoteCommandExecute(host, command, 10)
	if err != nil {
		return nil, fmt.Errorf("Unable to get volume info of volume name: %v", volume)
	}
	var volumeInfo CliOutput
	err = xml.Unmarshal([]byte(output[0]), &volumeInfo)
	if err != nil {
		return nil, fmt.Errorf("Unable to determine volume info of volume name: %v", volume)
	}
	logger.Debug("%+v\n", volumeInfo)
	return &volumeInfo.VolInfo.Volumes.VolumeList[0], nil
}

func (s *CmdExecutor) VolumeReplaceBrick(host string, volume string, oldBrick *executors.BrickInfo, newBrick *executors.BrickInfo) error {
	godbc.Require(volume != "")
	godbc.Require(host != "")
	godbc.Require(oldBrick != nil)
	godbc.Require(newBrick != nil)

	// Replace the brick
	command := []string{
		fmt.Sprintf("gluster --mode=script volume replace-brick %v %v:%v %v:%v commit force", volume, oldBrick.Host, oldBrick.Path, newBrick.Host, newBrick.Path),
	}
	_, err := s.RemoteExecutor.RemoteCommandExecute(host, command, 10)
	if err != nil {
		return logger.Err(fmt.Errorf("Unable to replace brick %v:%v with %v:%v for volume %v", oldBrick.Host, oldBrick.Path, newBrick.Host, newBrick.Path, volume))
	}

	return nil

}

func (s *CmdExecutor) HealInfo(host string, volume string) (*executors.HealInfo, error) {

	godbc.Require(volume != "")
	godbc.Require(host != "")

	type CliOutput struct {
		OpRet    int                `xml:"opRet"`
		OpErrno  int                `xml:"opErrno"`
		OpErrStr string             `xml:"opErrstr"`
		HealInfo executors.HealInfo `xml:"healInfo"`
	}

	command := []string{
		fmt.Sprintf("gluster --mode=script volume heal %v info --xml", volume),
	}

	output, err := s.RemoteExecutor.RemoteCommandExecute(host, command, 10)
	if err != nil {
		return nil, fmt.Errorf("Unable to get heal info of volume : %v", volume)
	}
	var healInfo CliOutput
	err = xml.Unmarshal([]byte(output[0]), &healInfo)
	if err != nil {
		return nil, fmt.Errorf("Unable to determine heal info of volume : %v", volume)
	}
	logger.Debug("%+v\n", healInfo)
	return &healInfo.HealInfo, nil
}
