package sshexec

import (
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/heketi/heketi/executors"
	"github.com/lpabon/godbc"
)

// GeoReplicationCreate creates a geo-rep session for the given volume
func (s *SshExecutor) GeoReplicationCreate(host, volume string, geoRep *executors.GeoReplicationRequest) error {
	logger.Debug("In GeoReplicationCreate")
	logger.Debug("actionParams: %+v", geoRep.ActionParams)

	godbc.Require(host != "")
	godbc.Require(volume != "")
	godbc.Require(geoRep.SlaveHost != "")
	godbc.Require(geoRep.SlaveVolume != "")
	_, optionOK := geoRep.ActionParams["option"]
	godbc.Require(optionOK && (geoRep.ActionParams["option"] == "push-pem" || geoRep.ActionParams["option"] == "no-verify"))

	sshPort := " "
	if geoRep.SlaveSSHPort != 0 {
		sshPort = fmt.Sprintf(" ssh-port %d ", geoRep.SlaveSSHPort)
	}
	cmd := fmt.Sprintf("gluster --mode=script volume geo-replication %s %s::%s create%s%s", volume, geoRep.SlaveHost, geoRep.SlaveVolume, sshPort, geoRep.ActionParams["option"])

	if force, ok := geoRep.ActionParams["force"]; ok && force == "true" {
		cmd = fmt.Sprintf("%s %s", cmd, force)
	}

	commands := []string{cmd}
	if _, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10); err != nil {
		return err
	}

	return nil
}

// GeoReplicationAction executes the given geo-replication action for the given volume
func (s *SshExecutor) GeoReplicationAction(host, volume, action string, geoRep *executors.GeoReplicationRequest) error {
	logger.Debug("In GeoReplicationAction: %s", action)

	godbc.Require(host != "")
	godbc.Require(volume != "")
	godbc.Require(geoRep.SlaveHost != "")
	godbc.Require(geoRep.SlaveVolume != "")

	cmd := fmt.Sprintf("gluster --mode=script volume geo-replication %s %s::%s %s", volume, geoRep.SlaveHost, geoRep.SlaveVolume, action)

	if force, ok := geoRep.ActionParams["force"]; ok && force == "true" {
		cmd = fmt.Sprintf("%s %s", cmd, force)
	}

	commands := []string{cmd}
	if _, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10); err != nil {
		return err
	}

	return nil
}

// GeoReplicationStatus returns the geo-replication status
func (s *SshExecutor) GeoReplicationStatus(host string) (*executors.GeoReplicationStatus, error) {
	logger.Debug("In GeoReplicationStatus")

	godbc.Require(host != "")

	type CliOutput struct {
		OpRet        int                            `xml:"opRet"`
		OpErrno      int                            `xml:"opErrno"`
		OpErrStr     string                         `xml:"opErrstr"`
		GeoRepStatus executors.GeoReplicationStatus `xml:"geoRep"`
	}

	commands := []string{"gluster --mode=script volume geo-replication status --xml"}

	var output []string
	var err error
	if output, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 10); err != nil {
		return nil, err
	}

	var geoRepStatus CliOutput

	if err := xml.Unmarshal([]byte(output[0]), &geoRepStatus); err != nil {
		return nil, fmt.Errorf("Unable to determine geo-replication status on host %s: %v", host, err)
	}

	return &geoRepStatus.GeoRepStatus, nil
}

// GeoReplicationVolumeStatus returns the geo-replication status of a specific volume
func (s *SshExecutor) GeoReplicationVolumeStatus(host, volume string) (*executors.GeoReplicationStatus, error) {
	logger.Debug("In GeoReplicationVolumeStatus")

	godbc.Require(host != "")
	godbc.Require(volume != "")

	type CliOutput struct {
		OpRet        int                            `xml:"opRet"`
		OpErrno      int                            `xml:"opErrno"`
		OpErrStr     string                         `xml:"opErrstr"`
		GeoRepStatus executors.GeoReplicationStatus `xml:"geoRep"`
	}

	cmd := fmt.Sprintf("gluster --mode=script volume geo-replication %s status --xml", volume)
	commands := []string{cmd}

	var output []string
	var err error
	if output, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 10); err != nil {
		return nil, err
	}

	var geoRepStatus CliOutput

	if err := xml.Unmarshal([]byte(output[0]), &geoRepStatus); err != nil {
		return nil, fmt.Errorf("Unable to determine geo-replication status for volume %v: %v", volume, err)
	}

	return &geoRepStatus.GeoRepStatus, nil
}

// GeoReplicationConfig configures the geo-replication session for the given volume
func (s *SshExecutor) GeoReplicationConfig(host, volume string, geoRep *executors.GeoReplicationRequest) error {
	logger.Debug("In GeoReplicationConfig")

	godbc.Require(host != "")
	godbc.Require(volume != "")
	godbc.Require(geoRep.SlaveHost != "")
	godbc.Require(geoRep.SlaveVolume != "")

	commands := s.createConfigCommands(volume, geoRep)

	if _, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10); err != nil {
		logger.LogError("Invalid configuration for volume georeplication %s", volume)
		return err
	}
	return nil
}

func (s *SshExecutor) createConfigCommands(volume string, geoRep *executors.GeoReplicationRequest) []string {
	commands := []string{}

	cmdTpl := "gluster --mode=script volume geo-replication %s %s::%s config %s %s"
	for param, value := range geoRep.ActionParams {
		switch param {
		// String parameters
		case "log-level", "gluster-log-level", "changelog-log-level", "ssh-command", "rsync-command":
			commands = append(commands, fmt.Sprintf(cmdTpl, volume, geoRep.SlaveHost, geoRep.SlaveVolume, param, value))
		// Boolean parameters
		case "use-tarssh", "use-meta-volume":
			if value != "false" && value != "true" {
				logger.LogError("Invalid value %v for config option %s", value, param)
				continue
			}
			commands = append(commands, fmt.Sprintf(cmdTpl, volume, geoRep.SlaveHost, geoRep.SlaveVolume, param, value))
		case "ignore-deletes":
			if value != "false" && value != "true" {
				logger.LogError("Invalid value %v for config option %s", value, param)
				continue
			}

			// set to 1 if explicitly set to true, skip otherwise
			if value == "true" {
				commands = append(commands, fmt.Sprintf(cmdTpl, volume, geoRep.SlaveHost, geoRep.SlaveVolume, param, "1"))
			}
		// Integer parameters
		case "timeout", "sync-jobs":
			if _, err := strconv.Atoi(value); err != nil {
				logger.LogError("Invalid value %v for config option %s", value, param)
				continue
			}
			commands = append(commands, fmt.Sprintf(cmdTpl, volume, geoRep.SlaveHost, geoRep.SlaveVolume, param, value))
		case "ssh-port":
			// due to gluster cli client inconsistency, set the parameter to ssh_port
			param = "ssh_port"
			if _, err := strconv.Atoi(value); err != nil {
				logger.LogError("Invalid value %v for config option %s", value, param)
				continue
			}
			commands = append(commands, fmt.Sprintf(cmdTpl, volume, geoRep.SlaveHost, geoRep.SlaveVolume, param, value))
		}

	}

	return commands
}
