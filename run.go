package main

import (
	"os"
	"strings"

	"mydocker/cgroups"
	"mydocker/cgroups/subsystems"
	"mydocker/container"

	log "github.com/sirupsen/logrus"
)

// Run 执行具体 command
/*
	这里的 Start 方法是真正开始执行由 NewParentProcess 构建好的 command 的调用，它首先会 clone 出来一个 namespace 隔离的
	进程，然后在子进程中，调用 /proc/self/exe，也就是调用自己，发送 init 参数，调用我们写的 init 方法，
	去初始化容器的一些资源。
*/
func Run(tty bool, cmdList []string, cfg *subsystems.ResourceConfig) {
	parent, writePipe := container.NewParentProcess(tty)
	if parent == nil {
		log.Errorf("New parent process error")
		return
	}
	if err := parent.Start(); err != nil {
		log.Errorf("Run parent.Start err:%v", err)
	}
	// 创建 cgroup manager, 并通过调用 Set 和 Apply 设置资源限制并使限制在容器上生效
	cgroupManager := cgroups.NewCgroupManager("mydocker-cgroup")
	defer cgroupManager.Destroy()
	_ = cgroupManager.Set(cfg)
	_ = cgroupManager.Apply(parent.Process.Pid, cfg)
	// 在子进程创建后才能通过管道来发送参数
	sendInitCommand(cmdList, writePipe)
	_ = parent.Wait()
}

// sendInitCommand 通过 writePipe 将指令发送给子进程
func sendInitCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	log.Infof("command all is %s", command)
	_, _ = writePipe.WriteString(command)
	_ = writePipe.Close()
}
