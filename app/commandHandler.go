package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
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

func emit(content string, t target, fallback io.Writer) {
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

func finish(stdout, stderr string, r redirects, outWriter io.Writer) {
	emit(stdout, r.stdout, outWriter)
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

func nextFreeJobNumber() int {
	n := 1
	for slices.ContainsFunc(jobs, func(j job) bool { return j.number == n }) {
		n++
	}
	return n
}

func reapJobs(onlyDone bool) string {
	jobsMu.Lock()
	snapshot := slices.Clone(jobs)
	jobsMu.Unlock()

	var out strings.Builder
	var finished []int

	for idx, j := range snapshot {
		if j.done {
			finished = append(finished, j.number)
		}
		if onlyDone && !j.done {
			continue
		}

		marker := " "
		switch idx {
		case len(snapshot) - 1:
			marker = "+"
		case len(snapshot) - 2:
			marker = "-"
		}

		status := "Running"
		command := j.command
		if j.done {
			status = "Done"
			command = strings.TrimSuffix(command, " &")
		}

		fmt.Fprintf(&out, "[%d]%s  %-24s%s\n", j.number, marker, status, command)
	}

	jobsMu.Lock()
	jobs = slices.DeleteFunc(jobs, func(j job) bool {
		return slices.Contains(finished, j.number)
	})
	jobsMu.Unlock()

	return out.String()
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

func handleSystemCmd(rawArgs []string, backGroundTask bool, stdin io.Reader, outWriter io.Writer) {

	args, r := splitRedirects(rawArgs)
	if len(args) == 0 {
		return
	}

	cmdToRun := args[0]

	if _, err := exec.LookPath(cmdToRun); err != nil {
		finish("", fmt.Sprintf("%s: command not found\n", cmdToRun), r, outWriter)
		return
	}

	cmd := exec.Command(cmdToRun, args[1:]...)
	cmd.Stdin = stdin
	if !backGroundTask {
		var stdout, stderr bytes.Buffer
		cmd.Stderr = &stderr
		if r.stdout.file == "" {
			cmd.Stdout = outWriter
		} else {
			cmd.Stdout = &stdout
		}
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				fmt.Fprintf(os.Stderr, "%s: %s\n", cmdToRun, err)
			}
		}

		finish(stdout.String(), stderr.String(), r, outWriter)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", cmdToRun, err)
			return
		}

		pid := cmd.Process.Pid

		jobsMu.Lock()
		number := nextFreeJobNumber()
		jobs = append(jobs, job{
			number:  number,
			pid:     pid,
			command: strings.Join(args, " ") + " &",
		})
		jobsMu.Unlock()

		fmt.Printf("[%d] %d\n", number, pid)

		go func() {
			cmd.Wait()
			jobsMu.Lock()
			for i := range jobs {
				if jobs[i].number == number {
					jobs[i].done = true
				}
			}
			jobsMu.Unlock()
		}()
	}
}

func handleCd(rawArgs []string, outWriter io.Writer) {
	args, r := splitRedirects(rawArgs)

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No args provided.\n")
		return
	}

	toDir := args[0]

	if err := changeDir(toDir); err != nil {
		finish("", fmt.Sprintf("cd: %s: No such file or directory\n", toDir), r, outWriter)
		return
	}
	finish("", "", r, outWriter)
}

func handleType(rawArgs []string, outWriter io.Writer) {
	args, r := splitRedirects(rawArgs)

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No args provided.\n")
		return
	}

	if slices.Contains(shellBuiltIn, args[0]) {
		finish(fmt.Sprintf("%s is a shell builtin\n", args[0]), "", r, outWriter)
		return
	}

	if path, err := exec.LookPath(args[0]); err == nil {
		finish(fmt.Sprintf("%s is %s\n", args[0], path), "", r, outWriter)
	} else {
		finish("", fmt.Sprintf("%s: not found\n", args[0]), r, outWriter)
	}
}

func handleEcho(rawArgs []string, outWriter io.Writer) {
	args, r := splitRedirects(rawArgs)
	finish(strings.Join(args, " ")+"\n", "", r, outWriter)
}

func handlePwd(rawArgs []string, outWriter io.Writer) {
	_, r := splitRedirects(rawArgs)
	finish(processDir+"\n", "", r, outWriter)
}

func handleComplete(rawArgs []string, outWriter io.Writer) {
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
	finish(printStr, errStr, r, outWriter)
}

func handleJobs(rawArgs []string, outWriter io.Writer) {
	_, r := splitRedirects(rawArgs)
	finish(reapJobs(false), "", r, outWriter)
}

func handleHistory(rawArgs []string, outWriter io.Writer) {
	args, r := splitRedirects(rawArgs)
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No proper args provided.\n")
		return
	}
	var toPrint strings.Builder
	lastNToPrint, err := strconv.Atoi(args[0])

	if err != nil {
		fmt.Fprintf(os.Stderr, "No proper args provided.\n")
		return
	}

	if lastNToPrint > len(historyStore) {
		lastNToPrint = len(historyStore)
	}

	for idx, item := range historyStore[len(historyStore)-lastNToPrint:] {
		fmt.Fprintf(&toPrint, "%5d  %s\n", len(historyStore)-lastNToPrint+idx+1, item)
	}

	finish(toPrint.String(), "", r, outWriter)
}
