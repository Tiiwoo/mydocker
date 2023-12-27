package subsystems

import (
	"fmt"
	"mydocker/constant"
	"os"
	"path"
	"strconv"
)

type CpuSubsystem struct{}

const (
	PeriodDefault = 100000
	Percent       = 100
)

func (s *CpuSubsystem) Name() string {
	return "cpu"
}

func (s *CpuSubsystem) Set(cgroupPath string, cfg *ResourceConfig) error {
	// 如果没有包含 cpu 子系统相关的修改，则返回 nil
	if cfg.CpuCfsQuota == 0 && cfg.CpuShare == "" {
		return nil
	}
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, true)
	if err != nil {
		return err
	}

	if cfg.CpuShare != "" {
		if err = os.WriteFile(path.Join(subsysCgroupPath, "cpu.shares"), []byte(cfg.CpuShare), constant.Perm0644); err != nil {
			return fmt.Errorf("set cgroup cpu share failed %v", err)
		}
	}

	if cfg.CpuCfsQuota != 0 {
		if err = os.WriteFile(path.Join(subsysCgroupPath, "cpu.cfs_period_us"), []byte("100000"), constant.Perm0644); err != nil {
			return fmt.Errorf("set cgroup cpu share failed %v", err)
		}
		if err = os.WriteFile(path.Join(subsysCgroupPath, "cpu.cfs_quota_us"), []byte(strconv.Itoa(PeriodDefault/Percent*cfg.CpuCfsQuota)), constant.Perm0644); err != nil {
			return fmt.Errorf("set cgroup cpu share failed %v", err)
		}
	}
	return nil
}

func (s *CpuSubsystem) Apply(cgroupPath string, pid int, cfg *ResourceConfig) error {
	// 确保配置中存在 cpu.shares 或者 cpu.cfs_quota_us，如果没有就直接返回 nil，不进行其他操作
	if cfg.CpuCfsQuota == 0 && cfg.CpuShare == "" {
		return nil
	}

	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return fmt.Errorf("get cgroup %s error: %v", cgroupPath, err)
	}
	if err := os.WriteFile(path.Join(subsysCgroupPath, "tasks"), []byte(strconv.Itoa(pid)), constant.Perm0644); err != nil {
		return fmt.Errorf("add process: %d to cgroup failed %v", pid, err)
	}
	return nil
}

func (s *CpuSubsystem) Remove(cgroupPath string) error {
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return err
	}
	return os.RemoveAll(subsysCgroupPath)
}
