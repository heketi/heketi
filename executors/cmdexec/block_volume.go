//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmdexec

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/lpabon/godbc"

	"github.com/heketi/heketi/v10/executors"
	rex "github.com/heketi/heketi/v10/pkg/remoteexec"
)

func (s *CmdExecutor) BlockVolumeCreate(host string,
	volume *executors.BlockVolumeRequest) (*executors.BlockVolumeInfo, error) {

	godbc.Require(volume != nil)
	godbc.Require(host != "")
	godbc.Require(volume.Name != "")

	type CliOutput struct {
		Iqn      string   `json:"IQN"`
		Username string   `json:"USERNAME"`
		Password string   `json:"PASSWORD"`
		Portal   []string `json:"PORTAL(S)"`
		Result   string   `json:"RESULT"`
		ErrCode  int      `json:"errCode"`
		ErrMsg   string   `json:"errMsg"`
	}

	var auth_set string
	if volume.Auth {
		auth_set = "enable"
	} else {
		auth_set = "disable"
	}

	cmd := rex.ToCmd(fmt.Sprintf(
		"gluster-block create %v/%v ha %v auth %v prealloc %v %v %vGiB --json",
		volume.GlusterVolumeName,
		volume.Name,
		volume.Hacount,
		auth_set,
		s.BlockVolumeDefaultPrealloc(),
		strings.Join(volume.BlockHosts, ","),
		volume.Size))

	if volume.Auth {
		// Do not write the stdout and stderr to logfile, this might contain sensitive data
		cmd.Options.Quiet = true
	}

	// Execute command
	results, err := s.RemoteExecutor.ExecCommands(host, rex.Cmds{cmd}, 10)
	if err != nil {
		return nil, err
	}

	output := results[0].Output
	if output == "" {
		output = results[0].ErrOutput
	}

	var blockVolumeCreate CliOutput
	err = json.Unmarshal([]byte(output), &blockVolumeCreate)
	if err != nil {
		logger.Warning("Unable to parse gluster-block output [%v]: %v",
			output, err)
		err = fmt.Errorf(
			"Unparsable error during block volume create: %v",
			output)
	} else if blockVolumeCreate.Result == "FAIL" {
		// the fail flag was set in the output json
		err = fmt.Errorf("Failed to create block volume: %v",
			blockVolumeCreate.ErrMsg)
	} else if !results.Ok() {
		// the fail flag is not set but the command still
		// exited non-zero for some reason
		err = fmt.Errorf("Failed to create block volume: %v",
			results[0].Error())
	}

	// if any of the cases above set err, log it and return
	if err != nil {
		logger.LogError("%v", err)
		return nil, err
	}

	var blockVolumeInfo executors.BlockVolumeInfo

	blockVolumeInfo.BlockHosts = volume.BlockHosts // TODO: split blockVolumeCreate.Portal into here instead of using request data
	blockVolumeInfo.GlusterNode = volume.GlusterNode
	blockVolumeInfo.GlusterVolumeName = volume.GlusterVolumeName
	blockVolumeInfo.Hacount = volume.Hacount
	blockVolumeInfo.Iqn = blockVolumeCreate.Iqn
	blockVolumeInfo.Name = volume.Name
	blockVolumeInfo.Size = volume.Size
	blockVolumeInfo.Username = blockVolumeCreate.Username
	blockVolumeInfo.Password = blockVolumeCreate.Password

	logger.Info("{'IQN': '%v', 'USERNAME': '%v', 'AUTH': '%v', 'PORTAL(S)': [ '%v' ], 'RESULT': '%v', 'errCode': %v, 'errMsg': '%v' }",
		blockVolumeCreate.Iqn, blockVolumeCreate.Username, auth_set,
		blockVolumeCreate.Portal, blockVolumeCreate.Result,
		blockVolumeCreate.ErrCode, blockVolumeCreate.ErrMsg)

	return &blockVolumeInfo, nil
}

func (s *CmdExecutor) BlockVolumeDestroy(host string, blockHostingVolumeName string, blockVolumeName string) error {
	godbc.Require(host != "")
	godbc.Require(blockHostingVolumeName != "")
	godbc.Require(blockVolumeName != "")

	commands := []string{
		fmt.Sprintf("gluster-block delete %v/%v --json",
			blockHostingVolumeName, blockVolumeName),
	}
	res, err := s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands), 10)
	if err != nil {
		// non-command error conditions
		return err
	}

	r := res[0]
	errOutput := r.ErrOutput
	if errOutput == "" {
		errOutput = r.Output
	}
	if errOutput == "" {
		// we ought to have some output but we don't
		return r.Err
	}

	type CliOutput struct {
		Result       string `json:"RESULT"`
		ResultOnHost string `json:"Result"`
		ErrCode      int    `json:"errCode"`
		ErrMsg       string `json:"errMsg"`
	}
	var blockVolumeDelete CliOutput
	if e := json.Unmarshal([]byte(errOutput), &blockVolumeDelete); e != nil {
		logger.LogError("Failed to unmarshal response from block "+
			"volume delete for volume %v", blockVolumeName)
		if r.Err != nil {
			return logger.Err(r.Err)
		}

		return logger.LogError("Unable to parse output from block "+
			"volume delete: %v", e)
	}

	if blockVolumeDelete.Result == "FAIL" {
		errHas := func(s string) bool {
			return strings.Contains(blockVolumeDelete.ErrMsg, s)
		}

		if (errHas("doesn't exist") && errHas(blockVolumeName)) ||
			(errHas("does not exist") && errHas(blockHostingVolumeName)) {
			return &executors.VolumeDoesNotExistErr{Name: blockVolumeName}
		}
		return logger.LogError("%v", blockVolumeDelete.ErrMsg)
	}
	return r.Err
}

