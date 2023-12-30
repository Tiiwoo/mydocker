package container

import (
	"fmt"
	"mydocker/constant"
	"os"
	"os/exec"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
)

const (
	RUNNING       = "running"
	STOP          = "stopped"
	Exit          = "exited"
	InfoLoc       = "/var/run/mydocker/"
	InfoLocFormat = InfoLoc + "%s/"
	ConfigName    = "config.json"
	IDLength      = 10
	Logfile       = "container.log"
)

type Info struct {
	Pid         string `json:"pid"`        // 容器的init进程在宿主机上的 PID
	Id          string `json:"id"`         // 容器Id
	Name        string `json:"name"`       // 容器名
	Command     string `json:"command"`    // 容器内init运行命令
	CreatedTime string `json:"createTime"` // 创建时间
	Status      string `json:"status"`     // 容器的状态
}

// NewParentProcess 构建 command 用于启动一个新进程
/*
	这里是父进程，也就是当前进程执行的内容
	1. 这里的 /proc/self/exe 调用中，/proc/self/ 指的是当前运行进程自己的环境，exe 其实就是自己调用了自己，使用这种方式对创建出来的进程进行初始化
	2. 后面的 args 是参数，其中 init 是传递给本进程的第一个参数，在本例中，其实就是会去调用 initCommand 去初始化进程的一些环境和资源
	3. 下面的 clone 参数就是去 fork 出来一个新进程，并且使用了 namespace 隔离新创建的进程和外部环境。
	4. 如果用户指定了 -it 参数，就需要把当前进程的输入输出导入到标准输入输出上
*/
func NewParentProcess(tty bool, volume, containerName string) (*exec.Cmd, *os.File) {
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
	} else {
		// 后台运行的容器，将输出到日志中
		dirPath := fmt.Sprintf(InfoLocFormat, containerName)
		if err := os.MkdirAll(dirPath, constant.Perm0622); err != nil {
			log.Errorf("NewParentProcess mkdir %s error: %v", dirPath, err)
			return nil, nil
		}
		stdLogFilePath := dirPath + Logfile
		stdLogFile, err := os.Create(stdLogFilePath)
		if err != nil {
			log.Errorf("NewParantProcess create file %s error: %v", stdLogFilePath, err)
		}
		cmd.Stdout = stdLogFile
	}
	cmd.ExtraFiles = []*os.File{readPipe}
	// 指定 rootfs
	// cmd.Dir = "/root/busybox"
	rootPath := "/root"
	mntPath := "/root/merged"
	NewWorkSpace(rootPath, mntPath, volume)
	cmd.Dir = mntPath
	return cmd, writePipe
}

func NewWorkSpace(rootPath, mntPath, volume string) {
	createLower(rootPath)
	createDirs(rootPath)
	mountOverlayFS(rootPath, mntPath)
	if volume != "" {
		volumePaths := volumePathExtract(volume)
		if len(volumePaths) == 2 && volumePaths[0] != "" && volumePaths[1] != "" {
			mountVolume(rootPath, mntPath, volumePaths)
			log.Infof("volumePath: %s", volumePaths)
		} else {
			log.Infof("volume parameter input is not correct.")
		}
	}
}

// volumePathExtract 通过冒号分割解析 volume 目录，比如 -v /tmp:/tmp
func volumePathExtract(volume string) []string {
	volumePaths := strings.Split(volume, ":")
	return volumePaths
}

func mountVolume(rootPath, mntPath string, volumePaths []string) {
	// 第 0 个元素为宿主机目录
	parentPath := volumePaths[0]
	// 先判断宿主机目录是否已经创建
	if _, err := os.Stat(parentPath); os.IsNotExist(err) {
		err := os.Mkdir(parentPath, constant.Perm0777)
		if err != nil {
			log.Infof("mkdir parent dir %s error: %v", parentPath, err)
		}
	}
	// 第 1 个元素为容器内部目录
	containerPath := volumePaths[1]
	containerVolumePath := mntPath + "/" + containerPath
	if err := os.Mkdir(containerVolumePath, constant.Perm0777); err != nil {
		log.Infof("mkdir container dir %s error: %v", containerVolumePath, err)
	}
	// 通过 bind mount 将宿主机目录挂载到容器中
	// mount -o bind /hostPath /containerPath
	cmd := exec.Command("mount", "-o", "bind", parentPath, containerVolumePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("mount volume failed: %v", err)
	}
}

