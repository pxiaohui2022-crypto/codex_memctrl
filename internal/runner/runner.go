package runner

import (
	"context"
	"os"
	"os/exec"
)

func RunWithPack(ctx context.Context, command []string, env map[string]string) error {
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	return cmd.Run()
}
