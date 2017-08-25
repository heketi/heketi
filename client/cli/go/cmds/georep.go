package cmds

import (
	"encoding/json"
	"errors"
	"fmt"

	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/spf13/cobra"
)

var (
	slaveHost, slaveVolume string
	volumeID               string
)

func initGeoRepCommand() {
	RootCmd.AddCommand(geoReplicationCommand)
	geoReplicationCommand.AddCommand(geoReplicationStatusCommand)
	volumeCommand.AddCommand(geoReplicationVolumeCommand)
	geoReplicationVolumeCommand.AddCommand(
		geoReplicationCreateCommand,
		geoReplicationVolumeStatusCommand,
		geoReplicationDeleteCommand,
		geoReplicationConfigCommand,
		geoReplicationStartCommand,
		geoReplicationStopCommand,
		geoReplicationPauseCommand,
		geoReplicationResumeCommand,
	)

	// Flags
	geoReplicationVolumeCommand.PersistentFlags().StringVar(&slaveHost, "slave-host", "", "The host of the slave volume")
	geoReplicationVolumeCommand.PersistentFlags().StringVar(&slaveVolume, "slave-volume", "", "The volume name of the geo-replication target")
	geoReplicationVolumeCommand.PersistentFlags().Bool("force", false, "\n\tForce creation")
	geoReplicationCreateCommand.Flags().String("option", "no-verify", "\n\tSet option for create command")
	geoReplicationCreateCommand.Flags().Int("ssh-port", 0, "\n\tThe gluster SSH port on the slave host")
	geoReplicationConfigCommand.Flags().Int("sync-jobs", 0, "\n\tConfigure sync-jobs option for the specific volume")
	geoReplicationConfigCommand.Flags().Int("timeout", 0, "\n\tConfigure timeout option for the specific volume")
	geoReplicationConfigCommand.Flags().Int("ssh-port", 0, "\n\tConfigure ssh_port option for the specific volume")
	geoReplicationConfigCommand.Flags().Bool("use-tarssh", false, "\n\tConfigure use-tarssh option for the specific volume")
	geoReplicationConfigCommand.Flags().Bool("use-meta-volume", false, "\n\tConfigure use-meta-volume option for the specific volume")
	geoReplicationConfigCommand.Flags().Bool("ignore-deletes", false, "\n\tConfigure ignore-deletes for the specific volume (to 1 if true)")
	geoReplicationConfigCommand.Flags().String("log-level", "", "\n\tConfigure log-level for the specific volume")
	geoReplicationConfigCommand.Flags().String("gluster-log-level", "", "\n\tConfigure gluster-log-level for the specific volume")
	geoReplicationConfigCommand.Flags().String("changelog-log-level", "", "\n\tConfigure changelog-log-level for the specific volume")
	geoReplicationConfigCommand.Flags().String("ssh-command", "", "\n\tConfigure ssh-command for the specific volume")
	geoReplicationConfigCommand.Flags().String("rsync-command", "", "\n\tConfigure rsync-command for the specific volume")
}

var geoReplicationCommand = &cobra.Command{
	Use:   "georep",
	Short: "Heketi geo-replication Management",
	Long:  "Heketi geo-replication Management",
}

var geoReplicationStatusCommand = &cobra.Command{
	Use:   "status",
	Short: "geo-replication status",
	Long:  "Displays geo-replication status from the first node of the first cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create a client
		heketi := client.NewClient(options.Url, options.User, options.Key)
		// Get volume status
		status, err := heketi.GeoReplicationStatus()
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(status)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "%v", status)
		}
		return nil
	},
}

var geoReplicationVolumeCommand = &cobra.Command{
	Use:   "georep",
	Short: "Volume geo-replication Management",
	Long:  "Heketi Volume geo-replication Management",
}