func (c *CmdExecutor) ListBlockVolumes(host string, blockhostingvolume string) ([]string, error) {
	godbc.Require(host != "")
	godbc.Require(blockhostingvolume != "")

	commands := []string{fmt.Sprintf("gluster-block list %v --json", blockhostingvolume)}

	results, err := c.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands), 10)
	if err := rex.AnyError(results, err); err != nil {
		logger.Err(err)
		return nil, fmt.Errorf("unable to list blockvolumes on block hosting volume %v : %v", blockhostingvolume, err)
	}

	type BlockVolumeListOutput struct {
		Blocks []string `json:"blocks"`
		RESULT string   `json:"RESULT"`
	}

	var blockVolumeList BlockVolumeListOutput

	err = json.Unmarshal([]byte(results[0].Output), &blockVolumeList)
	if err != nil {
		logger.Err(err)
		return nil, fmt.Errorf("Unable to get the block volume list for block hosting volume %v : %v", blockhostingvolume, err)
	}

	return blockVolumeList.Blocks, nil
}

const (
	GIGABYTE = 1 << (10 * iota)
	TERABYTE
	PETABYTE
	EXABYTE
)

func toGigaBytes(s string) (int, error) {
	s = strings.Replace(s, " ", "", -1)
	s = strings.ToUpper(s)

	i := strings.IndexFunc(s, unicode.IsLetter)
	if i == -1 {
		return 0, fmt.Errorf("Not a valid Size")
	}

	sizeNumber, unitsString := s[:i], s[i:]
	bytes, err := strconv.ParseFloat(sizeNumber, 64)
	if err != nil || bytes < 0 {
		return 0, fmt.Errorf("Not a valid Size")
	}

	// heketi always requests size in integer and in GiB
	switch unitsString {
	case "E", "EB", "EIB":
		return int(bytes * EXABYTE), nil
	case "P", "PB", "PIB":
		return int(bytes * PETABYTE), nil
	case "T", "TB", "TIB":
		return int(bytes * TERABYTE), nil
	case "G", "GB", "GIB":
		return int(bytes), nil
	default:
		return 0, fmt.Errorf("Not a valid Size")
	}
}

