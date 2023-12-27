package subsystems

// 内存限制
// cpu 时间片限制
// cpu 权重设置
// cpu 亲合度设置
type ResourceConfig struct {
	MemoryLimit string
	CpuCfsQuota int
	CpuShare    string
	CpuSet      string
}

type Subsystem interface {
	// 返回 subsystem 的名字
	Name() string
	// 设置某个 cgroup 的参数
	Set(cgroupPath string, cfg *ResourceConfig) error
	// 将某个进程添加到 cgroup 中
	Apply(cgroupPath string, pid int, cfg *ResourceConfig) error
	// 删除某个 cgroup
	Remove(cgroupPath string) error
}

// 通过不同的 subsystem 初始化实例创建资源限制组
var SubsystemInts = []Subsystem{
	&CpuSubsystem{},
	&CpusetSubsystem{},
	&MemorySubsystem{},
}
