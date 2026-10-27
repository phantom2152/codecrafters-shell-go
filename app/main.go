package main

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/chzyer/readline"
)

var shellBuiltIn = []string{"echo", "exit", "type", "pwd", "cd", "complete", "jobs"}
var completeMap map[string]string
var processDir string
var shellOut io.Writer = os.Stdout
var shellErr io.Writer = os.Stderr

type job struct {
	number  int
	pid     int
	command string
	done    bool
}

var jobs []job
var jobsMu sync.Mutex

func runCommand(commandArgs []string, background bool) bool {
	command := commandArgs[0]
	args := commandArgs[1:]

	switch command {
	case "exit":
		return false
	case "pwd":
		handlePwd(args)
	case "cd":
		handleCd(args)
	case "type":
		handleType(args)
	case "echo":
		handleEcho(args)
	case "complete":
		handleComplete(args)
	case "jobs":
		handleJobs(args)
	default:
		handleSystemCmd(commandArgs, background)
	}
	return true
}

func main() {
	completeMap = make(map[string]string)
	rl, err := readline.NewEx(&readline.Config{
		Prompt:       "$ ",
		AutoComplete: &builtinCompleter{names: shellBuiltIn},
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()
	processDir, _ = os.Getwd()

mainLoop:
	for {
		backGroundTask := false
		fmt.Print(reapJobs(true))
		text, err := rl.Readline()
		if err == readline.ErrInterrupt {
			continue
		}
		if err == io.EOF {
			break // Ctrl+D
		}
		text = strings.TrimSuffix(text, "\n")
		commandArgs := handleInputString(text)

		if len(commandArgs) == 0 {
			continue mainLoop
		}

		if len(commandArgs) > 1 && commandArgs[len(commandArgs)-1] == "&" {
			commandArgs = commandArgs[:len(commandArgs)-1]
			backGroundTask = true
		}

		if idx := slices.Index(commandArgs, "|"); idx != -1 {
			// TODO handle pipes
			continue mainLoop
		}

		if !runCommand(commandArgs, backGroundTask) {
			break mainLoop
		}
	}
}
