package main

import (
	"os"

	"mydocker/container"

	log "github.com/sirupsen/logrus"
)

// Run 执行具体 command
/*
	这里的 Start 方法是真正开始执行由 NewParentProcess 构建好的 command 的调用，它首先会 clone 出来一个 namespace 隔离的
	进程，然后在子进程中，调用 /proc/self/exe，也就是调用自己，发送 init 参数，调用我们写的 init 方法，
	去初始化容器的一些资源。
*/
func Run(tty bool, cmd string) {
	parent := container.NewParentProcess(tty, cmd)
	if err := parent.Start(); err != nil {
		log.Error(err)
	}
	_ = parent.Wait()
	os.Exit(-1)
}
