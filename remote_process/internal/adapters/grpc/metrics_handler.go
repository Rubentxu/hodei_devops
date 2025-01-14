package grpc

import (
	"dev.rubentxu.devops-platform/protos/remote_process"
	"dev.rubentxu.devops-platform/remote_process/internal/domain"
	"dev.rubentxu.devops-platform/remote_process/internal/ports"
)

type MetricsHandler struct {
	metricsService ports.MetricsService
}

func NewMetricsHandler(service ports.MetricsService) *MetricsHandler {
	return &MetricsHandler{
		metricsService: service,
	}
}

func (h *MetricsHandler) CollectMetrics(stream remote_process.RemoteProcessService_CollectMetricsServer) error {
	// Recibir la solicitud inicial
	req, err := stream.Recv()
	if err != nil {
		return err
	}

	// Iniciar la recolección de métricas
	metricsChan, err := h.metricsService.StreamMetrics(
		stream.Context(),
		req.WorkerId,
		req.MetricTypes,
		req.Interval,
	)
	if err != nil {
		return err
	}

	// Enviar métricas al cliente
	for metrics := range metricsChan {
		protoMetrics := convertToProtoMetrics(metrics)
		if err := stream.Send(protoMetrics); err != nil {
			return err
		}
	}

	return nil
}

func convertToProtoMetrics(metrics domain.WorkerMetrics) *remote_process.WorkerMetrics {
	return &remote_process.WorkerMetrics{
		WorkerId:  metrics.WorkerID,
		Timestamp: metrics.Timestamp,
		Cpu:       convertCPUMetrics(metrics.CPU),
		Memory:    convertMemoryMetrics(metrics.Memory),
		Disks:     convertDiskMetrics(metrics.Disks),
		Networks:  convertNetworkMetrics(metrics.Networks),
		System:    convertSystemMetrics(metrics.System),
		Processes: convertProcessMetrics(metrics.Processes),
		Io:        convertIOMetrics(metrics.IO),
	}
}

func convertCPUMetrics(cpu *domain.CPUMetrics) *remote_process.CPUMetrics {
	if cpu == nil {
		return nil
	}

	cores := make([]*remote_process.CPUCoreMetrics, len(cpu.Cores))
	for i, core := range cpu.Cores {
		cores[i] = &remote_process.CPUCoreMetrics{
			CoreId:       core.CoreID,
			UsagePercent: core.UsagePercent,
			FrequencyMhz: core.FrequencyMhz,
			Temperature:  convertTemperature(core.Temperature),
			Times:        convertCPUTime(core.Times),
		}
	}

	return &remote_process.CPUMetrics{
		TotalUsagePercent: cpu.TotalUsagePercent,
		Cores:             cores,
		LoadAverage:       cpu.LoadAverage,
		Temperature:       convertTemperature(cpu.Temperature),
		CoreCount:         cpu.CoreCount,
		ThreadCount:       cpu.ThreadCount,
		Times:             convertCPUTimes(cpu.Times),
	}
}

func convertMemoryMetrics(mem *domain.MemoryMetrics) *remote_process.MemoryMetrics {
	if mem == nil {
		return nil
	}

	vmStats := make([]*remote_process.VirtualMemoryMetrics, len(mem.VMStats))
	for i, vm := range mem.VMStats {
		vmStats[i] = &remote_process.VirtualMemoryMetrics{
			SwapInRate:  vm.SwapInRate,
			SwapOutRate: vm.SwapOutRate,
			DirtyPages:  vm.DirtyPages,
			Slab:        vm.Slab,
			Mapped:      vm.Mapped,
			Active:      vm.Active,
			Inactive:    vm.Inactive,
		}
	}

	return &remote_process.MemoryMetrics{
		Total:     mem.Total,
		Used:      mem.Used,
		Free:      mem.Free,
		Shared:    mem.Shared,
		Buffers:   mem.Buffers,
		Cached:    mem.Cached,
		Available: mem.Available,
		Swap:      convertSwapMetrics(mem.Swap),
		VmStats:   vmStats,
	}
}

func convertDiskMetrics(disks []domain.DiskMetrics) []*remote_process.DiskMetrics {
	result := make([]*remote_process.DiskMetrics, len(disks))
	for i, disk := range disks {
		result[i] = &remote_process.DiskMetrics{
			Device:       disk.Device,
			MountPoint:   disk.MountPoint,
			FsType:       disk.FSType,
			Total:        disk.Total,
			Used:         disk.Used,
			Free:         disk.Free,
			UsagePercent: disk.UsagePercent,
			InodesTotal:  disk.InodesTotal,
			InodesUsed:   disk.InodesUsed,
			InodesFree:   disk.InodesFree,
			Io:           convertDiskIOMetrics(disk.IO),
		}
	}
	return result
}

