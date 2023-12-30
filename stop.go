package main

import (
	"encoding/json"
	"fmt"
	"mydocker/constant"
	"mydocker/container"
	"os"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func stopContainer(containerName string) {
	// 1. 根据容器名称获取其 PID
	containerInfo, err := getContainerInfoByName(containerName)
	if err != nil {
		log.Errorf("Get container %s info error: %v", containerName, err)
		return
	}
	pid, err := strconv.Atoi(containerInfo.Pid)
	if err != nil {
		log.Errorf("Conver pid from string to int error: %v", err)
		return
	}
	// 2. 发送 SIGTERM 信号
	if err = syscall.Kill(pid, syscall.SIGTERM); err != nil {
		log.Errorf("Stop container %s error: %v", containerName, err)
		return
	}
	// 3. 修改容器信息，将容器置为 STOP 状态，并清空 PID
	containerInfo.Status = container.STOP
	containerInfo.Pid = ""
	newContent, err := json.Marshal(containerInfo)
	if err != nil {
		log.Errorf("Json marshal %s error %v", containerName, err)
		return
	}
	// 4. 重新存储容器信息
	dirPath := fmt.Sprintf(container.InfoLocFormat, containerName)
	configFilePath := dirPath + container.ConfigName
	if err = os.WriteFile(configFilePath, newContent, constant.Perm0622); err != nil {
		log.Errorf("Write file %s error: %v", configFilePath, err)
	}
}

func removeContainer(containerName string) {
	containerInfo, err := getContainerInfoByName(containerName)
	if err != nil {
		log.Errorf("Get container %s info error: %v", containerName, err)
		return
	}
	// 只删除 STOP 状态下的容器
	if containerInfo.Status != container.STOP {
		log.Errorf("Couldn't remove running container")
		return
	}
	dirPath := fmt.Sprintf(container.InfoLocFormat, containerName)
	if err = os.RemoveAll(dirPath); err != nil {
		log.Errorf("Remove file %s error: %v", dirPath, err)
		return
	}
}

func getContainerInfoByName(containerName string) (*container.Info, error) {
	dirPath := fmt.Sprintf(container.InfoLocFormat, containerName)
	configFilePath := dirPath + container.ConfigName
	contentBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "read file %s", configFilePath)
	}
	var containerInfo container.Info
	if err = json.Unmarshal(contentBytes, &containerInfo); err != nil {
		return nil, err
	}
	return &containerInfo, nil
}
