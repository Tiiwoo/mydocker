package container

import (
	"fmt"
	"mydocker/constant"
	"os"
	"os/exec"
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

// 容器目录相关
const (
	RootPath        = "/root/"
	lowerDirFormat  = "/root/%s/lower"
	upperDirFormat  = "/root/%s/upper"
	workDirFormat   = "/root/%s/work"
	mergedDirFormat = "/root/%s/merged"
	overlayFSFormat = "lowerdir=%s,upperdir=%s,workdir=%s"
)

type Info struct {
	Pid         string   `json:"pid"`         // 容器的init进程在宿主机上的 PID
	Id          string   `json:"id"`          // 容器Id
	Name        string   `json:"name"`        // 容器名
	Command     string   `json:"command"`     // 容器内init运行命令
	CreatedTime string   `json:"createTime"`  // 创建时间
	Status      string   `json:"status"`      // 容器的状态
	Volume      string   `json:"volume"`      // 挂载的数据卷
	PortMapping []string `json:"portmapping"` // 端口映射
}

// NewParentProcess 构建 command 用于启动一个新进程
/*
	这里是父进程，也就是当前进程执行的内容
	1. 这里的 /proc/self/exe 调用中，/proc/self/ 指的是当前运行进程自己的环境，exe 其实就是自己调用了自己，使用这种方式对创建出来的进程进行初始化
	2. 后面的 args 是参数，其中 init 是传递给本进程的第一个参数，在本例中，其实就是会去调用 initCommand 去初始化进程的一些环境和资源
	3. 下面的 clone 参数就是去 fork 出来一个新进程，并且使用了 namespace 隔离新创建的进程和外部环境。
	4. 如果用户指定了 -it 参数，就需要把当前进程的输入输出导入到标准输入输出上
*/
func NewParentProcess(tty bool, volume, containerName, imageName string, envSlice []string) (*exec.Cmd, *os.File) {
	// 创建匿名管道用于传递参数，将 readPipe 作为子进程的 ExtraFiles，子进程从 readPipe 中读取参数
	// 父进程中则通过 writePipe 将参数写入管道
	// fmt.Println("===New===")
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		log.Errorf("New pipe error %v", err)
		return nil, nil
	}
	// 创建一个新进程，执行 init
	// cmd := exec.Command("/proc/self/exe", "init")
	initCmd, err := os.Readlink("/proc/self/exe")
	if err != nil {
		log.Errorf("get init process error: %v", err)
		return nil, nil
	}
	cmd := exec.Command(initCmd, "init")
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
	// rootPath := "/root"
	// mntPath := "/root/merged"
	// NewWorkSpace(rootPath, mntPath, volume)
	// cmd.Dir = mntPath
	cmd.Dir = fmt.Sprintf(mergedDirFormat, containerName)
	cmd.Env = append(os.Environ(), envSlice...)
	NewWorkSpace(volume, imageName, containerName)
	return cmd, writePipe
}
