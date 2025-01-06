package integration

import "time"

// TestCase define la estructura de un caso de prueba
type TestCase struct {
	Name       string            `json:"name"`
	ProcessID  string            `json:"process_id"`
	Command    []string          `json:"command"`
	Env        map[string]string `json:"env,omitempty"`
	WorkingDir string            `json:"working_directory,omitempty"`
	Duration   time.Duration
}

// ProcessRequest define la estructura para las peticiones al servidor
type ProcessRequest struct {
	RemoteProcessServerAddress string            `json:"remote_process_server_address"`
	Command                    []string          `json:"command"`
	ProcessID                  string            `json:"process_id"`
	CheckInterval              int64             `json:"check_interval"`
	Env                        map[string]string `json:"env,omitempty"`
	WorkingDirectory           string            `json:"working_directory,omitempty"`
}
