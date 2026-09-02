package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "Manage the workspace SSH agent container",
	Long: `Manage a docker container running an ssh-agent (default image whilp/ssh-agent,
overridable via dva.yml's ssh.agent_image) used to forward an SSH key into other
containers. 'up' starts it and adds a key, 'down' stops and removes it along with its
volume, 'status' inspects its current docker state. See USAGE.md's "ssh up" section.`,
}

const defaultSSHAgentImage = "whilp/ssh-agent"

var sshUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Start SSH agent container",
	Long: `Start the ssh-agent docker container and its ssh_data volume, then run
'ssh-add' with --key inside it. --user sets the container's user, and --volume
bind-mounts an extra host path (default $HOME) into the container so ssh-add can reach a
key stored there.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e, envReport := loadEnv(c)
		// `ssh up` starts a container and interpolates key/volume from the environment.
		// `ssh down` and `ssh status` take no env input and keep their existing behavior.
		if err := envReport.Err(); err != nil {
			return err
		}

		key, _ := cmd.Flags().GetString("key")
		volume, _ := cmd.Flags().GetString("volume")
		user, _ := cmd.Flags().GetString("user")

		key = e.Interpolate(key)
		volume = e.Interpolate(volume)

		agentImage := c.Ssh.AgentImage
		if agentImage == "" {
			agentImage = defaultSSHAgentImage
		}

		// Create volume
		runDockerSilent("volume", "create", "--name", "ssh_data")

		// Run ssh-agent container
		runArgs := []string{"run", "--detach", "--volume", "ssh_data:/ssh", "--name=ssh-agent"}
		if user != "" {
			runArgs = append(runArgs, "-u", user)
		}
		runArgs = append(runArgs, agentImage)
		runDockerSilent(runArgs...)

		// Add key
		addArgs := []string{"run", "--rm", "--volume", "ssh_data:/ssh"}
		if volume != "" {
			addArgs = append(addArgs, "--volume", volume+":"+volume)
		}
		addArgs = append(addArgs, "--interactive", "--tty", agentImage, "ssh-add", key)
		c2 := exec.Command("docker", addArgs...)
		c2.Stdin = os.Stdin
		c2.Stdout = os.Stdout
		c2.Stderr = os.Stderr
		return c2.Run()
	},
}

var sshDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop and remove SSH agent container",
	Long:  `Stop and remove the ssh-agent container along with its ssh_data volume.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		runDockerSilent("stop", "ssh-agent")
		runDockerSilent("rm", "-v", "ssh-agent")
		runDockerSilent("volume", "rm", "ssh_data")
		fmt.Println("SSH agent stopped and removed")
		return nil
	},
}

var sshStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SSH agent container status",
	Long:  `Print the ssh-agent container's docker state via 'docker inspect --format {{.State.Status}}'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", "ssh-agent")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	sshUpCmd.Flags().StringP("key", "k", "$HOME/.ssh/id_rsa", "SSH key path")
	sshUpCmd.Flags().StringP("volume", "v", "$HOME", "Volume to mount")
	sshUpCmd.Flags().StringP("user", "u", "", "User for ssh-agent container")

	sshCmd.AddCommand(sshUpCmd, sshDownCmd, sshStatusCmd)
	setGroupParentBehavior(sshCmd)
}

// runDockerSilent runs a docker subcommand suppressing all output. Every call site
// in this file targets docker, so the binary is fixed here rather than repeated.
func runDockerSilent(args ...string) {
	c := exec.Command("docker", args...)
	c.Stdout = nil
	c.Stderr = nil
	_ = c.Run()
}
