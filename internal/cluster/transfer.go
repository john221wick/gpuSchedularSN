package cluster

import (
	"fmt"
	"os/exec"
)

// RsyncToRemote syncs a local directory to the remote node via SSH.
// Returns the rsync output for display.
func RsyncToRemote(localDir string, config SSHConfig, remoteDir string) error {
	sshCmd := fmt.Sprintf("ssh -p %d -o StrictHostKeyChecking=no", config.Port)
	if config.KeyPath != "" {
		sshCmd += fmt.Sprintf(" -i %s", config.KeyPath)
	}

	remote := fmt.Sprintf("%s@%s:%s/", config.User, config.Host, remoteDir)
	args := []string{
		"-avz",
		"-e", sshCmd,
		localDir + "/",
		remote,
	}

	fmt.Printf("rsync cmd: rsync %v\n", args)
	cmd := exec.Command("rsync", args...)
	out, err := cmd.CombinedOutput()
	fmt.Printf("rsync output: %s\n", string(out))
	if err != nil {
		return fmt.Errorf("rsync to %s: %w\n%s", config.Host, err, string(out))
	}
	return nil
}

// RsyncFromRemote syncs a remote directory back to local.
func RsyncFromRemote(config SSHConfig, remoteDir, localDir string) error {
	sshCmd := fmt.Sprintf("ssh -p %d -o StrictHostKeyChecking=no", config.Port)
	if config.KeyPath != "" {
		sshCmd += fmt.Sprintf(" -i %s", config.KeyPath)
	}

	remote := fmt.Sprintf("%s@%s:%s/", config.User, config.Host, remoteDir)
	args := []string{
		"-az",
		"-e", sshCmd,
		remote,
		localDir + "/",
	}

	cmd := exec.Command("rsync", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsync from %s: %w\n%s", config.Host, err, string(out))
	}
	return nil
}
