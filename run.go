package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"mydocker/cgroups"
	"mydocker/cgroups/subsystems"
	"mydocker/constant"
	"mydocker/container"

	"github.com/creack/pty"
	log "github.com/sirupsen/logrus"
)

// Run 执行具体 command
/*
	这里的 Start 方法是真正开始执行由 NewParentProcess 构建好的 command 的调用，它首先会 clone 出来一个 namespace 隔离的
	进程，然后在子进程中，调用 /proc/self/exe，也就是调用自己，发送 init 参数，调用我们写的 init 方法，
	去初始化容器的一些资源。
*/
func Run(tty bool, cmdList []string, cfg *subsystems.ResourceConfig, volume, containerName, imageName string) {
	// 如果没有设置 containerName 则用 containerID 代替
	containerID := randStringBytes(container.IDLength)
	if containerName == "" {
		containerName = containerID
	}
	parent, writePipe := container.NewParentProcess(tty, volume, containerName, imageName)
	if parent == nil {
		log.Errorf("new parent process error")
		return
	}
	// if err := parent.Start(); err != nil {
	// 	log.Errorf("Run parent.Start err:%v", err)
	// }
	// 创建一个伪终端
	ptmx, err := pty.Start(parent)
	if err != nil {
		panic(err)
	}
	// 确保在退出前关闭ptmx
	defer func() { _ = ptmx.Close() }()
	// 这里只是简单地将伪终端的输出复制到标准输出
	go func() {
		_, _ = io.Copy(os.Stdout, ptmx)
	}()

	// 记录 container 的 info
	err = recordContainerInfo(parent.Process.Pid, cmdList, containerName, containerID, volume)
	if err != nil {
		log.Errorf("Record container info error %v", err)
		return
	}

	// 创建 cgroup manager, 并通过调用 Set 和 Apply 设置资源限制并使限制在容器上生效
	cgroupManager := cgroups.NewCgroupManager("mydocker-cgroup")
	defer cgroupManager.Destroy()
	_ = cgroupManager.Set(cfg)
	_ = cgroupManager.Apply(parent.Process.Pid, cfg)
	// 在子进程创建后才能通过管道来发送参数
	sendInitCommand(cmdList, writePipe)
	// 只有设置了 tty 才需要 Wait
	if tty {
		_ = parent.Wait()
		deleteContainerInfo(volume, containerName)
		// rootPath := "/root"
		// mntPath := "/root/merged"
		// container.DeleteWorkSpace(rootPath, mntPath, volume)
	}
	// 这里就不能在结束后删除了，因为要后台运行
	// 需要运行完后删除相关目录
	// rootPath := "/root"
	// mntPath := "/root/merged"
	// container.DeleteWorkSpace(rootPath, mntPath, volume)
}

// sendInitCommand 通过 writePipe 将指令发送给子进程
func sendInitCommand(cmdList []string, writePipe *os.File) {
	command := strings.Join(cmdList, " ")
	log.Infof("command all is: %s", command)
	_, _ = writePipe.WriteString(command)
	_ = writePipe.Close()
}

func recordContainerInfo(containerPID int, cmdList []string, containerName, containerID, volume string) error {
	// 生成容器的创建时间
	createTime := time.Now().Format("2006-01-02 15:04:05")
	fmt.Println(createTime)
	command := strings.Join(cmdList, "")
	// 默认 Status 为 RUNNING
	containerInfo := &container.Info{
		Pid:         strconv.Itoa(containerPID),
		Id:          containerID,
		Name:        containerName,
		Command:     command,
		CreatedTime: createTime,
		Status:      container.RUNNING,
		Volume:      volume,
	}

	jsonBytes, err := json.Marshal(containerInfo)
	if err != nil {
		log.Errorf("Record container info error: %v", err)
		return err
	}
	jsonStr := string(jsonBytes)
	// 容器文件所在的路径
	dirPath := fmt.Sprintf(container.InfoLocFormat, containerName)
	if err := os.MkdirAll(dirPath, constant.Perm0622); err != nil {
		log.Errorf("Mkdir %s error: %v", dirPath, err)
		return err
	}
	// 将容器信息写入文件
	fileName := dirPath + "/" + container.ConfigName
	file, err := os.Create(fileName)
	if err != nil {
		log.Errorf("Create file %s error: %v", fileName, err)
		return err
	}
	defer file.Close()
	// 将内容写入文件中
	if _, err := file.WriteString(jsonStr); err != nil {
		log.Errorf("File write string error: %v", err)
		return err
	}
	return nil
}

func deleteContainerInfo(volume, containerName string) {
	dirPath := fmt.Sprintf(container.InfoLocFormat, containerName)
	if err := os.RemoveAll(dirPath); err != nil {
		log.Errorf("Remove dir %s error: %v", dirPath, err)
	}
}

func randStringBytes(n int) string {
	letterBytes := "1234567890"
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[r.Intn(len(letterBytes))]
	}
	return string(b)
}
