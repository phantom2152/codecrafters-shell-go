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
	"strings"

	"github.com/chzyer/readline"
)

type target struct {
	file   string
	append bool
}

type builtinCompleter struct {
	names      []string
	lastPrefix string
}

func (c *builtinCompleter) Do(line []rune, pos int) ([][]rune, int) {
	prefix := string(line[:pos])

	matches := NewSet[string]()
	for _, name := range c.names {
		if strings.HasPrefix(name, prefix) {
			matches.Add(name)
		}
	}

	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, prefix) {
				continue
			}

			info, err := os.Stat(filepath.Join(dir, name))
			if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
				continue
			}

			matches.Add(name)
		}
	}

	if len(matches) == 0 {
		fmt.Print("\x07") // bell
		return nil, 0
	}

	names := make([]string, 0, len(matches))
	for name := range matches {
		names = append(names, name)
	}
	slices.Sort(names)
	
	lcp := longestCommonPrefix(names)

	if len(names) == 1 {
		c.lastPrefix = ""
		return [][]rune{[]rune(names[0][len(prefix):] + " ")}, len(prefix)
	}
	
	if len(lcp) > len(prefix) {
		c.lastPrefix = ""
    	return [][]rune{[]rune(lcp[len(prefix):])}, len(prefix)
	}

	if prefix != c.lastPrefix {
		c.lastPrefix = prefix
		fmt.Print("\x07") // bell
		return nil, 0
	}

	
	c.lastPrefix = ""
	fmt.Print("\r\n" + strings.Join(names, "  ") + "\r\n")
	fmt.Print("$ " + prefix)
	return nil, 0

}

func longestCommonPrefix(names []string) string {
	if len(names) == 0{
		return  ""
	}

	first := names[0]
	last := names[len(names)-1]

	i:=0
	for i < len(first) && i < len(last) && first[i] == last[i] {
        i++
    }

    return first[:i]
}

type redirects struct {
	stdout target
	stderr target
}

var shellBuiltIn = []string{"echo", "exit", "type", "pwd", "cd"}
var processDir string

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
			fmt.Fprintf(os.Stderr, "%s: %s\n", cmdToRun, err)
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

	rl, err := readline.NewEx(&readline.Config{
		Prompt:       "$ ",
		AutoComplete: &builtinCompleter{names: []string{"echo", "exit"}},
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
			runSystemCmd(commandArgs)
		}

	}
}
