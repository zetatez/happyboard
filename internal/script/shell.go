package script

import (
	"os/exec"
	"runtime"

	"github.com/sirupsen/logrus"
)

func RunShell(command string, async bool) error {
	run := func() error {
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", command)
		} else {
			cmd = exec.Command("sh", "-c", command)
		}
		return cmd.Run()
	}

	if async {
		go func() {
			if err := run(); err != nil {
				logrus.Errorf("shell: async command failed: %v", err)
			}
		}()
		return nil
	}
	return run()
}
