package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type builder struct {
	wd     string
	app    string
	bindir string

	out string
}

func New(
	_app string,
	_bindir string,
) (*builder, error) {
	_wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &builder{
		wd:     _wd,
		app:    _app,
		bindir: _bindir,
		out:    filepath.Join(_bindir, _app),
	}, nil
}

func (b *builder) run(action string) error {
	switch action {
	case "build":
		return b.setup(
			func() error {
				return b.build()
			})
	}
	return nil
}

func (b *builder) setup(f ...func() error) error {
	if err := os.MkdirAll(b.bindir, 0o777); err != nil {
		return err
	}
	if len(f) > 0 {
		if err := f[0](); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) build(f ...func() error) error {
	args := []string{
		"build",
		"-tags=netgo",
		`-ldflags="-extldflags '-static' -w -s -buildid="`,
		fmt.Sprintf(`-gcflags="all=-trimpath=%s -dwarf=false -l"`, b.wd),
		fmt.Sprintf(`-asmflags="-trimpath=%s"`, b.wd),
		fmt.Sprintf(`-o %s`, b.out),
		fmt.Sprintf(`cmd/%s/main.go`, b.app),
	}
	envs := []string{
		"CGO_ENABLED=0",
		"GOARCH=amd64",
		"GOOS=linux",
	}
	if _, err := Cmd("go", args...).WithEnv(envs).Run(); err != nil {
		return err
	}
	if len(f) > 0 {
		if err := f[0](); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}

	builder, err := New(
		"t2s",
		"bin",
	)
	if err != nil {
		log.Fatalln(err)
	}

	if err := builder.run(os.Args[1]); err != nil {
		log.Fatalln(err)
	}
	os.Exit(0)
}

type Command struct {
	cmd *exec.Cmd
}

func Cmd(command string, args ...string) *Command {
	_cmd := &Command{
		cmd: exec.Command(command, args...),
	}
	_cmd.Log()
	return _cmd
}

func (c *Command) String() string {
	return strings.Join(c.cmd.Args, " ")
}

func (c *Command) Log() {
	log.Println(c)
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
	out, err := c.cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w %s", err, string(out))
	}
	return string(out), nil
}

func (c *Command) RunCode() (string, int, error) {
	out, err := c.cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), exitErr.ExitCode(), fmt.Errorf("%w %s", err, string(out))
		}
		return string(out), -1, fmt.Errorf("%w %s", err, string(out))
	}
	return string(out), 0, nil
}
