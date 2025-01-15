package metrics

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"

	"dev.rubentxu.devops-platform/remote_process/internal/domain"
)

type GopsutilCollector struct {
	lastCPUTimes []cpu.TimesStat
	lastIOTime   time.Time
}

func NewGopsutilCollector() *GopsutilCollector {
	return &GopsutilCollector{
		lastIOTime: time.Now(),
	}
}

func (g *GopsutilCollector) CollectCPUMetrics(ctx context.Context) (*domain.CPUMetrics, error) {
	percentages, err := cpu.PercentWithContext(ctx, 0, false)
	if err != nil {
		return nil, fmt.Errorf("error getting CPU percentages: %w", err)
	}

	cores, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting CPU info: %w", err)
	}

	loadAvg, err := load.AvgWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting load average: %w", err)
	}

	times, err := cpu.TimesWithContext(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("error getting CPU times: %w", err)
	}

	cpuMetrics := &domain.CPUMetrics{
		TotalUsagePercent: percentages[0],
		CoreCount:         int32(len(cores)),
		ThreadCount:       int32(cores[0].Cores),
		LoadAverage: map[string]float64{
			"1min":  loadAvg.Load1,
			"5min":  loadAvg.Load5,
			"15min": loadAvg.Load15,
		},
		Times: make([]domain.CPUTime, len(times)),
	}

	// Convertir tiempos de CPU
	for i, t := range times {
		cpuMetrics.Times[i] = domain.CPUTime{
			User:    t.User,
			System:  t.System,
			Idle:    t.Idle,
			Nice:    t.Nice,
			Iowait:  t.Iowait,
			Irq:     t.Irq,
			Softirq: t.Softirq,
			Steal:   t.Steal,
			Guest:   t.Guest,
		}
	}

	return cpuMetrics, nil
}

func (g *GopsutilCollector) CollectMemoryMetrics(ctx context.Context) (*domain.MemoryMetrics, error) {
	v, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting virtual memory: %w", err)
	}

	swap, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting swap memory: %w", err)
	}

	return &domain.MemoryMetrics{
		Total:     v.Total,
		Used:      v.Used,
		Free:      v.Free,
		Shared:    v.Shared,
		Buffers:   v.Buffers,
		Cached:    v.Cached,
		Available: v.Available,
		Swap: domain.SwapMetrics{
			Total:   swap.Total,
			Used:    swap.Used,
			Free:    swap.Free,
			SwapIn:  float64(swap.Sin),
			SwapOut: float64(swap.Sout),
		},
		VMStats: []domain.VirtualMemoryMetrics{
			{
				Active:      v.Active,
				Inactive:    v.Inactive,
				SwapInRate:  float64(swap.Sin) / time.Since(g.lastIOTime).Seconds(),
				SwapOutRate: float64(swap.Sout) / time.Since(g.lastIOTime).Seconds(),
				DirtyPages:  v.Dirty,
				Mapped:      v.Mapped,
				Slab:        v.Slab,
			},
		},
	}, nil
}

func (g *GopsutilCollector) CollectDiskMetrics(ctx context.Context) ([]domain.DiskMetrics, error) {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("error getting disk partitions: %w", err)
	}

	var metrics []domain.DiskMetrics
	for _, partition := range partitions {
		usage, err := disk.UsageWithContext(ctx, partition.Mountpoint)
		if err != nil {
			log.Printf("Error getting usage for partition %s: %v", partition.Mountpoint, err)
			continue
		}

		ioCounters, err := disk.IOCountersWithContext(ctx, partition.Device)
		if err != nil {
			log.Printf("Error getting IO counters for device %s: %v", partition.Device, err)
			continue
		}

		var counter disk.IOCountersStat
		if ioCounters != nil {
			counter = ioCounters[partition.Device]
		}

		metrics = append(metrics, domain.DiskMetrics{
			Device:       partition.Device,
			MountPoint:   partition.Mountpoint,
			FSType:       partition.Fstype,
			Total:        usage.Total,
			Used:         usage.Used,
			Free:         usage.Free,
			UsagePercent: usage.UsedPercent,
			InodesTotal:  usage.InodesTotal,
			InodesUsed:   usage.InodesUsed,
			InodesFree:   usage.InodesFree,
			IO: domain.DiskIOMetrics{
				ReadsCompleted:  counter.ReadCount,
				WritesCompleted: counter.WriteCount,
				ReadBytes:       counter.ReadBytes,
				WriteBytes:      counter.WriteBytes,
				ReadTime:        float64(counter.ReadTime),
				WriteTime:       float64(counter.WriteTime),
				IOTime:          counter.IoTime,
				WeightedIO:      counter.WeightedIO,
			},
		})
	}

	return metrics, nil
}

