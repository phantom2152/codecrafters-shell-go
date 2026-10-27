package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/chzyer/readline"
)

var shellBuiltIn = []string{"echo", "exit", "type", "pwd", "cd", "complete", "jobs", "history"}
var completeMap map[string]string
var processDir string
var historyStore []string

type job struct {
	number  int
	pid     int
	command string
	done    bool
}

var jobs []job
var jobsMu sync.Mutex

func splitPipes(args []string) [][]string {
	var cmds [][]string
	var current []string
	for _, arg := range args {
		if arg == "|" {
			if len(current) > 0 {
				cmds = append(cmds, current)
				current = nil
			}
		} else {
			current = append(current, arg)
		}
	}
	if len(current) > 0 {
		cmds = append(cmds, current)
	}
	return cmds
}

func addToHistory(cmd string) {
	//TODO
}

func runCommand(commandArgs []string, background bool, stdin io.Reader, outWriter io.Writer) bool {
	command := commandArgs[0]
	args := commandArgs[1:]

	switch command {
	case "exit":
		return false
	case "pwd":
		handlePwd(args, outWriter)
	case "cd":
		handleCd(args, outWriter)
	case "type":
		handleType(args, outWriter)
	case "echo":
		handleEcho(args, outWriter)
	case "complete":
		handleComplete(args, outWriter)
	case "jobs":
		handleJobs(args, outWriter)
	case "history":
		handleHistory(args, outWriter)
	default:
		handleSystemCmd(commandArgs, background, stdin, outWriter)
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
		historyStore = append(historyStore, text)
		commandArgs := handleInputString(text)

		if len(commandArgs) == 0 {
			continue mainLoop
		}

		if len(commandArgs) > 1 && commandArgs[len(commandArgs)-1] == "&" {
			commandArgs = commandArgs[:len(commandArgs)-1]
			backGroundTask = true
		}

		commands := splitPipes(commandArgs)
		if len(commands) == 0 {
			continue mainLoop
		}
		var pipelineDone sync.WaitGroup
		var nextStageInput io.Reader = os.Stdin
		shouldBreak := false

		for i, cmdArgs := range commands {
			var stageOutput io.Writer = os.Stdout
			var pipeWriter, pipeReader *os.File

			if i < len(commands)-1 {
				reader, writer, _ := os.Pipe()
				pipeReader, pipeWriter = reader, writer
				stageOutput = writer
			}
			stageInput, args := nextStageInput, cmdArgs

			pipelineDone.Add(1)

			go func() {
				defer pipelineDone.Done()
				if !runCommand(args, backGroundTask, stageInput, stageOutput) && len(commands) == 1 {
					shouldBreak = true
				}
				if pipeWriter != nil {
					pipeWriter.Close() // gives the next stage its EOF
				}
				if f, ok := stageInput.(*os.File); ok && f != os.Stdin {
					f.Close() // we're done reading this pipe
				}
			}()

			if pipeReader != nil {
				nextStageInput = pipeReader
			}
		}

		pipelineDone.Wait()
		if shouldBreak {
			break mainLoop
		}
	}
}
