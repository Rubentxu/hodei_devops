package integration

import (
	"fmt"
	"testing"
	"time"
)

func TestProcessExecution(t *testing.T) {
	env, err := setupTestEnvironment(t)
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Obtener la dirección del cliente
	clientHost, err := env.ClientContainer.Host(env.Context)
	if err != nil {
		t.Fatalf("Failed to get client host: %v", err)
	}
	clientPort, err := env.ClientContainer.MappedPort(env.Context, "8080")
	if err != nil {
		t.Fatalf("Failed to get client port: %v", err)
	}

	// Ejecutar los tests usando el cliente HTTP
	baseURL := fmt.Sprintf("http://%s:%s", clientHost, clientPort.Port())
	testRunner := NewTestRunner(baseURL)

	// Definir los casos de prueba
	testCases := []TestCase{
		// Tests básicos
		{
			Name:      "Simple Echo",
			ProcessID: "echo-test",
			Command:   []string{"echo", "Hello, World!"},
			Duration:  5 * time.Second,
		},
		{
			Name:      "Multiple Commands Pipeline",
			ProcessID: "pipeline",
			Command:   []string{"bash", "-c", "echo 'test' | grep 't' | tr 't' 'T'"},
			Duration:  5 * time.Second,
		},

		// Tests de E/S
		{
			Name:      "File Creation and Reading",
			ProcessID: "file-io",
			Command:   []string{"bash", "-c", "echo 'test content' > test.txt && cat test.txt && rm test.txt"},
			Duration:  10 * time.Second,
		},
		{
			Name:      "Large Output",
			ProcessID: "large-output",
			Command:   []string{"bash", "-c", "for i in {1..1000}; do echo \"Line $i of test\"; done"},
			Duration:  15 * time.Second,
		},

		// Tests de recursos del sistema
		{
			Name:      "CPU Intensive",
			ProcessID: "cpu-test",
			Command:   []string{"bash", "-c", "for i in {1..1000000}; do echo \"$i\" > /dev/null; done"},
			Duration:  20 * time.Second,
		},
		{
			Name:      "Memory Usage",
			ProcessID: "memory-test",
			Command:   []string{"bash", "-c", "head -c 50M /dev/urandom > test.dat && rm test.dat"},
			Duration:  15 * time.Second,
		},

		// Tests de señales y control de procesos
		{
			Name:      "Trap Signals",
			ProcessID: "signal-trap",
			Command:   []string{"bash", "-c", "trap 'echo Signal caught' SIGTERM; sleep 30"},
			Duration:  10 * time.Second,
		},
		{
			Name:      "Process Group",
			ProcessID: "process-group",
			Command:   []string{"bash", "-c", "sleep 100 & sleep 200 & sleep 300 & wait"},
			Duration:  15 * time.Second,
		},

		// Tests de variables de entorno y directorios
		{
			Name:      "Complex Environment",
			ProcessID: "complex-env",
			Command:   []string{"bash", "-c", "echo $CUSTOM_VAR1-$CUSTOM_VAR2-$PATH"},
			Env: map[string]string{
				"CUSTOM_VAR1": "value1",
				"CUSTOM_VAR2": "value2",
				"PATH":        "/custom/path:$PATH",
			},
			Duration: 5 * time.Second,
		},
		{
			Name:       "Directory Operations",
			ProcessID:  "dir-ops",
			Command:    []string{"bash", "-c", "mkdir -p test/subdir && cd test && pwd && cd subdir && pwd && cd ../.. && rm -r test"},
			WorkingDir: "/tmp",
			Duration:   10 * time.Second,
		},

		// Tests de redirección y pipes
		{
			Name:      "Complex Redirection",
			ProcessID: "redirection",
			Command:   []string{"bash", "-c", "echo 'test' > file1 2>/dev/null && cat file1 1>&2 && rm file1"},
			Duration:  10 * time.Second,
		},
		{
			Name:      "Multiple Pipes",
			ProcessID: "pipes",
			Command:   []string{"bash", "-c", "cat /etc/passwd | grep root | cut -d: -f1 | tr 'r' 'R'"},
			Duration:  10 * time.Second,
		},

		// Tests de procesos anidados
		{
			Name:      "Nested Processes",
			ProcessID: "nested",
			Command: []string{"bash", "-c", `
				bash -c 'for i in {1..3}; do 
					bash -c \"echo Level 3 - Process \$PPID\"; 
					sleep 1; 
				done' &
				echo Level 1 - Process \$\$
				wait
			`},
			Duration: 15 * time.Second,
		},

		// Tests de límites y timeouts
		{
			Name:      "Rapid Process Creation",
			ProcessID: "rapid-spawn",
			Command:   []string{"bash", "-c", "for i in {1..50}; do sleep 0.1 & done; wait"},
			Duration:  10 * time.Second,
		},
		{
			Name:      "Large Environment",
			ProcessID: "large-env",
			Command:   []string{"env"},
			Env:       generateLargeEnv(100), // Función helper que genera muchas variables
			Duration:  5 * time.Second,
		},
	}

	// Ejecutar los tests
	testRunner.RunTests(t, testCases)
}

