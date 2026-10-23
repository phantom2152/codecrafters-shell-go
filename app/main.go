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
		text = strings.NewReplacer("\n", "", " ", "").Replace(text)
		if text == "exit" {
			break
		}
		fmt.Printf("%s: command not found\n", text)
	}
}
