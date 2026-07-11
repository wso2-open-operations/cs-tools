package main

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

type clipboardCommand struct {
	name string
	args []string
}

var writeSystemClipboard = copyToSystemClipboard

func copyToSystemClipboard(value string) error {
	var lastErr error
	for _, candidate := range systemClipboardCommands(runtime.GOOS) {
		path, err := exec.LookPath(candidate.name)
		if err != nil {
			continue
		}
		command := exec.Command(path, candidate.args...)
		command.Stdin = strings.NewReader(value)
		if err := command.Run(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return errors.New("no supported system clipboard command found")
}

func systemClipboardCommands(goos string) []clipboardCommand {
	switch goos {
	case "darwin":
		return []clipboardCommand{{name: "pbcopy"}}
	case "linux":
		return []clipboardCommand{
			{name: "wl-copy"},
			{name: "xclip", args: []string{"-selection", "clipboard"}},
			{name: "xsel", args: []string{"--clipboard", "--input"}},
			{name: "clip.exe"},
		}
	case "windows":
		return []clipboardCommand{{name: "clip.exe"}, {name: "clip"}}
	default:
		return nil
	}
}