func TestParallelProcessExecution(t *testing.T) {
	env, err := setupTestEnvironment(t)
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	clientHost, err := env.ClientContainer.Host(env.Context)
	if err != nil {
		t.Fatalf("Failed to get client host: %v", err)
	}
	clientPort, err := env.ClientContainer.MappedPort(env.Context, "8080")
	if err != nil {
		t.Fatalf("Failed to get client port: %v", err)
	}

	baseURL := fmt.Sprintf("http://%s:%s", clientHost, clientPort.Port())
	testRunner := NewTestRunner(baseURL)

	// Tests que se ejecutarán en paralelo
	parallelTests := []TestCase{
		{
			Name:      "CPU Bound Parallel",
			ProcessID: "cpu-parallel",
			Command:   []string{"bash", "-c", "for i in {1..1000000}; do echo \"$i\" > /dev/null; done"},
			Duration:  20 * time.Second,
		},
		{
			Name:      "IO Bound Parallel",
			ProcessID: "io-parallel",
			Command:   []string{"bash", "-c", "dd if=/dev/zero of=test.dat bs=1M count=100 && rm test.dat"},
			Duration:  15 * time.Second,
		},
		{
			Name:      "Mixed Load Parallel",
			ProcessID: "mixed-parallel",
			Command: []string{"bash", "-c", `
				dd if=/dev/zero of=test.dat bs=1M count=50 &
				for i in {1..500000}; do echo "$i" > /dev/null; done &
				wait
				rm test.dat
			`},
			Duration: 25 * time.Second,
		},
	}

	// Ejecutar tests en paralelo
	t.Run("ParallelGroup", func(t *testing.T) {
		for _, tc := range parallelTests {
			tc := tc // Capturar variable para la goroutine
			t.Run(tc.Name, func(t *testing.T) {
				t.Parallel() // Marcar el test para ejecución paralela
				testRunner.RunTests(t, []TestCase{tc})
			})
		}
	})
}

func TestErrorCases(t *testing.T) {
	env, err := setupTestEnvironment(t)
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	clientHost, err := env.ClientContainer.Host(env.Context)
	if err != nil {
		t.Fatalf("Failed to get client host: %v", err)
	}
	clientPort, err := env.ClientContainer.MappedPort(env.Context, "8080")
	if err != nil {
		t.Fatalf("Failed to get client port: %v", err)
	}

	baseURL := fmt.Sprintf("http://%s:%s", clientHost, clientPort.Port())
	testRunner := NewTestRunner(baseURL)

	// Casos de error
	errorTests := []TestCase{
		{
			Name:      "Command Not Found",
			ProcessID: "not-found",
			Command:   []string{"nonexistentcommand"},
			Duration:  5 * time.Second,
		},
		{
			Name:      "Permission Denied",
			ProcessID: "permission-denied",
			Command:   []string{"cat", "/root/secret"},
			Duration:  5 * time.Second,
		},
		{
			Name:       "Invalid Working Directory",
			ProcessID:  "invalid-dir",
			Command:    []string{"ls"},
			WorkingDir: "/nonexistent/directory",
			Duration:   5 * time.Second,
		},
		{
			Name:      "Invalid File Descriptor",
			ProcessID: "bad-fd",
			Command:   []string{"bash", "-c", "echo test >&999"},
			Duration:  5 * time.Second,
		},
		{
			Name:      "Process Kill Test",
			ProcessID: "kill-test",
			Command:   []string{"bash", "-c", "kill -9 $$"},
			Duration:  5 * time.Second,
		},
		{
			Name:      "Memory Exhaustion",
			ProcessID: "memory-exhaust",
			Command:   []string{"bash", "-c", "head -c 10G /dev/urandom > /dev/null"},
			Duration:  10 * time.Second,
		},
		{
			Name:      "Fork Bomb",
			ProcessID: "fork-bomb",
			Command:   []string{"bash", "-c", ":(){ :|:& };:"},
			Duration:  5 * time.Second,
		},
		{
			Name:      "Syntax Error",
			ProcessID: "syntax-error",
			Command:   []string{"bash", "-c", "if then fi"},
			Duration:  5 * time.Second,
		},
	}

	testRunner.RunTests(t, errorTests)
}

// Helper function to generate large environment
func generateLargeEnv(count int) map[string]string {
	env := make(map[string]string)
	for i := 0; i < count; i++ {
		env[fmt.Sprintf("TEST_VAR_%d", i)] = fmt.Sprintf("value_%d", i)
	}
	return env
}
