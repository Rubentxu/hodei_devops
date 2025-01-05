package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	// Define the command and its arguments
	cmd := exec.Command("bash", "-c", "echo VAR1 is ${VAR1}")

	// Set the environment variables
	cmd.Env = append(os.Environ(), "VAR1=value1")

	// Capture the standard output and error
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error creating StdoutPipe: %v\n", err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("Error creating StderrPipe: %v\n", err)
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting command: %v\n", err)
		return
	}

	// Read the standard output
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Printf("stdout: %s\n", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading stdout: %v\n", err)
		}
	}()

	// Read the standard error
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Printf("stderr: %s\n", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading stderr: %v\n", err)
		}
	}()

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		fmt.Printf("Command finished with error: %v\n", err)
	} else {
		fmt.Println("Command finished successfully")
	}
}
