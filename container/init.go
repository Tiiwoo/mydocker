package container

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// RunContainerInitProcess 启动容器的 init 进程
/*
	这里的 init 函数是在容器内部执行的，也就是说，代码执行到这里后，容器所在的进程其实就已经创建出来了，
	这是本容器执行的第一个进程。
	使用 mount 先去挂载 proc 文件系统，以便后面通过 ps 等系统命令去查看当前进程资源的情况。
*/
func RunContainerInitProcess() error {
	// 通过 readPipe 读取 writePipe 写入的需要执行的命令
	cmdList := readUserCommand()
	if len(cmdList) == 0 {
		return errors.New("run container get user command error, cmdList is nil")
	}
	path, err := exec.LookPath(cmdList[0])
	if err != nil {
		log.Errorf("Exec loop path error %v", err)
		return err
	}
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	// 设置容器内部 hostname
	_ = syscall.Sethostname([]byte("container"))
	// mount --make-private /proc
	// 防止在新的 namespace 中修改会传播到原来的 namespace 中
	_ = syscall.Mount("", "/proc", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	// mount -t proc proc /proc
	_ = syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")
	log.Infof("Find path %s", path)
	if err = syscall.Exec(path, cmdList[0:], os.Environ()); err != nil {
		log.Errorf("RunContainerInitProcess exec :" + err.Error())
	}
	return nil
}

const fdIndex = 3

func readUserCommand() []string {
	// uintptr(3) 就是指 index 为 3 的文件描述符，也就是传递进来的管道的另一端，至于为什么是 3，具体解释如下：
	/*
		因为每个进程默认都会有3个文件描述符，分别是标准输入、标准输出、标准错误。这3个是子进程一创建的时候就会默认带着的，
		前面通过ExtraFiles方式带过来的 readPipe 理所当然地就成为了第4个。
		在进程中可以通过index方式读取对应的文件，比如
		index0：标准输入
		index1：标准输出
		index2：标准错误
		index3：带过来的第一个 FD，也就是 readPipe
		由于可以带多个 FD 过来，所以这里的 3 就不是固定的了。
		比如像这样：cmd.ExtraFiles = []*os.File{a,b,c,readPipe} 这里带了4个文件过来，分别的 index 就是 3,4,5,6
		那么我们的 readPipe 就是 index6，读取时就要像这样：pipe := os.NewFile(uintptr(6), "pipe")
	*/
	pipe := os.NewFile(uintptr(fdIndex), "pipe")
	msg, err := io.ReadAll(pipe)
	if err != nil {
		log.Errorf("init read pipe error %v", err)
		return nil
	}
	msgStr := string(msg)
	return strings.Split(msgStr, " ")
}
