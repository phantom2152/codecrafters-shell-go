package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

type target struct {
	file   string
	append bool
}

type redirects struct {
	stdout target
	stderr target
}

func replaceTilde(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}
	return path
}

func splitRedirects(args []string) ([]string, redirects) {
	var r redirects
	var rest []string

	for i := 0; i < len(args); i++ {
		if i+1 < len(args) {
			switch args[i] {
			case ">", "1>":
				r.stdout = target{args[i+1], false}
				i++
				continue
			case ">>", "1>>":
				r.stdout = target{args[i+1], true}
				i++
				continue
			case "2>":
				r.stderr = target{args[i+1], false}
				i++
				continue
			case "2>>":
				r.stderr = target{args[i+1], true}
				i++
				continue
			}
		}
		rest = append(rest, args[i])
	}

	return rest, r
}

func emit(content string, t target, fallback *os.File) {
	if t.file == "" {
		fmt.Fprint(fallback, content)
		return
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if t.append {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}

	f, err := os.OpenFile(t.file, flags, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while opening file: %s\n", err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		fmt.Fprintf(os.Stderr, "Error while writing file: %s\n", err)
	}
}

func finish(stdout, stderr string, r redirects) {
	emit(stdout, r.stdout, os.Stdout)
	emit(stderr, r.stderr, os.Stderr)
}

func changeDir(changePath string) error {
	changePath = replaceTilde(changePath)

	err := os.Chdir(changePath)
	if err != nil {
		return err
	}
	processDir, _ = os.Getwd()
	return err
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

func handleSystemCmd(rawArgs []string) {

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
			fmt.Fprintf(os.Stderr, "%s: %s\n", cmdToRun, err)
		}
	}

	finish(stdout.String(), stderr.String(), r)
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

func handleComplete(rawArgs []string) {
	args, r := splitRedirects(rawArgs)

	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "No proper args provided.\n")
		return
	}

	flag := args[0]

	var errStr, printStr string
	switch flag {
	case "-C":
		{
			if len(args) < 3 {
				fmt.Fprintf(os.Stderr, "No proper args provided.\n")
				return
			}
			completeMap[args[2]] = args[1]
		}
	case "-p":
		{
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "No proper args provided.\n")
				return
			}
			cPath, exists := completeMap[args[1]]
			if exists {
				printStr = fmt.Sprintf("complete -C '%s' %s\n", cPath, args[1])
			} else {
				errStr = fmt.Sprintf("complete: %s: no completion specification\n", args[1])
			}
		}
	case "-r":
		{
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "No proper args provided.\n")
				return
			}
			delete(completeMap, args[1])

		}
	default:
		fmt.Fprintf(os.Stderr, "No proper flags provided.\n")
	}
	finish(printStr, errStr, r)
}
