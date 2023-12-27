package container

import (
	"os"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// RunContainerInitProcess 启动容器的 init 进程
/*
	这里的 init 函数是在容器内部执行的，也就是说，代码执行到这里后，容器所在的进程其实就已经创建出来了，
	这是本容器执行的第一个进程。
	使用 mount 先去挂载 proc 文件系统，以便后面通过 ps 等系统命令去查看当前进程资源的情况。
*/
func RunContainerInitProcess(command string, args []string) error {
	log.Infof("command:%s", command)
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	// mount --make-private /proc
	// 防止在新的 namespace 中修改会传播到原来的 namespace 中
	_ = syscall.Mount("", "/proc", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	// mount -t proc proc /proc
	_ = syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")
	argv := []string{command}
	if err := syscall.Exec(command, argv, os.Environ()); err != nil {
		log.Errorf(err.Error())
	}
	return nil
}
