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

type redirects struct {
	stdout string
	stderr string
}

func splitRedirects(args []string) ([]string, redirects) {
	var r redirects
	var rest []string

	for i := 0; i < len(args); i++ {
		if i+1 < len(args) {
			switch args[i] {
			case ">", "1>":
				r.stdout = args[i+1]
				i++
				continue
			case "2>":
				r.stderr = args[i+1]
				i++
				continue
			}
		}
		rest = append(rest, args[i])
	}

	return rest, r
}

func emit(content, file string, fallback *os.File) {
	if file == "" {
		fmt.Fprint(fallback, content)
		return
	}

	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error while writing file: %s\n", err)
	}
}

func finish(stdout, stderr string, r redirects) {
	emit(stdout, r.stdout, os.Stdout)
	emit(stderr, r.stderr, os.Stderr)
}

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

func runSystemCmd(rawArgs []string) {

	args, r := splitRedirects(rawArgs)
	if len(args) == 0 {
		return
	}

	cmdToRun := args[0]

	if _, err := exec.LookPath(cmdToRun); err != nil {
		finish("", fmt.Sprintf("%s: command not found\n", cmdToRun), r)
		return
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(cmdToRun, args[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr,"%s: %s\n", cmdToRun, err)
		}
	}

	finish(stdout.String(), stderr.String(), r)
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

func handleCd(rawArgs []string) {
	args, r := splitRedirects(rawArgs)

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No args provided.\n")
		return
	}

	toDir := args[0]

	if err := changeDir(toDir); err != nil {
		finish("", fmt.Sprintf("cd: %s: No such file or directory\n", toDir), r)
		return
	}
	finish("", "", r)
}

func handleType(rawArgs []string) {
	args, r := splitRedirects(rawArgs)

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No args provided.\n")
		return
	}

	if slices.Contains(shellBuiltIn, args[0]) {
		finish(fmt.Sprintf("%s is a shell builtin\n", args[0]), "", r)
		return
	}

	if path, err := exec.LookPath(args[0]); err == nil {
		finish(fmt.Sprintf("%s is %s\n", args[0], path), "", r)
	} else {
		finish("", fmt.Sprintf("%s: not found\n", args[0]), r)
	}
}

func handleEcho(rawArgs []string) {
	args, r := splitRedirects(rawArgs)
	finish(strings.Join(args, " ")+"\n", "", r)
}

func handlePwd(rawArgs []string) {
	_, r := splitRedirects(rawArgs)
	finish(processDir+"\n", "", r)
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
