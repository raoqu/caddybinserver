package module

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var (
	runningCmds   = make(map[string]*exec.Cmd)
	runningCmdsMu sync.Mutex
)

func fileExists(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !fileInfo.IsDir() && fileInfo.Mode().IsRegular()
}

func uuid() string {
	arr := make([]byte, 16)
	_, err := rand.Read(arr)
	if err != nil {
		panic(err)
	}

	// 设置 UUID 版本和变体
	arr[8] = arr[8]&^0xc0 | 0x80
	arr[6] = arr[6]&^0xf0 | 0x40

	return fmt.Sprintf("%x-%x-%x-%x-%x", arr[0:4], arr[4:6], arr[6:8], arr[8:10], arr[10:])
}

func startCommand(command string, dir string, async bool) (string, error) {
	cmdParts := strings.Split(command, " ")
	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	cmd.Dir = dir

	runningCmdsMu.Lock()
	defer runningCmdsMu.Unlock()

	var err_ error = nil
	cmdID := uuid()
	runningCmds[cmdID] = cmd

	var cmdStdout, cmdStderr bytes.Buffer
	cmd.Stdout = &cmdStdout
	cmd.Stderr = &cmdStderr

	if async {
		go func() {
			executeCmd(cmd, cmdID, async, command, dir)
		}()
	} else {
		err_ = executeCmd(cmd, cmdID, async, command, dir)
		errMsg := cmdStderr.String()
		println(errMsg)
	}

	return cmdID, err_
}

func executeCmd(cmd *exec.Cmd, cmdID string, async bool, command string, dir string) error {
	err_ := cmd.Run()
	if err_ != nil {
		fmt.Printf("Command %s in directory %s failed: %s\n", command, dir, err_)
	}
	if async {
		runningCmdsMu.Lock()
		delete(runningCmds, cmdID)
		runningCmdsMu.Unlock()
	}
	return err_
}

func terminateCommand(cmdID string) bool {
	runningCmdsMu.Lock()
	defer runningCmdsMu.Unlock()

	cmd, ok := runningCmds[cmdID]
	if !ok {
		return false
	}

	err := cmd.Process.Kill()
	if err != nil {
		fmt.Printf("Command %d termination failed: %s\n", cmdID, err)
		return false
	}

	delete(runningCmds, cmdID)
	return true
}

func terminateAllCommands() bool {
	runningCmdsMu.Lock()
	defer runningCmdsMu.Unlock()

	for cmdID, cmd := range runningCmds {
		err := cmd.Process.Kill()
		if err != nil {
			// fmt.Printf("Command %d termination failed: %s\n", cmdID, err)
			return false
		}

		delete(runningCmds, cmdID)
	}

	return true
}

func getCommandOutput(cmdID string) ([]byte, bool) {
	runningCmdsMu.Lock()
	defer runningCmdsMu.Unlock()

	cmd, ok := runningCmds[cmdID]
	if !ok {
		return nil, false
	}

	output := cmd.Stdout.(*bytes.Buffer).Bytes()
	return output, true
}
