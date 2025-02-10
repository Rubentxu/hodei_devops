export type InstanceType = 'docker' | 'kubernetes' | 'local';

export type State = 'pending' | 'running' | 'completed' | 'failed' | 'stopped';

export type Category = 'system' | 'files' | 'network' | 'database' | 'docker' | 'kubernetes' | 'git' | 'build' | 'test' | 'deploy';

export type TaskStatus = {
  exitCode?: number;
  error?: string;
};

export type WorkerSpec = {
  instance_type: InstanceType;
  image: string;
  container_name?: string;
  command?: string[];
  env?: { [key: string]: string };
  working_dir?: string;
};

export interface Task {
  id: string;  // UUID
  name: string;
  display_name?: string;
  description?: string;
  category: Category;
  state: State;
  worker_spec: WorkerSpec;
  status: TaskStatus;
  created_at: string;  // ISO date string
  updated_at: string;  // ISO date string
};

export type TaskCategory = 'Frontend' | 'Backend' | 'Database' | 'Security';

export interface CreateTaskRequest {
  name: string;
  image: string;
  command?: string[];
  env?: { [key: string]: string };
  working_dir?: string;
  instance_type: InstanceType;
  timeout?: number;
  category?: TaskCategory;
}

export interface WSMessage {
  action: string;
  payload: any;
}

export interface TaskStats {
  total: number;
  running: number;
  completed: number;
  failed: number;
}

export interface TaskOutput {
  task_id: string;
  output: string;
  timestamp: string;
}

export interface TaskStatusUpdate {
  task_id: string;
  state: State;
  exitCode?: number;
  error?: string;
}
