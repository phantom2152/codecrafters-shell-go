package main

import (
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
)

var shellBuiltIn = []string{"echo", "exit", "type", "pwd", "cd", "complete"}
var processDir string

func main() {

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
		default:
			handleSystemCmd(commandArgs)
		}

	}
}
