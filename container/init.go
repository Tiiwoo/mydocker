package container

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"

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

	// 挂载文件系统
	setUpMount()

	path, err := exec.LookPath(cmdList[0])
	if err != nil {
		log.Errorf("Exec loop path error %v", err)
		return err
	}
	// 设置容器内部 hostname
	// syscall.Sethostname([]byte("container"))
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

// Init 挂载点
func setUpMount() {
	pwd, err := os.Getwd()
	// fmt.Println("PWD: " + pwd)
	if err != nil {
		log.Errorf("Get current location error %v", err)
		return
	}
	log.Infof("Current location is %s", pwd)
	pivotRoot(pwd)

	// Mount /proc
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	// syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	// mount --make-private /proc
	// 防止在新的 namespace 中修改会传播到原来的 namespace 中
	syscall.Mount("", "/proc", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	// mount -t proc proc /proc
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	// Mount tmpfs
	syscall.Mount("", "/dev", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	syscall.Mount("none", "/dev", "devtmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")
	// syscall.Mount("devpts", "/dev/pts", "devpts", syscall.MS_NOEXEC|syscall.MS_NOSUID, "newinstance,ptmxmode=0666")
	// syscall.Mknod("/dev/ptmx", syscall.S_IFCHR, int(unix.Mkdev(5, 2)))
	// syscall.Setsid()
}

func pivotRoot(rootPath string) error {
	/*
		为了使当前 root 的老 root 和新 root 不在同一个文件系统下，我们把 root 重新 mount 了一次
		bind mount 是把相同的内容换了一个挂载点的挂载方法
	*/
	// pivotRoot 要求 newroot 是一个挂载点，但是在前面的时候已经通过 mount overlayfs 挂载过了
	// 所以不需要再次挂载，否则会重复挂载
	// if err := syscall.Mount(rootPath, rootPath, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
	// 	return errors.Wrap(err, "mount rootfs to itself")
	// }
	// 创建 rootfs/.old_root 来存储 old root
	oldDir := filepath.Join(rootPath, ".old_root")
	if err := os.Mkdir(oldDir, 0777); err != nil {
		return err
	}
	// 系统调用 pivot_root 切换到新的 root，将老的 root 挂载到 rootfs/.old_root 下
	if err := syscall.PivotRoot(rootPath, oldDir); err != nil {
		return fmt.Errorf("pivot_root %v", err)
	}
	// 修改当前目录到 "/"
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir %v", err)
	}

	oldDir = filepath.Join("/", ".old_root")
	// unmount rootfs/.old_root
	if err := syscall.Unmount(oldDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount old_root dir %v", err)
	}
	// 删除 .old_root 临时文件夹
	return os.Remove(oldDir)
}
