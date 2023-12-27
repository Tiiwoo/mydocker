package cgroups

import (
	"mydocker/cgroups/subsystems"

	"github.com/sirupsen/logrus"
)

// Path 为创建的 cgroup 相对于 root cgroup 的路径
// Resource 则为需要初始化的配置
type CgroupManager struct {
	Path     string
	Resource *subsystems.ResourceConfig
}

func NewCgroupManager(path string) *CgroupManager {
	return &CgroupManager{
		Path: path,
	}
}

// 将进程加入到 cgroup 中
func (c *CgroupManager) Apply(pid int, cfg *subsystems.ResourceConfig) error {
	// 将 pid 加入到已有的每个子系统中
	for _, subsysIns := range subsystems.SubsystemInts {
		if err := subsysIns.Apply(c.Path, pid, cfg); err != nil {
			logrus.Errorf("apply subsystem: %s, err: %s", subsysIns.Name(), err)
		}
	}
	return nil
}

// 设置和创建 cgroup
func (c *CgroupManager) Set(cfg *subsystems.ResourceConfig) error {
	for _, subsysIns := range subsystems.SubsystemInts {
		if err := subsysIns.Set(c.Path, cfg); err != nil {
			logrus.Errorf("apply subsystem: %s, err: %s", subsysIns.Name(), err)
		}
	}
	return nil
}

// 删除释放 cgroup
func (c *CgroupManager) Destroy() error {
	for _, subsysIns := range subsystems.SubsystemInts {
		if err := subsysIns.Remove(c.Path); err != nil {
			logrus.Warnf("remove cgroup failed %v", err)
		}
	}
	return nil
}
