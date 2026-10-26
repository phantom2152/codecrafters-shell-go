package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

var shellBuiltIn = []string{"echo", "exit", "type", "pwd", "cd"}
var processDir string

func changeDir(changePath string) error {
	if strings.HasPrefix(changePath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("Failed to get home directory: %w", err)
		}

		changePath = filepath.Join(home, changePath[1:])
	}

	err := os.Chdir(changePath)

	if err != nil {
		return fmt.Errorf("Failed to change directory: %w", err)
	}
	newDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Error getting current directory: %w", err)
	}
	processDir = newDir

	return err
}

func runSystemCmd(args []string) {
	if len(args) == 0 {
		return
	}

	cmdToRun := args[0]
	args = args[1:]

	_, err := exec.LookPath(cmdToRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: command not found\n", cmdToRun)
		return
	}

	toBeSliced := len(args)
	if len(args) >= 2 && (args[len(args)-2] == ">" || args[len(args)-2] == "1>") {
		toBeSliced = len(args) - 2
	}

	cmd := exec.Command(cmdToRun, args[:toBeSliced]...)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr, "%s: %s\n", cmdToRun, err)
		}
	}

	handleStdOut(string(output), args[toBeSliced:])
}

func handleInputString(argString string) []string {
	var escapeInsideDoubleQuotes = []byte{'"', '\\', '$', '`', '\n'}
	var args []string
	var toAppend strings.Builder
	isInsideSingleQuote := false
	isInsideDoubleQuote := false
	tokenStarted := false

	for i := 0; i < len(argString); i++ {
		ch := argString[i]

		if isInsideDoubleQuote && ch == '\\' && i+1 < len(argString) && slices.Contains(escapeInsideDoubleQuotes, argString[i+1]) {
			i++
			toAppend.WriteByte(argString[i])
			continue
		}

		if ch == '"' && !isInsideSingleQuote {
			isInsideDoubleQuote = !isInsideDoubleQuote
			tokenStarted = true
			continue
		}

		if ch == '\'' && !isInsideDoubleQuote {
			isInsideSingleQuote = !isInsideSingleQuote
			tokenStarted = true
			continue
		}

		if ch == '\\' && (!isInsideSingleQuote && !isInsideDoubleQuote) && i+1 < len(argString) {
			tokenStarted = true
			i++
			toAppend.WriteByte(argString[i])
			continue
		}

		if ch == ' ' && (!isInsideSingleQuote && !isInsideDoubleQuote) {
			if tokenStarted {
				args = append(args, toAppend.String())
				toAppend.Reset()
				tokenStarted = false
			}
			continue
		}

		toAppend.WriteByte(ch)
		tokenStarted = true
	}

	if tokenStarted {
		args = append(args, toAppend.String())
	}

	return args
}

func handleStdOut(content string, args []string) {

	if len(args) == 0 || (args[0] != ">" && args[0] != "1>") {
		fmt.Print(content)
		return
	}

	file, err := os.Create(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while creating file: %s\n", err)
		return
	}

	defer file.Close()

	_, err = file.WriteString(content)

}

func handleCd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No args provided.\n")
		return
	}

	toDir := args[0]
	err := changeDir(toDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cd: %s: No such file or directory\n", toDir)
	}

}

func handleType(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No args provided.\n")
		return
	}

	if slices.Contains(shellBuiltIn, args[0]) {
		toPrint := fmt.Sprintf("%s is a shell builtin\n", args[0])
		handleStdOut(toPrint, args[1:])
		return
	}

	if path, err := exec.LookPath(args[0]); err == nil {
		toPrint := fmt.Sprintf("%s is %s\n", args[0], path)
		handleStdOut(toPrint, args[1:])
	} else {
		fmt.Fprintf(os.Stderr, "%s: not found\n", args[0])
	}
}

func handleEcho(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No args provided.\n")
		return
	}

	toBeSliced := len(args)

	if len(args) >= 2 && (args[len(args)-2] == ">" || args[len(args)-2] == "1>") {
		toBeSliced = len(args) - 2
	}

	toPrint := strings.Join(args[:toBeSliced], " ") + "\n"
	handleStdOut(toPrint, args[toBeSliced:])
}

func handlePwd(args []string) {
	toPrint := fmt.Sprintf("%s\n", processDir)
	handleStdOut(toPrint, args)
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	processDir, _ = os.Getwd()

mainLoop:
	for {
		fmt.Print("$ ")
		text, _ := reader.ReadString('\n')
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
			runSystemCmd(commandArgs)
		}

	}
}