func (c *CmdExecutor) BlockVolumeInfo(host string, blockhostingvolume string,
	blockVolumeName string) (*executors.BlockVolumeInfo, error) {

	godbc.Require(host != "")
	godbc.Require(blockhostingvolume != "")
	godbc.Require(blockVolumeName != "")

	type CliOutput struct {
		BlockName              string            `json:"NAME"`
		BlockHostingVolumeName string            `json:"VOLUME"`
		Gbid                   string            `json:"GBID"`
		Size                   string            `json:"SIZE"`
		Ha                     int               `json:"HA"`
		Password               string            `json:"PASSWORD"`
		Portal                 []string          `json:"EXPORTED ON"`
		ResizeFailed           map[string]string `json:"RESIZE FAILED ON,omitempty"`
	}

	cmd := rex.ToCmd(fmt.Sprintf("gluster-block info %v/%v --json", blockhostingvolume, blockVolumeName))
	// Do not write the stdout and stderr to logfile, this might contain sensitive data
	cmd.Options.Quiet = true

	results, err := c.RemoteExecutor.ExecCommands(host, rex.Cmds{cmd}, 10)
	if err := rex.AnyError(results, err); err != nil {
		logger.Err(err)
		return nil, fmt.Errorf("unable to get info of blockvolume %v on block hosting volume %v : %v",
			blockVolumeName, blockhostingvolume, err)
	}

	output := results[0].Output
	if output == "" {
		output = results[0].ErrOutput
	}

	var blockVolumeInfoExec CliOutput
	err = json.Unmarshal([]byte(output), &blockVolumeInfoExec)
	if err != nil {
		logger.Warning("Unable to parse gluster-block info output [%v]: %v",
			output, err)
		err = fmt.Errorf("Unparsable error during block volume info: %v",
			output)
	}

	// if any of the cases above set err, log it and return
	if err != nil {
		logger.LogError("Failed BlockVolumeInfo: %v", err)
		return nil, err
	}

	var blockVolumeInfo executors.BlockVolumeInfo

	blockVolumeInfo.BlockHosts = blockVolumeInfoExec.Portal
	blockVolumeInfo.GlusterNode = host
	blockVolumeInfo.GlusterVolumeName = blockVolumeInfoExec.BlockHostingVolumeName
	blockVolumeInfo.Hacount = blockVolumeInfoExec.Ha
	blockVolumeInfo.Iqn = "iqn.2016-12.org.gluster-block:" + blockVolumeInfoExec.Gbid
	blockVolumeInfo.Name = blockVolumeInfoExec.BlockName
	blockVolumeInfo.Size, err = toGigaBytes(blockVolumeInfoExec.Size)
	if err != nil {
		logger.LogError("Error: %v : %v", err, blockVolumeInfoExec.Size)
		return nil, err
	}
	blockVolumeInfo.Username = blockVolumeInfoExec.Gbid
	blockVolumeInfo.Password = blockVolumeInfoExec.Password

	auth_set := "enable"
	if blockVolumeInfoExec.Password == "" {
		auth_set = "disable"
	}

	var resizeFailed string
	if len(blockVolumeInfoExec.ResizeFailed) != 0 {
		for key, value := range blockVolumeInfoExec.ResizeFailed {
			resizeFailed += fmt.Sprintf("'%v' : '%v', ", key, value)
		}
	}

	logger.Info("{ 'NAME': '%v', 'VOLUME': '%v', 'GBID': '%v', 'SIZE': '%v', 'HA': %v, 'AUTH': '%v', 'EXPORTED ON': [ '%v' ], 'RESIZE FAILED ON': { %v } }",
		blockVolumeInfoExec.BlockName, blockVolumeInfoExec.BlockHostingVolumeName,
		blockVolumeInfoExec.Gbid, blockVolumeInfoExec.Size, blockVolumeInfoExec.Ha,
		auth_set, blockVolumeInfoExec.Portal, resizeFailed)

	// From gluster-block output,
	// get the Minimum Size from the list of Resize failed nodes
	minSize := -1
	for _, size := range blockVolumeInfoExec.ResizeFailed {
		value, err := toGigaBytes(size)
		if err != nil {
			logger.LogError("Error calculating usable size: %v : %v", err, size)
			return nil, err
		}
		if minSize == -1 || value < minSize {
			minSize = value
		}
	}
	if minSize != -1 {
		blockVolumeInfo.UsableSize = minSize
	}

	return &blockVolumeInfo, nil
}

func (s *CmdExecutor) BlockVolumeExpand(host string, blockHostingVolumeName string, blockVolumeName string, newSize int) error {
	godbc.Require(host != "")
	godbc.Require(blockHostingVolumeName != "")
	godbc.Require(blockVolumeName != "")
	godbc.Require(newSize > 0)

	commands := []string{
		fmt.Sprintf("gluster-block modify %v/%v  size %vGiB --json",
			blockHostingVolumeName, blockVolumeName, newSize),
	}
	results, err := s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands), 10)
	if err != nil {
		// non-command error conditions
		return err
	}

	output := results[0].Output
	if output == "" {
		output = results[0].ErrOutput
	}

	type CliOutput struct {
		Iqn       string   `json:"IQN"`
		Size      string   `json:"SIZE"`
		SuccessOn []string `json:"SUCCESSFUL ON"`
		FailedOn  []string `json:"FAILED ON"`
		SkippedOn []string `json:"SKIPPED ON"`
		Result    string   `json:"RESULT"`
		ErrCod    int      `json:"errCode"`
		ErrMsg    string   `json:"errMsg"`
	}

	var blockVolumeExpand CliOutput
	err = json.Unmarshal([]byte(output), &blockVolumeExpand)
	if err != nil {
		logger.Warning("Unable to parse gluster-block output [%v]: %v",
			output, err)
		err = fmt.Errorf("Unparsable error during block volume expand: %v",
			output)
	} else if blockVolumeExpand.Result == "FAIL" {
		// the fail flag was set in the output json
		if blockVolumeExpand.ErrMsg != "" {
			err = fmt.Errorf("Failed to expand block volume: %v (see logs for details, and retry the operation)", blockVolumeExpand.ErrMsg)
		} else {
			err = fmt.Errorf("Failed to expand block volume (see logs for details, and retry the operation)")
		}
	} else if !results.Ok() {
		// the fail flag is not set but the command still
		// exited non-zero for some reason
		err = fmt.Errorf("Failed to expand block volume: %v",
			results[0].Error())
	}

	// if any of the cases above set err, log it and return
	if err != nil {
		logger.LogError("Failed BlockVolumeExpand: %v", err)
		return err
	}

	return nil
}
