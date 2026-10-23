package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("$ ")
		text, _ := reader.ReadString('\n')
		text = strings.ReplaceAll(text, "\n", "")
		text = strings.TrimSpace(text)

		if text == "exit" {
			break
		}

		if strings.HasPrefix(text, "echo ") {
			fmt.Printf("%s\n", text[5:])
			continue
		}
		fmt.Printf("%s: command not found\n", text)
	}
}
