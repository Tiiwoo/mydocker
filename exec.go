package main

import (
	"encoding/json"
	"fmt"
	"mydocker/container"
	"os"
	"os/exec"
	"strings"

	// 需要导入 nsenter 包，以触发 C 代码
	_ "mydocker/nsenter"

	log "github.com/sirupsen/logrus"
)

// 控制是否执行 C 代码里面的 setns
const (
	EnvExecPid = "mydocker_pid"
	EnvExecCmd = "mydocker_cmd"
)

func ExecContainer(containerName string, cmdList []string) {
	// 获取容器的 Pid
	pid, err := getContainerPidByName(containerName)
	if err != nil {
		log.Errorf("Exec container getContainerPidByName %s error: %v", containerName, err)
		return
	}

	cmd := exec.Command("/proc/self/exe", "exec")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmdStr := strings.Join(cmdList, " ")
	log.Infof("container pid: %s command: %s", pid, cmdStr)
	_ = os.Setenv(EnvExecPid, pid)
	_ = os.Setenv(EnvExecCmd, cmdStr)

	if err := cmd.Run(); err != nil {
		log.Errorf("Exec container %s error: %v", containerName, err)
	}
}

func getContainerPidByName(containerName string) (string, error) {
	dirPath := fmt.Sprintf(container.InfoLocFormat, containerName)
	configFilePath := dirPath + container.ConfigName
	// 读取内容并解析
	content, err := os.ReadFile(configFilePath)
	if err != nil {
		return "", err
	}
	var containerInfo container.Info
	if err = json.Unmarshal(content, &containerInfo); err != nil {
		return "", err
	}
	return containerInfo.Pid, nil
}
