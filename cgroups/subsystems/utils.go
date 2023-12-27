package subsystems

import (
	"bufio"
	"os"
	"path"
	"strings"

	"mydocker/constant"

	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"
)

const mountPointIndex = 4

// getCgroupPath 找到 cgroup 在文件系统中的绝对路径
/*
	实际就是将根目录和 cgroup 名称拼接成一个路径
	如果指定了自动创建，就先检测一下是否存在，如果对应的目录不存在，则说明cgroup不存在，根据 autoCreate 觉得是否创建
*/
func getCgroupPath(subsystemName string, cgroupPath string, autoCreate bool) (string, error) {
	// cgroup 子系统的根目录路径
	cgroupRootPath := findCgroupMountpoint(subsystemName)
	// 绝对路径
	absPath := path.Join(cgroupRootPath, cgroupPath)
	// 如果不需要创建就直接返回绝对路径
	if !autoCreate {
		return absPath, nil
	}
	// 需要创建时先判断是否存在
	_, err := os.Stat(absPath)
	// 只有不存在才创建，这里如果不存在的话 err 会返回 not exist 的错误
	if err != nil && os.IsNotExist(err) {
		err = os.Mkdir(absPath, constant.Perm0755)
		return absPath, err
	}
	// 其他错误或者没有错误都直接返回，如果 err == nil，那么 errors.Wrap(err, "") 也会是 nil
	return absPath, errors.Wrap(err, "create cgroup")
}

// findCgroupMountpoint 通过 /proc/self/mountinfo 找出挂载了某个 subsystem 的 hierarchy cgroup 根节点所在的目录
func findCgroupMountpoint(subsystem string) string {
	// /proc/self/mountinfo 为当前进程的 mountinfo 信息
	// 直接通过 cat /proc/self/mountinfo 命令查看
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// 格式如下
		// 43 38 0:38 / /sys/fs/cgroup/cpu,cpuacct rw,nosuid,nodev,noexec,relatime shared:16 - cgroup cgroup rw,cpu,cpuacct
		// 54 38 0:49 / /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime shared:27 - cgroup cgroup rw,memory
		txt := scanner.Text()
		fields := strings.Split(txt, " ")
		// 对最后一个元素按逗号进行分割，这里的最后一个元素就是 rw,memory
		// 其中的 memory 就表示这是一个 memory subsystem
		subsystems := strings.Split(fields[len(fields)-1], ",")
		for _, opt := range subsystems {
			if opt == subsystem {
				// 如果等于指定的 subsystem，返回这个挂载点根目录，就是第四个元素
				// 这里就是 /sys/fs/cgroup/memory，即我们要找的根目录
				return fields[mountPointIndex]
			}
		}
	}

	if err = scanner.Err(); err != nil {
		log.Error("read err:", err)
		return ""
	}
	return ""
}
