package domain

// WorkerMetrics representa las métricas recopiladas del worker
type WorkerMetrics struct {
	WorkerID  string
	Timestamp int64
	CPU       *CPUMetrics
	Memory    *MemoryMetrics
	Disks     []DiskMetrics
	Networks  []NetworkMetrics
	System    *SystemMetrics
	Processes []ProcessMetrics
	IO        *IOMetrics
}

// CPU Metrics
type CPUMetrics struct {
	TotalUsagePercent float64
	Cores             []CPUCoreMetrics
	LoadAverage       map[string]float64
	Temperature       Temperature
	CoreCount         int32
	ThreadCount       int32
	Times             []CPUTime
}

type CPUCoreMetrics struct {
	CoreID       int32
	UsagePercent float64
	FrequencyMhz float64
	Temperature  Temperature
	Times        CPUTime
}

type CPUTime struct {
	User    float64
	System  float64
	Idle    float64
	Nice    float64
	Iowait  float64
	Irq     float64
	Softirq float64
	Steal   float64
	Guest   float64
}

// Memory Metrics
type MemoryMetrics struct {
	Total     uint64
	Used      uint64
	Free      uint64
	Shared    uint64
	Buffers   uint64
	Cached    uint64
	Available uint64
	Swap      SwapMetrics
	VMStats   []VirtualMemoryMetrics
}

type SwapMetrics struct {
	Total   uint64
	Used    uint64
	Free    uint64
	SwapIn  float64
	SwapOut float64
}

type VirtualMemoryMetrics struct {
	PageTables  uint64
	SwapInRate  float64
	SwapOutRate float64
	DirtyPages  uint64
	Slab        uint64
	Mapped      uint64
	Active      uint64
	Inactive    uint64
}

// Disk Metrics
type DiskMetrics struct {
	Device       string
	MountPoint   string
	FSType       string
	Total        uint64
	Used         uint64
	Free         uint64
	UsagePercent float64
	InodesTotal  uint64
	InodesUsed   uint64
	InodesFree   uint64
	IO           DiskIOMetrics
}

type DiskIOMetrics struct {
	ReadsCompleted  uint64
	WritesCompleted uint64
	ReadBytes       uint64
	WriteBytes      uint64
	ReadTime        float64
	WriteTime       float64
	IOTime          uint64
	WeightedIO      uint64
}

// Network Metrics
type NetworkMetrics struct {
	Interface      string
	BytesSent      uint64
	BytesRecv      uint64
	PacketsSent    uint64
	PacketsRecv    uint64
	ErrIn          uint64 `proto:"errin"`
	ErrOut         uint64 `proto:"errout"`
	DropIn         uint64 `proto:"dropin"`
	DropOut        uint64 `proto:"dropout"`
	IPAddress      string
	MACAddress     string
	BandwidthUsage float64
	Status         NetworkStatus
}

type NetworkStatus struct {
	IsUp      bool
	IsRunning bool
	Duplex    string
	Speed     uint64
	MTU       string
}

// System Metrics
type SystemMetrics struct {
	Hostname     string
	OS           string
	Platform     string
	Kernel       string
	Uptime       int32
	ProcessCount int32
	ThreadCount  int32
	UserCount    int32
	Users        []string
	BootTime     string
}

// Process Metrics
type ProcessMetrics struct {
	PID        int32
	Name       string
	Status     string
	CPUPercent float64
	MemoryRSS  uint64
	MemoryVMS  uint64
	Username   string
	Threads    int32
	FDs        int32
	Cmdline    string
	Priority   int32
	Nice       int32
	IOCounters ProcessIOCounters
	ParentPID  int32
}

type ProcessIOCounters struct {
	ReadCount  uint64
	WriteCount uint64
	ReadBytes  uint64
	WriteBytes uint64
}

// IO Metrics
type IOMetrics struct {
	ReadBytesTotal  uint64
	WriteBytesTotal uint64
	ReadSpeed       float64
	WriteSpeed      float64
	ActiveRequests  int32
	QueueLength     float64
}

// Common types
type Temperature struct {
	Current  float64
	Critical float64
	Unit     string
}
