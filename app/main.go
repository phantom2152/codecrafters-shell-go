package main

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"
)

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
			fmt.Printf("%s: not found\n", text[5:])
		} else if strings.HasPrefix(text, "echo ") {
			fmt.Println(text[5:])
		} else {
			fmt.Printf("%s: command not found\n", text)
		}
	}
}
