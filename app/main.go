package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

func checkPathCmd(cmd string) {
	if path, err := exec.LookPath(cmd); err == nil {
		fmt.Printf("%s is %s\n", cmd, path)
	} else {
		fmt.Printf("%s: not found\n", cmd)
	}
}

func runPathCmd(cmdString string) {
	args := strings.Split(cmdString, " ")
	_, err := exec.LookPath(args[0])
	if err != nil {
		fmt.Printf("%s: command not found\n", args[0])
		return
	}

	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error executing %s: %s\n", args[0], err)
	}
	fmt.Println(string(output))
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	var commands = []string{"echo", "exit", "type"}
	for {
		fmt.Print("$ ")
		text, _ := reader.ReadString('\n')
		text = strings.ReplaceAll(text, "\n", "")
		text = strings.TrimSpace(text)

		if text == "exit" {
			break
		} else if strings.HasPrefix(text, "type ") && slices.Contains(commands, text[5:]) {
			fmt.Printf("%s is a shell builtin\n", text[5:])
		} else if strings.HasPrefix(text, "type ") {
			checkPathCmd(text[5:])
		} else if strings.HasPrefix(text, "echo ") {
			fmt.Println(text[5:])
		} else {
			runPathCmd(text)
		}
	}
}
