package shell

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type Command struct {
	cmd *exec.Cmd
}

func New(command string, args ...string) *Command {
	return &Command{
		cmd: exec.Command(command, args...),
	}
}

func (c *Command) WithEnv(env []string) *Command {
	c.cmd.Env = env
	return c
}

func (c *Command) WithDir(dir string) *Command {
	c.cmd.Dir = dir
	return c
}

func (c *Command) Run() (string, error) {
	log.Println(strings.Join(c.cmd.Args, " "))

	out, err := c.cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w %s", err, string(out))
	}
	return string(out), nil
}

func (c *Command) RunWithReturnCode() (string, int, error) {
	log.Println(strings.Join(c.cmd.Args, " "))

	out, err := c.cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), exitErr.ExitCode(), fmt.Errorf("%w %s", err, string(out))
		}
		return string(out), -1, fmt.Errorf("%w %s", err, string(out))
	}
	return string(out), 0, nil
}
