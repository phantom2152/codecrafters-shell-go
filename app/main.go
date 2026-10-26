package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

var shellBuiltIn = []string{"echo", "exit", "type", "pwd", "cd"}
var redirectOperators = []string{">", "1>", "2>"}
var processDir string

func changeDir(changePath string) error {
	if strings.HasPrefix(changePath, "~") {
		home, _ := os.UserHomeDir()
		changePath = filepath.Join(home, changePath[1:])
	}
	err := os.Chdir(changePath)
	if err != nil {
		return err
	}
	processDir, _ = os.Getwd()
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
	if len(args) >= 2 && slices.Contains(redirectOperators, args[len(args)-2]) {
		toBeSliced = len(args) - 2
	}

	var stdout,stderr bytes.Buffer
	cmd := exec.Command(cmdToRun, args[:toBeSliced]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			stderr.WriteString(fmt.Sprintf("%s: %s\n", cmdToRun, err))
		}
	}
	
	redirects := args[toBeSliced:]
	handleStdOut(stdout.String(), redirects)
	handleStdErr(stderr.String(), redirects)
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

func handleStdErr(content string, args []string) {
	if len(args) == 0 || (args[0] != "2>") {
		fmt.Fprint(os.Stderr, content)
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
	toBeSliced := len(args)
	if len(args) >= 2 && slices.Contains(redirectOperators, args[len(args)-2]) {
		toBeSliced = len(args) - 2
	}
	err := changeDir(toDir)
	if err != nil {
		toPrint := fmt.Sprintf("cd: %s: No such file or directory\n", toDir)
		handleStdErr(toPrint, args[toBeSliced:])
	}

}

func handleType(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No args provided.\n")
		return
	}

	toBeSliced := len(args)
	if len(args) >= 2 && slices.Contains(redirectOperators, args[len(args)-2]) {
		toBeSliced = len(args) - 2
	}

	if slices.Contains(shellBuiltIn, args[0]) {
		toPrint := fmt.Sprintf("%s is a shell builtin\n", args[0])
		handleStdOut(toPrint, args[toBeSliced:])
		return
	}

	if path, err := exec.LookPath(args[0]); err == nil {
		toPrint := fmt.Sprintf("%s is %s\n", args[0], path)
		handleStdOut(toPrint, args[toBeSliced:])
	} else {
		toPrint := fmt.Sprintf("%s: not found\n", args[0])
		handleStdErr(toPrint, args[toBeSliced:])
	}
}

func handleEcho(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No args provided.\n")
		return
	}

	toBeSliced := len(args)
	if len(args) >= 2 && slices.Contains(redirectOperators, args[len(args)-2]) {
		toBeSliced = len(args) - 2
	}

	toPrint := strings.Join(args[:toBeSliced], " ") + "\n"
	handleStdOut(toPrint, args[toBeSliced:])
	handleStdErr("", args[toBeSliced:])
}

func handlePwd(args []string) {
	toPrint := fmt.Sprintf("%s\n", processDir)
	handleStdOut(toPrint, args)
	handleStdErr("", args)
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
