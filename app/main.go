package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/chzyer/readline"
)

var shellBuiltIn = []string{"echo", "exit", "type", "pwd", "cd", "complete", "jobs"}
var completeMap map[string]string
var processDir string

type job struct {
	number  int
	pid     int
	command string
	done    bool
}

var jobs []job
var nextJobNumber = 1
var jobsMu sync.Mutex

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

		command := commandArgs[0]
		args := commandArgs[1:]

		switch command {
		case "exit":
			break mainLoop
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
			handleSystemCmd(commandArgs, backGroundTask)
		}

	}
}
