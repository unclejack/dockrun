package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type cmdResult struct {
	output   string
	exitCode int
	err      error
}

func getExitCode(err error) (int, error) {
	exitCode := 0
	if exiterr, ok := err.(*exec.ExitError); ok {
		if procExit := exiterr.Sys().(syscall.WaitStatus); ok {
			return procExit.ExitStatus(), nil
		}
	}
	return exitCode, fmt.Errorf("failed to get exit code")
}

func runCommandWithOutput(cmd *exec.Cmd) (output string, exitCode int, err error) {
	exitCode = 0
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exiterr error
		if exitCode, exiterr = getExitCode(err); exiterr != nil {
			// TODO: Fix this so we check the error's text.
			// we've failed to retrieve exit code, so we set it to 127
			exitCode = 127
		}
	}
	output = string(out)
	return
}

func runCommand(cmd *exec.Cmd) (exitCode int, err error) {
	exitCode = 0
	err = cmd.Run()
	if err != nil {
		var exiterr error
		if exitCode, exiterr = getExitCode(err); exiterr != nil {
			// TODO: Fix this so we check the error's text.
			// we've failed to retrieve exit code, so we set it to 127
			exitCode = 127
		}
	}
	return
}

func startCommand(cmd *exec.Cmd) (exitCode int, err error) {
	exitCode = 0
	err = cmd.Start()
	if err != nil {
		var exiterr error
		if exitCode, exiterr = getExitCode(err); exiterr != nil {
			// TODO: Fix this so we check the error's text.
			// we've failed to retrieve exit code, so we set it to 127
			exitCode = 127
		}
	}
	return
}

func runCommandWithOutputResult(cmd *exec.Cmd) cmdResult {
	output, exitCode, err := runCommandWithOutput(cmd)
	return cmdResult{output, exitCode, err}
}

func runCommandSendResult(cmd *exec.Cmd, c chan cmdResult) {
	c <- runCommandWithOutputResult(cmd)
}

func waitForResult(containerID string, signals chan os.Signal, waitCmd chan cmdResult) cmdResult {
	var action string
	for {
		select {
		case sig := <-signals:
			switch sig {
			case syscall.SIGINT:
				action = "stop"
			case syscall.SIGTERM:
				action = "stop"
			}
			fmt.Printf("Received signal: %s; cleaning up\n", sig)
			cmd := exec.Command("docker", action, "-t", "2", containerID)
			out, _, err := runCommandWithOutput(cmd)
			if err != nil || strings.Contains(out, "Error") {
				fmt.Printf("stopping container via signal %s failed\n", sig)
			}
		case waitResult := <-waitCmd:
			return waitResult
		}
	}
}

func validateArgs(args []string) {
	failed := false
	if len(args) < 1 {
		fmt.Println("dockrun [OPTIONS] IMAGE [COMMAND]")
		fmt.Println("OPTIONS - same options as docker run, without -a & -i")
		failed = true
	}

	for _, val := range args {
		if val == "-a" {
			fmt.Printf("ERROR: dockrun doesn't support -a\n")
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func stringInArgs(args []string, target string) (bool, int) {
	for key, value := range args {
		if value == target {
			return true, key
		}
	}
	return false, -1
}

func filterSlice(s []string, fn func(int, string) bool) []string {
	var newSlice []string
	for k, v := range s {
		if fn(k, v) {
			newSlice = append(newSlice, v)
		}
	}
	return newSlice
}

func filterNamedArgs(flagsToFilter []string, args []string) []string {
	filteredArgs := filterSlice(args, func(k int, s string) bool {
		shouldFilter, _ := stringInArgs(flagsToFilter, s)
		return !shouldFilter
	})
	return filteredArgs
}

// WARNING: 'docker wait', 'docker logs', 'docker rm', 'docker kill' and 'docker stop'
// exit with status code 0 even if they've failed.

func main() {
	var containerID string
	var finalExitCode int
	defaultArgs := []string{"run", "-cidfile"}

	args := os.Args[1:]
	validateArgs(args)

	flagsToFilter := []string{"-rm"}

	autoRemoveContainer, _ := stringInArgs(args, "-rm")

	filteredArgs := filterNamedArgs(flagsToFilter, args)

	var CIDFilename string
	getTempFilename := exec.Command("mktemp", "-u")
	if out, exitCode, err := runCommandWithOutput(getTempFilename); err != nil {
		fmt.Printf("mktemp failed: %s\n", CIDFilename)
		fmt.Printf("ERROR mktemp failed with exit code: %d\n", exitCode)
		os.Exit(1)
	} else {
		CIDFilename = strings.Trim(string(out), "\n")
	}

	defaultArgs = append(defaultArgs, CIDFilename)
	finalArgs := append(defaultArgs, filteredArgs...)

	startCmd := exec.Command("docker", finalArgs...)
	startCmd.Stdout = os.Stdout
	startCmd.Stdin = os.Stdin
	startCmd.Stderr = os.Stderr
	if exitCode, err := startCommand(startCmd); err != nil {
		fmt.Printf("ERROR docker exited with exit code: %d\n", exitCode)
		os.Exit(1)
	}

	for i := 0; i <= 10; i++ {
		if out, err := ioutil.ReadFile(CIDFilename); err != nil {
			if i == 10 {
				fmt.Printf("ERROR couldn't read container ID from %s\n", CIDFilename)
				os.Exit(1)
			}
		} else {
			containerID = strings.Trim(string(out), "\n")
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if len(containerID) < 4 {
		fmt.Printf("ERROR: docker container ID is too small, possibly invalid\n")
		os.Exit(1)
	}

	// hack to handle signals & wait for "docker wait" to be finished
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	waitCmdRes := make(chan cmdResult, 1)
	waitCmd := exec.Command("docker", "wait", containerID)
	go runCommandSendResult(waitCmd, waitCmdRes)
	waitResult := waitForResult(containerID, signals, waitCmdRes)

	waitOutput := waitResult.output
	waiterr := waitResult.err
	// try to run 'docker wait' again; this is needed when we receive a
	// signal and 'docker wait' fails to retrieve the correct exit code
	// of the container
	if waiterr != nil {
		waitCmd := exec.Command("docker", "wait", containerID)
		waitOutput, _, waiterr = runCommandWithOutput(waitCmd)
	}
	// end hack

	if waiterr != nil || strings.Contains(waitOutput, "Error") {
		// docker wait failed
		fmt.Printf("ERROR: docker wait: %s %s\n", waitOutput, waiterr)
		fmt.Printf("ERROR: docker wait failed\n")
		os.Exit(1)
	}
	waitOutput = strings.Trim(waitOutput, "\n")
	finalExitCode, err := strconv.Atoi(waitOutput)
	if err != nil {
		fmt.Println(waitOutput)
		fmt.Printf("ERROR: failed to convert exit code to int\n")
		os.Exit(1)
	}

	if err = os.Remove(CIDFilename); err != nil {
		fmt.Printf("WARNING: failed to remove container ID file\n")
	}

	if autoRemoveContainer {
		rmCmd := exec.Command("docker", "rm", containerID)
		rmOutput, _, rmerr := runCommandWithOutput(rmCmd)
		if rmerr != nil || strings.Contains(rmOutput, "Error") {
			fmt.Printf("ERROR: docker rm: %s %s\n", rmOutput, rmerr)
			fmt.Printf("ERROR: docker rm failed\n")
			// fall through and let the return code of the container go through
		}
	}
	os.Exit(finalExitCode)
}
