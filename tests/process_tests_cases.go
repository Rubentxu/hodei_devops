package integration

import "time"

var ProcessTestCases = []ProcessTestCase{
	// Tests básicos de conectividad y comunicación
	{
		Name:      "Basic Echo",
		ProcessID: "echo-basic",
		Command:   []string{"echo", "Hello"},
		Duration:  5 * time.Second,
	},

	// Tests de latencia y timeouts
	{
		Name:      "Slow Process with Network Delay",
		ProcessID: "slow-process",
		Command:   []string{"bash", "-c", "sleep 1 && echo 'Step 1' && sleep 2 && echo 'Step 2'"},
		Duration:  10 * time.Second,
	},

	// Tests de carga y recursos
	{
		Name:      "CPU Intensive",
		ProcessID: "cpu-heavy",
		Command: []string{"bash", "-c", `
				for i in {1..1000000}; do
					if [ $((i % 100000)) -eq 0 ]; then
						echo "Processed $i iterations"
					fi
					echo $i > /dev/null
				done
			`},
		Duration: 20 * time.Second,
	},
	{
		Name:      "CPU Load Monitor",
		ProcessID: "cpu-monitor",
		Command: []string{"bash", "-c", `
				for i in {1..20}; do
					echo "CPU Load Check $i:"
					top -b -n1 | head -n 3
					sleep 1
				done
			`},
		Duration: 25 * time.Second,
	},
	{
		Name:      "System Resources",
		ProcessID: "sys-resources",
		Command: []string{"bash", "-c", `
				echo "=== Memory Info ==="
				free -h
				echo "=== CPU Info ==="
				lscpu | grep "CPU(s):"
				echo "=== Process Count ==="
				ps aux | wc -l
				echo "=== System Load ==="
				uptime
			`},
		Duration: 10 * time.Second,
	},
	{
		Name:      "Memory Intensive",
		ProcessID: "memory-heavy",
		Command:   []string{"bash", "-c", "dd if=/dev/zero of=/tmp/test bs=1M count=1024 && rm /tmp/test"},
		Duration:  30 * time.Second,
	},

	// Tests de E/S y bloqueo
	{
		Name:      "IO Blocking",
		ProcessID: "io-block",
		Command:   []string{"bash", "-c", "dd if=/dev/urandom of=/dev/null bs=1M count=1000"},
		Duration:  15 * time.Second,
	},

	// Tests de manejo de errores y casos límite
	{
		Name:      "Invalid Command",
		ProcessID: "invalid-cmd",
		Command:   []string{"nonexistentcommand"},
		Duration:  5 * time.Second,
	},
	{
		Name:      "Permission Denied",
		ProcessID: "perm-denied",
		Command:   []string{"cat", "/root/secret"},
		Duration:  5 * time.Second,
	},

	// Tests de variables de entorno y contexto
	{
		Name:      "Environment Variables",
		ProcessID: "env-vars",
		Command:   []string{"bash", "-c", "echo $TEST_VAR1-$TEST_VAR2"},
		Env: map[string]string{
			"TEST_VAR1": "value1",
			"TEST_VAR2": "value2",
		},
		Duration: 5 * time.Second,
	},

	// Tests de red y conectividad
	{
		Name:      "Network Connectivity",
		ProcessID: "network-test",
		Command:   []string{"curl", "-s", "http://example.com"},
		Duration:  10 * time.Second,
	},
	{
		Name:      "DNS Resolution",
		ProcessID: "dns-test",
		Command:   []string{"dig", "+short", "google.com"},
		Duration:  10 * time.Second,
	},

	// Tests de procesos anidados y grupos
	{
		Name:      "Process Tree",
		ProcessID: "process-tree",
		Command: []string{"bash", "-c",
			`child_proc() { sleep 2; };
				 child_proc &
				 child_proc &
				 wait`},
		Duration: 15 * time.Second,
	},

	// Tests de límites y recursos
	{
		Name:      "File Descriptors",
		ProcessID: "fd-test",
		Command: []string{"bash", "-c",
			`for i in $(seq 1 1000); do
					exec {fd}>/dev/null;
					eval "exec $fd>&-";
				done`},
		Duration: 10 * time.Second,
	},

	// Tests de recuperación y resiliencia
	{
		Name:      "Process Recovery",
		ProcessID: "recovery-test",
		Command: []string{"bash", "-c",
			`trap 'echo "Recovering..."; sleep 1' ERR;
				 false;
				 echo "Recovered"`},
		Duration: 10 * time.Second,
	},

	// Tests de concurrencia
	{
		Name:      "Concurrent Operations",
		ProcessID: "concurrent-ops",
		Command: []string{"bash", "-c",
			`for i in {1..5}; do
					(echo "Thread $i"; sleep 1) &
				done;
				wait`},
		Duration: 15 * time.Second,
	},

	// Los últimos dos tests son para parada de procesos en paralelo
	{
		Name:      "Long Running Process 1",
		ProcessID: "long-process-1",
		Command: []string{"bash", "-c",
			`trap 'echo "Graceful shutdown 1"' TERM;
				 i=1;
				 while true; do
					echo "Process 1 - iteration $i";
					i=$((i+1));
					sleep 5;
				 done`},
		Duration: 30 * time.Second,
		Timeout:  45 * time.Second,
	},
	{
		Name:      "Long Running Process 2",
		ProcessID: "long-process-2",
		Command: []string{"bash", "-c",
			`trap 'echo "Graceful shutdown 2"' TERM;
				 i=1;
				 while true; do
					echo "Process 2 - iteration $i";
					i=$((i+1));
					sleep 5;
				 done`},
		Duration: 30 * time.Second,
		Timeout:  45 * time.Second,
	},

	// Tests específicos de MonitorHealth
	{
		Name:      "Health Check Basic",
		ProcessID: "health-basic",
		Command:   []string{"sleep", "10"},
		Duration:  12 * time.Second,
	},
	{
		Name:      "Health Check State Changes",
		ProcessID: "health-states",
		Command: []string{"bash", "-c", `
				echo "Starting..."
				sleep 2
				echo "Running..."
				sleep 2
				echo "Finishing..."
				exit 0
			`},
		Duration: 10 * time.Second,
	},
	{
		Name:      "Health Check Error State",
		ProcessID: "health-error",
		Command: []string{"bash", "-c", `
				echo "Starting..."
				sleep 2
				echo "About to fail..."
				exit 1
			`},
		Duration: 8 * time.Second,
	},
	{
		Name:      "Health Check Resource Usage",
		ProcessID: "health-resources",
		Command: []string{"bash", "-c", `
				echo "Starting CPU work..."
				for i in {1..3}; do
					dd if=/dev/zero of=/dev/null bs=1M count=1024
					echo "CPU iteration $i complete"
					sleep 1
				done
			`},
		Duration: 15 * time.Second,
	},
	{
		Name:      "Health Check Signal Handling",
		ProcessID: "health-signals",
		Command: []string{"bash", "-c", `
				trap 'echo "Received SIGTERM"; exit 0' TERM
				echo "Starting..."
				while true; do
					echo "Still running..."
					sleep 1
				done
			`},
		Duration: 10 * time.Second,
	},
	{
		Name:      "Health Check Rapid Status Changes",
		ProcessID: "health-rapid",
		Command: []string{"bash", "-c", `
				for i in {1..5}; do
					echo "State $i"
					sleep 0.5
					if [ $i -eq 3 ]; then
						(dd if=/dev/zero of=/dev/null bs=1M count=512 &)
					fi
				done
			`},
		Duration: 8 * time.Second,
	},
	{
		Name:      "Health Check with Subprocess",
		ProcessID: "health-subprocess",
		Command: []string{"bash", "-c", `
				echo "Parent starting..."
				(sleep 2; echo "Child 1 running..."; sleep 2) &
				(sleep 3; echo "Child 2 running..."; sleep 2) &
				wait
				echo "All processes complete"
			`},
		Duration: 12 * time.Second,
	},
	{
		Name:      "Health Check Network Status",
		ProcessID: "health-network",
		Command: []string{"bash", "-c", `
				echo "Checking network..."
				ping -c 3 8.8.8.8
				curl -s https://api.github.com/zen
				echo "Network checks complete"
			`},
		Duration: 15 * time.Second,
	},
	{
		Name:      "Health Check Memory Pressure",
		ProcessID: "health-memory",
		Command: []string{"bash", "-c", `
				echo "Allocating memory..."
				dd if=/dev/zero of=/tmp/test bs=1M count=512
				echo "Memory allocated"
				sleep 2
				rm /tmp/test
				echo "Memory freed"
			`},
		Duration: 10 * time.Second,
	},
	{
		Name:      "Health Check Concurrent Operations",
		ProcessID: "health-concurrent",
		Command: []string{"bash", "-c", `
				echo "Starting concurrent operations..."
				(dd if=/dev/zero of=/dev/null bs=1M count=256) &
				(ping -c 5 8.8.8.8) &
				(sleep 3; echo "Operation 3") &
				wait
				echo "All operations complete"
			`},
		Duration: 15 * time.Second,
	},
}
