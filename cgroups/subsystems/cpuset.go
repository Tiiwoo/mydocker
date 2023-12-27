package subsystems

import (
	"fmt"
	"mydocker/constant"
	"os"
	"path"
	"strconv"

	"github.com/pkg/errors"
)

type CpusetSubsystem struct{}

func (s *CpusetSubsystem) Name() string {
	return "cpuset"
}

func (s *CpusetSubsystem) Set(cgroupPath string, cfg *ResourceConfig) error {
	if cfg.CpuSet == "" {
		return nil
	}
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, true)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(subsysCgroupPath, "cpuset.cpus"), []byte(cfg.CpuSet), constant.Perm0644); err != nil {
		return fmt.Errorf("set cgroup cpuset failed %v", err)
	}
	return nil
}

func (s *CpusetSubsystem) Apply(cgroupPath string, pid int, cfg *ResourceConfig) error {
	if cfg.CpuSet == "" {
		return nil
	}
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return errors.Wrapf(err, "get cgroup %s", cgroupPath)
	}
	if err := os.WriteFile(path.Join(subsysCgroupPath, "tasks"), []byte(strconv.Itoa(pid)), constant.Perm0644); err != nil {
		return fmt.Errorf("add process: %d to cgroup failed %v", pid, err)
	}
	return nil
}

func (s *CpusetSubsystem) Remove(cgroupPath string) error {
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return err
	}
	return os.RemoveAll(subsysCgroupPath)
}