func actionFunc(actionName string) func(*cobra.Command, []string) error {
	var action api.GeoReplicationActionType
	var doneMsg string

	switch actionName {
	case "start":
		action = api.GeoReplicationActionStart
		doneMsg = "geo-replication session started\n"
	case "stop":
		action = api.GeoReplicationActionStop
		doneMsg = "geo-replication session stopped\n"
	case "pause":
		action = api.GeoReplicationActionPause
		doneMsg = "geo-replication session paused\n"
	case "resume":
		action = api.GeoReplicationActionResume
		doneMsg = "geo-replication session resumed\n"
	case "delete":
		action = api.GeoReplicationActionDelete
		doneMsg = "geo-replication session deleted\n"
	default:
		return nil
	}

	return func(cmd *cobra.Command, args []string) error {
		//ensure proper number of args
		if len(cmd.Flags().Args()) < 1 {
			return errors.New("Volume id missing")
		}

		volumeID := cmd.Flags().Arg(0)
		if volumeID == "" {
			return errors.New("Volume id missing")
		}

		if slaveHost == "" {
			return errors.New("Slave host is missing")
		}

		if slaveVolume == "" {
			return errors.New("Slave volume is missing")
		}

		actionParams := make(map[string]string)
		addActionParam("force", "bool", cmd, actionParams)

		// Create a client
		heketi := client.NewClient(options.Url, options.User, options.Key)
		req := api.GeoReplicationRequest{
			Action:       action,
			ActionParams: actionParams,
			GeoReplicationInfo: api.GeoReplicationInfo{
				SlaveHost:   slaveHost,
				SlaveVolume: slaveVolume,
			},
		}

		// Execute geo-replication action
		_, err := heketi.GeoReplicationPostAction(volumeID, &req)
		if err != nil {
			return err
		}

		fmt.Fprintf(stdout, doneMsg)

		return nil
	}
}

var geoReplicationStartCommand = &cobra.Command{
	Use:     "start",
	Short:   "Start session",
	Long:    "Start geo-replication session for given volume",
	Example: "  $ heketi-cli volume georep --slave-host=blah --slave-volume=23423423 start 886a86a868711bef83001",
	RunE:    actionFunc("start"),
}

var geoReplicationStopCommand = &cobra.Command{
	Use:     "stop",
	Short:   "Stop session",
	Long:    "Stop geo-replication session for given volume",
	Example: "  $ heketi-cli volume georep --slave-host=blah --slave-volume=23423423 stop 886a86a868711bef83001",
	RunE:    actionFunc("stop"),
}

var geoReplicationPauseCommand = &cobra.Command{
	Use:     "pause",
	Short:   "Pause session",
	Long:    "Pause geo-replication session for given volume",
	Example: "  $ heketi-cli volume georep --slave-host=blah --slave-volume=23423423 pause 886a86a868711bef83001",
	RunE:    actionFunc("pause"),
}

var geoReplicationResumeCommand = &cobra.Command{
	Use:     "resume",
	Short:   "Resume session",
	Long:    "Resume geo-replication session for given volume",
	Example: "  $ heketi-cli volume georep --slave-host=blah --slave-volume=23423423 resume 886a86a868711bef83001",
	RunE:    actionFunc("resume"),
}

var geoReplicationDeleteCommand = &cobra.Command{
	Use:     "delete",
	Short:   "Delete a geo-replication session",
	Long:    "Delete the geo-replication session for a specific volume",
	Example: "  $ heketi-cli volume georep --slave-host=blah --slave-volume=23423423 delete 886a86a868711bef83001",
	RunE:    actionFunc("delete"),
}