// Create Lower
func createLower(rootPath string) {
	// 使用 busybox 作为 overlayfs 的 lower 层
	busyboxPath := rootPath + "/busybox"
	busyboxTarPath := rootPath + "/busybox.tar"
	// 检查 busybox 路径是否已经存在
	exist, err := PathExists(busyboxPath)
	if err != nil {
		log.Infof("fail to judge whether dir %s exists: %v", busyboxPath, err)
	}
	// 不存在则创建目录并将 busybox.tar 解压到 busybox 文件夹中
	if !exist {
		if err := os.Mkdir(busyboxPath, constant.Perm0777); err != nil {
			log.Errorf("mkdir dir %s error: %v", busyboxPath, err)
		}
		if _, err := exec.Command("tar", "-xvf", busyboxTarPath, "-C", busyboxPath).CombinedOutput(); err != nil {
			log.Errorf("untar dir %s error: %v", busyboxPath, err)
		}
	}
}

// 创建 overlayfs 的 upper 以及 worker 目录
func createDirs(rootPath string) {
	upperURL := rootPath + "/upper"
	if err := os.Mkdir(upperURL, constant.Perm0777); err != nil {
		log.Errorf("mkdir dir %s error: %v", upperURL, err)
	}
	workURL := rootPath + "/work"
	if err := os.Mkdir(workURL, constant.Perm0777); err != nil {
		log.Errorf("mkdir dir %s error: %v", workURL, err)
	}
	// mergedURL := rootPath + "/merged"
	// if err := os.Mkdir(mergedURL, constant.Perm0777); err != nil {
	// 	log.Errorf("mkdir dir %s error: %v", workURL, err)
	// }
}

// 挂载 overlayfs
func mountOverlayFS(rootPath string, mntPath string) {
	// 创建对应挂载目录
	// fmt.Println("Mount OverlayFS")
	if err := os.Mkdir(mntPath, constant.Perm0777); err != nil {
		log.Errorf("mkdir dir %s error: %v", mntPath, err)
	}
	// lowerdir=/root/busybox,upperdir=/root/upper,workdir=/root/merged
	dirs := "lowerdir=" + rootPath + "/busybox" + ",upperdir=" + rootPath + "/upper" + ",workdir=" + rootPath + "/work"
	// mount -t overlay overlay -o lowerdir=/root/busybox,upperdir=/root/upper,workdir=/root/work /root/merged
	cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", dirs, mntPath)
	log.Infof("mountOverlayFS cmd: %s", cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("mountOverlayFS mount error: %v", err)
	}
}

// 容器退出时删除文件系统
func DeleteWorkSpace(rootPath string, mntPath string, volume string) {
	// 先判断是否有 volume 挂载，如果有则要先 umount volume
	if volume != "" {
		volumePaths := volumePathExtract(volume)
		l := len(volumePaths)
		if l == 2 && volumePaths[0] != "" && volumePaths[1] != "" {
			umountVolume(mntPath, volumePaths)
		}
	}
	umountOverlayFS(mntPath)
	deleteDirs(rootPath)
}

func umountVolume(mntPath string, volumePaths []string) {
	containerPath := mntPath + "/" + volumePaths[1]
	cmd := exec.Command("umount", containerPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("umount volume failed: %v", err)
	}
}

func umountOverlayFS(mntPath string) {
	cmd := exec.Command("umount", mntPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
	if err := os.RemoveAll(mntPath); err != nil {
		log.Errorf("remove dir %s error: %v", mntPath, err)
	}
}

func deleteDirs(rootPath string) {
	writePath := rootPath + "/upper"
	if err := os.RemoveAll(writePath); err != nil {
		log.Errorf("remove dir %s error: %v", writePath, err)
	}
	workPath := rootPath + "/work"
	if err := os.RemoveAll(workPath); err != nil {
		log.Errorf("remove dir %s error: %v", workPath, err)
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
