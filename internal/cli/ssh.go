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
}

const defaultSSHAgentImage = "whilp/ssh-agent"

var sshUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Start SSH agent container",
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