func convertNetworkMetrics(networks []domain.NetworkMetrics) []*remote_process.NetworkMetrics {
	result := make([]*remote_process.NetworkMetrics, len(networks))
	for i, net := range networks {
		result[i] = &remote_process.NetworkMetrics{
			Interface:      net.Interface,
			BytesSent:      net.BytesSent,
			BytesRecv:      net.BytesRecv,
			PacketsSent:    net.PacketsSent,
			PacketsRecv:    net.PacketsRecv,
			ErrIn:          net.ErrIn,
			ErrOut:         net.ErrOut,
			DropIn:         net.DropIn,
			DropOut:        net.DropOut,
			IpAddress:      net.IPAddress,
			MacAddress:     net.MACAddress,
			BandwidthUsage: net.BandwidthUsage,
			Status:         convertNetworkStatus(net.Status),
		}
	}
	return result
}

func convertSystemMetrics(sys *domain.SystemMetrics) *remote_process.SystemMetrics {
	if sys == nil {
		return nil
	}

	return &remote_process.SystemMetrics{
		Hostname:     sys.Hostname,
		Os:           sys.OS,
		Platform:     sys.Platform,
		Kernel:       sys.Kernel,
		Uptime:       sys.Uptime,
		ProcessCount: sys.ProcessCount,
		ThreadCount:  sys.ThreadCount,
		UserCount:    sys.UserCount,
		Users:        sys.Users,
		BootTime:     sys.BootTime,
	}
}

func convertProcessMetrics(processes []domain.ProcessMetrics) []*remote_process.ProcessMetrics {
	result := make([]*remote_process.ProcessMetrics, len(processes))
	for i, proc := range processes {
		result[i] = &remote_process.ProcessMetrics{
			Pid:        proc.PID,
			Name:       proc.Name,
			Status:     proc.Status,
			CpuPercent: proc.CPUPercent,
			MemoryRss:  proc.MemoryRSS,
			MemoryVms:  proc.MemoryVMS,
			Username:   proc.Username,
			Threads:    proc.Threads,
			Fds:        proc.FDs,
			Cmdline:    proc.Cmdline,
			Priority:   proc.Priority,
			Nice:       proc.Nice,
			IoCounters: convertProcessIOCounters(proc.IOCounters),
			ParentPid:  proc.ParentPID,
		}
	}
	return result
}

func convertIOMetrics(io *domain.IOMetrics) *remote_process.IOMetrics {
	if io == nil {
		return nil
	}

	return &remote_process.IOMetrics{
		ReadBytesTotal:  io.ReadBytesTotal,
		WriteBytesTotal: io.WriteBytesTotal,
		ReadSpeed:       io.ReadSpeed,
		WriteSpeed:      io.WriteSpeed,
		ActiveRequests:  io.ActiveRequests,
		QueueLength:     io.QueueLength,
	}
}

// Funciones auxiliares de conversión
func convertTemperature(temp domain.Temperature) *remote_process.Temperature {
	return &remote_process.Temperature{
		Current:  temp.Current,
		Critical: temp.Critical,
		Unit:     temp.Unit,
	}
}

func convertCPUTime(time domain.CPUTime) *remote_process.CPUTime {
	return &remote_process.CPUTime{
		User:    time.User,
		System:  time.System,
		Idle:    time.Idle,
		Nice:    time.Nice,
		Iowait:  time.Iowait,
		Irq:     time.Irq,
		Softirq: time.Softirq,
		Steal:   time.Steal,
		Guest:   time.Guest,
	}
}

func convertCPUTimes(times []domain.CPUTime) []*remote_process.CPUTime {
	result := make([]*remote_process.CPUTime, len(times))
	for i, time := range times {
		result[i] = convertCPUTime(time)
	}
	return result
}

func convertSwapMetrics(swap domain.SwapMetrics) *remote_process.SwapMetrics {
	return &remote_process.SwapMetrics{
		Total:   swap.Total,
		Used:    swap.Used,
		Free:    swap.Free,
		SwapIn:  swap.SwapIn,
		SwapOut: swap.SwapOut,
	}
}

func convertDiskIOMetrics(io domain.DiskIOMetrics) *remote_process.DiskIOMetrics {
	return &remote_process.DiskIOMetrics{
		ReadsCompleted:  io.ReadsCompleted,
		WritesCompleted: io.WritesCompleted,
		ReadBytes:       io.ReadBytes,
		WriteBytes:      io.WriteBytes,
		ReadTime:        io.ReadTime,
		WriteTime:       io.WriteTime,
		IoTime:          io.IOTime,
		WeightedIo:      io.WeightedIO,
	}
}

func convertNetworkStatus(status domain.NetworkStatus) *remote_process.NetworkStatus {
	return &remote_process.NetworkStatus{
		IsUp:      status.IsUp,
		IsRunning: status.IsRunning,
		Duplex:    status.Duplex,
		Speed:     status.Speed,
		Mtu:       status.MTU,
	}
}

func convertProcessIOCounters(io domain.ProcessIOCounters) *remote_process.ProcessIOCounters {
	return &remote_process.ProcessIOCounters{
		ReadCount:  io.ReadCount,
		WriteCount: io.WriteCount,
		ReadBytes:  io.ReadBytes,
		WriteBytes: io.WriteBytes,
	}
}
