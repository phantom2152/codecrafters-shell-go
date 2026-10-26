package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

type PathHolder struct{
	CurrentPath string
}

func NewPH() (*PathHolder,error ){
	originalDir, err := os.Getwd()
	if err != nil {
			return &PathHolder{},fmt.Errorf("Error getting current directory: %w", err)
		}
	return &PathHolder{CurrentPath: originalDir}, nil
}

func (p *PathHolder) getCWD() (string){
	return p.CurrentPath
}

func (p *PathHolder) changeDir(changePath string) (error){
	err := os.Chdir(changePath)

	if err != nil {
			return fmt.Errorf("Failed to change directory: %w", err)
		}
		newDir, err := os.Getwd()
		if err != nil {
				return fmt.Errorf("Error getting current directory: %w", err)
			}
		p.CurrentPath = newDir

		return  err
}

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
	fmt.Printf("%s", string(output))
}

func runPwd() {
	cwd, _ := os.Getwd()
	fmt.Println(cwd)

}

func main() {
	reader := bufio.NewReader(os.Stdin)
	var commands = []string{"echo", "exit", "type", "pwd","cd"}
	ph,_ := NewPH()
	
	for {
		fmt.Print("$ ")
		text, _ := reader.ReadString('\n')
		text = strings.ReplaceAll(text, "\n", "")
		text = strings.TrimSpace(text)

		if text == "exit" {
			break
		} else if text == "pwd" {
			fmt.Println(ph.getCWD())
		
		} else if strings.HasPrefix(text, "cd "){
			toDir := text[3:]
			err := ph.changeDir(toDir)
			if (err != nil){
				fmt.Printf("cd: %s: No such file or directory\n",toDir)
			}
		}else if strings.HasPrefix(text, "type ") && slices.Contains(commands, text[5:]) {
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
