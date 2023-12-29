package container

import (
	"os"
	"os/exec"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// NewParentProcess 构建 command 用于启动一个新进程
/*
	这里是父进程，也就是当前进程执行的内容
	1. 这里的 /proc/self/exe 调用中，/proc/self/ 指的是当前运行进程自己的环境，exe 其实就是自己调用了自己，使用这种方式对创建出来的进程进行初始化
	2. 后面的 args 是参数，其中 init 是传递给本进程的第一个参数，在本例中，其实就是会去调用 initCommand 去初始化进程的一些环境和资源
	3. 下面的 clone 参数就是去 fork 出来一个新进程，并且使用了 namespace 隔离新创建的进程和外部环境。
	4. 如果用户指定了 -it 参数，就需要把当前进程的输入输出导入到标准输入输出上
*/
func NewParentProcess(tty bool) (*exec.Cmd, *os.File) {
	// 创建匿名管道用于传递参数，将 readPipe 作为子进程的 ExtraFiles，子进程从 readPipe 中读取参数
	// 父进程中则通过 writePipe 将参数写入管道
	// fmt.Println("===New===")
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		log.Errorf("New pipe error %v", err)
		return nil, nil
	}
	// 创建一个新进程，执行 init
	cmd := exec.Command("/proc/self/exe", "init")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
		// | syscall.CLONE_NEWUSER
		// 新的命名空间设置 id 映射，即可以通过非 root 用户来在新 namespace 中使用 root 的权限
		// UidMappings: []syscall.SysProcIDMap{
		// 	{
		// 		ContainerID: 0,
		// 		HostID:      os.Getuid(),
		// 		Size:        1,
		// 	},
		// },
		// GidMappings: []syscall.SysProcIDMap{
		// 	{
		// 		ContainerID: 0,
		// 		HostID:      os.Getgid(),
		// 		Size:        1,
		// 	},
		// },
		Setsid: true,
	}
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	cmd.ExtraFiles = []*os.File{readPipe}
	// 指定 rootfs
	// cmd.Dir = "/root/busybox"
	rootPath := "/root/"
	mntPath := "/root/merged/"
	NewWorkSpace(rootPath, mntPath)
	cmd.Dir = mntPath
	return cmd, writePipe
}

func NewWorkSpace(rootPath string, mntPath string) {
	createLower(rootPath)
	createDirs(rootPath)
	mountOverlayFS(rootPath, mntPath)
}

// Create Lower
func createLower(rootPath string) {
	// 使用 busybox 作为 overlayfs 的 lower 层
	busyboxPath := rootPath + "busybox/"
	busyboxTarPath := rootPath + "busybox.tar"
	// 检查 busybox 路径是否已经存在
	exist, err := PathExists(busyboxPath)
	if err != nil {
		log.Infof("Fail to judge whether dir %s exists. %v", busyboxPath, err)
	}
	// 不存在则创建目录并将 busybox.tar 解压到 busybox 文件夹中
	if !exist {
		if err := os.Mkdir(busyboxPath, 0777); err != nil {
			log.Errorf("Mkdir dir %s error. %v", busyboxPath, err)
		}
		if _, err := exec.Command("tar", "-xvf", busyboxTarPath, "-C", busyboxPath).CombinedOutput(); err != nil {
			log.Errorf("Untar dir %s error %v", busyboxPath, err)
		}
	}
}

// 创建 overlayfs 的 upper 以及 worker 目录
func createDirs(rootPath string) {
	upperURL := rootPath + "upper/"
	if err := os.Mkdir(upperURL, 0777); err != nil {
		log.Errorf("mkdir dir %s error. %v", upperURL, err)
	}
	workURL := rootPath + "work/"
	if err := os.Mkdir(workURL, 0777); err != nil {
		log.Errorf("mkdir dir %s error. %v", workURL, err)
	}
	mergedURL := rootPath + "merged/"
	if err := os.Mkdir(mergedURL, 0777); err != nil {
		log.Errorf("mkdir dir %s error. %v", workURL, err)
	}
}

// 挂载 overlayfs
func mountOverlayFS(rootPath string, mntPath string) {
	// 创建对应挂载目录
	if err := os.Mkdir(mntPath, 0777); err != nil {
		log.Errorf("Mkdir dir %s error. %v", mntPath, err)
	}
	// lowerdir=/root/busybox,upperdir=/root/upper,workdir=/root/merged
	dirs := "lowerdir=" + rootPath + "busybox" + ",upperdir=" + rootPath + "upper" + ",workdir=" + rootPath + "work"
	// mount -t overlay overlay -o lowerdir=/root/busybox,upperdir=/root/upper,workdir=/root/work /root/merged
	cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", dirs, mntPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
}

// 容器退出时删除文件系统
func DeleteWorkSpace(rootPath string, mntPath string) {
	umountOverlayFS(mntPath)
	deleteDirs(rootPath)
}

func umountOverlayFS(mntPath string) {
	cmd := exec.Command("umount", mntPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
	cmd = exec.Command("umount", mntPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
	if err := os.RemoveAll(mntPath); err != nil {
		log.Errorf("Remove dir %s error %v", mntPath, err)
	}
}

func deleteDirs(rootPath string) {
	writePath := rootPath + "upper/"
	if err := os.RemoveAll(writePath); err != nil {
		log.Errorf("Remove dir %s error %v", writePath, err)
	}
	workPath := rootPath + "work"
	if err := os.RemoveAll(workPath); err != nil {
		log.Errorf("Remove dir %s error %v", workPath, err)
	}
}

func PathExists(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