func (g *GopsutilCollector) CollectNetworkMetrics(ctx context.Context) ([]domain.NetworkMetrics, error) {
	// Obtener interfaces de red
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("error getting network interfaces: %w", err)
	}

	// Obtener estadísticas de IO
	ioStats, err := net.IOCounters(true)
	if err != nil {
		return nil, fmt.Errorf("error getting network IO stats: %w", err)
	}

	var metrics []domain.NetworkMetrics
	for _, iface := range interfaces {
		// Buscar estadísticas correspondientes
		var stats *net.IOCountersStat
		for _, s := range ioStats {
			if s.Name == iface.Name {
				stats = &s
				break
			}
		}

		if stats == nil {
			continue // Skip interfaces without stats
		}

		metric := domain.NetworkMetrics{
			Interface:   iface.Name,
			BytesSent:   stats.BytesSent,
			BytesRecv:   stats.BytesRecv,
			PacketsSent: stats.PacketsSent,
			PacketsRecv: stats.PacketsRecv,
			IPAddress:   getIPAddress(iface),
			MACAddress:  iface.HardwareAddr,
			Status: domain.NetworkStatus{
				IsUp:      containsFlag(iface.Flags, "up"),
				IsRunning: containsFlag(iface.Flags, "running"),
				MTU:       fmt.Sprintf("%d", iface.MTU),
			},
			BandwidthUsage: calculateBandwidthUsage(stats),
		}

		metrics = append(metrics, metric)
	}

	return metrics, nil
}

func calculateBandwidthUsage(stats *net.IOCountersStat) float64 {
	// Implementar cálculo de uso de ancho de banda
	// Por ejemplo, porcentaje del máximo teórico o del histórico
	return 0.0 // Por ahora retornamos 0
}

func containsFlag(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

func (g *GopsutilCollector) CollectSystemMetrics(ctx context.Context) (*domain.SystemMetrics, error) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting host info: %w", err)
	}

	users, err := host.UsersWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting users: %w", err)
	}

	usernames := make([]string, len(users))
	for i, user := range users {
		usernames[i] = user.User
	}

	return &domain.SystemMetrics{
		Hostname:     info.Hostname,
		OS:           info.OS,
		Platform:     info.Platform,
		Kernel:       info.KernelVersion,
		Uptime:       int32(info.Uptime),
		ProcessCount: int32(info.Procs),
		UserCount:    int32(len(users)),
		Users:        usernames,
		BootTime:     time.Unix(int64(info.BootTime), 0).String(),
	}, nil
}

func (g *GopsutilCollector) CollectProcessMetrics(ctx context.Context) ([]domain.ProcessMetrics, error) {
	processes, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting processes: %w", err)
	}

	var metrics []domain.ProcessMetrics
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			log.Printf("Error getting name for PID %d: %v", p.Pid, err)
			continue
		}

		status, err := p.Status()
		if err != nil {
			log.Printf("Error getting status for PID %d: %v", p.Pid, err)
			continue
		}

		cpu, err := p.CPUPercent()
		if err != nil {
			log.Printf("Error getting CPU percent for PID %d: %v", p.Pid, err)
			cpu = 0.0 // Usar un valor por defecto en caso de error
		}

		memInfo, err := p.MemoryInfo()
		if err != nil {
			log.Printf("Error getting memory info for PID %d: %v", p.Pid, err)
			continue
		}

		username, err := p.Username()
		if err != nil {
			log.Printf("Error getting username for PID %d: %v", p.Pid, err)
			username = "unknown"
		}

		numThreads, err := p.NumThreads()
		if err != nil {
			log.Printf("Error getting thread count for PID %d: %v", p.Pid, err)
			numThreads = 0
		}

		metrics = append(metrics, domain.ProcessMetrics{
			PID:        int32(p.Pid),
			Name:       name,
			Status:     status[0],
			CPUPercent: cpu,
			MemoryRSS:  memInfo.RSS,
			MemoryVMS:  memInfo.VMS,
			Username:   username,
			Threads:    int32(numThreads),
		})
	}

	return metrics, nil
}

func (g *GopsutilCollector) CollectIOMetrics(ctx context.Context) (*domain.IOMetrics, error) {
	counters, err := disk.IOCountersWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting IO counters: %w", err)
	}

	now := time.Now()
	timeDiff := now.Sub(g.lastIOTime).Seconds()
	g.lastIOTime = now

	var totalRead, totalWrite uint64
	var activeRequests int32
	for _, counter := range counters {
		totalRead += counter.ReadBytes
		totalWrite += counter.WriteBytes
		activeRequests += int32(counter.IoTime)
	}

	return &domain.IOMetrics{
		ReadBytesTotal:  totalRead,
		WriteBytesTotal: totalWrite,
		ReadSpeed:       float64(totalRead) / timeDiff,
		WriteSpeed:      float64(totalWrite) / timeDiff,
		ActiveRequests:  activeRequests,
		QueueLength:     float64(activeRequests) / timeDiff,
	}, nil
}

func getIPAddress(iface net.InterfaceStat) string {
	if len(iface.Addrs) > 0 {
		return iface.Addrs[0].Addr
	}
	return ""
}