var geoReplicationConfigCommand = &cobra.Command{
	Use:     "config",
	Short:   "Configure session",
	Long:    "Configure geo-replication session",
	Example: "  $ heketi-cli volume georep --slave-host=blah --slave-volume=23423423 config sync-jobs 1 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		//ensure proper number of args
		if len(cmd.Flags().Args()) < 1 {
			return errors.New("Volume id missing")
		}

		volumeID := cmd.Flags().Arg(0)
		if volumeID == "" {
			return errors.New("Volume id missing")
		}

		if slaveHost == "" {
			return errors.New("Slave host is missing")
		}

		if slaveVolume == "" {
			return errors.New("Slave volume is missing")
		}

		actionParams := make(map[string]string)
		addActionParam("timeout", "int", cmd, actionParams)
		addActionParam("ssh-port", "int", cmd, actionParams) // due to gluster CLI inconsistency, this is translated to ssh_port serverside
		addActionParam("sync-jobs", "int", cmd, actionParams)
		addActionParam("use-tarssh", "bool", cmd, actionParams)
		addActionParam("use-meta-volume", "bool", cmd, actionParams)
		addActionParam("ignore-deletes", "bool", cmd, actionParams)
		addActionParam("log-level", "string", cmd, actionParams)
		addActionParam("gluster-log-level", "string", cmd, actionParams)
		addActionParam("changelog-log-level", "string", cmd, actionParams)
		addActionParam("ssh-command", "string", cmd, actionParams)
		addActionParam("rsync-command", "string", cmd, actionParams)

		// Create a client
		heketi := client.NewClient(options.Url, options.User, options.Key)
		req := api.GeoReplicationRequest{
			Action:       api.GeoReplicationActionConfig,
			ActionParams: actionParams,
			GeoReplicationInfo: api.GeoReplicationInfo{
				SlaveHost:   slaveHost,
				SlaveVolume: slaveVolume,
			},
		}

		// Send geo-replication config command
		if _, err := heketi.GeoReplicationPostAction(volumeID, &req); err != nil {
			return err
		}

		return nil
	},
}

var geoReplicationVolumeStatusCommand = &cobra.Command{
	Use:     "status",
	Short:   "Status of geo-replication session",
	Long:    "Get the status of the geo-replication session for a specific volume",
	Example: "  $ heketi-cli volume status 886a86a868711bef83001 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		//ensure proper number of args
		if len(cmd.Flags().Args()) < 1 {
			return errors.New("Volume id missing")
		}

		volumeID := cmd.Flags().Arg(0)
		if volumeID == "" {
			return errors.New("Volume id missing")
		}

		// Create a client
		heketi := client.NewClient(options.Url, options.User, options.Key)
		// Get volume status
		status, err := heketi.GeoReplicationVolumeStatus(volumeID)
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(status)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "%v", status)
		}
		return nil
	},
}

func addActionParam(name, paramType string, cmd *cobra.Command, params map[string]string) {
	switch paramType {
	case "string":
		if value, err := cmd.Flags().GetString(name); err == nil && value != "" {
			params[name] = value
		}
	case "bool":
		if value, err := cmd.Flags().GetBool(name); err == nil && value {
			params[name] = fmt.Sprintf("%t", value)
		}
	case "int":
		if value, err := cmd.Flags().GetInt(name); err == nil && value != 0 {
			params[name] = fmt.Sprintf("%d", value)
		}
	}
}

var geoReplicationCreateCommand = &cobra.Command{
	Use:     "create",
	Short:   "Create session",
	Long:    "Create geo-replication session",
	Example: "  $ heketi-cli volume georep --slave-host=blah --slave-volume=23423423 create 886a86a868711bef83001",
	RunE: func(cmd *cobra.Command, args []string) error {
		//ensure proper number of args
		if len(cmd.Flags().Args()) < 1 {
			return errors.New("Volume id missing")
		}

		volumeID := cmd.Flags().Arg(0)
		if volumeID == "" {
			return errors.New("Volume id missing")
		}

		if slaveHost == "" {
			return errors.New("Slave host is missing")
		}

		if slaveVolume == "" {
			return errors.New("Slave volume is missing")
		}

		actionParams := make(map[string]string)
		addActionParam("force", "bool", cmd, actionParams)
		addActionParam("option", "string", cmd, actionParams)

		sshPortFlag, err := cmd.Flags().GetInt("ssh-port")
		if err != nil {
			return err
		}

		// Create a client
		heketi := client.NewClient(options.Url, options.User, options.Key)
		req := api.GeoReplicationRequest{
			Action:       api.GeoReplicationActionCreate,
			ActionParams: actionParams,
			GeoReplicationInfo: api.GeoReplicationInfo{
				SlaveHost:    slaveHost,
				SlaveVolume:  slaveVolume,
				SlaveSSHPort: sshPortFlag,
			},
		}

		// Get volume status
		status, err := heketi.GeoReplicationPostAction(volumeID, &req)
		if err != nil {
			return err
		}

		if options.Json {
			data, err := json.Marshal(status)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "%v", status)
		}
		return nil
	},
}
