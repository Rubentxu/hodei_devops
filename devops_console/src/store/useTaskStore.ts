import { create } from 'zustand';
import { Task, CreateTaskRequest, WSMessage } from '../types/task';

const WS_URL = 'ws://localhost:8080/ws';

// Mantener una única instancia del WebSocket
let globalWs: WebSocket | null = null;

interface TaskStore {
  tasks: Task[];
  selectedTask: Task | null;
  taskOutput: string;
  ws: WebSocket | null;
  isConnected: boolean;
  
  // Acciones
  setTasks: (tasks: Task[]) => void;
  selectTask: (task: Task | null) => void;
  clearTaskOutput: () => void;
  connect: () => void;
  disconnect: () => void;
  createTask: (request: CreateTaskRequest) => Promise<void>;
}

const defaultTasks: Task[] = [
  {
    id: '1',
    name: 'Listar Archivos',
    description: 'Lista todos los archivos del directorio actual con detalles como permisos y tamaños',
    category: 'files',
    state: 'completed',
    worker_spec: {
      instance_type: 'docker',
      image: 'posts_mpv-remote-process',
      command: ['ls', '-la'],
      env: {},
      working_dir: '/app',
    },
    status: {
      exitCode: 0,
    },
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: '2',
    name: 'Mostrar Variables de Entorno',
    description: 'Muestra todas las variables de entorno disponibles en el contenedor',
    category: 'system',
    state: 'running',
    worker_spec: {
      instance_type: 'docker',
      image: 'posts_mpv-remote-process',
      command: ['env'],
      env: { 
        GREETING: 'Hello',
        APP_ENV: 'development',
        DEBUG: 'true'
      },
      working_dir: '/app',
    },
    status: {},
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: '3',
    name: 'Buscar Archivos por Patrón',
    description: 'Busca archivos recursivamente que coincidan con un patrón específico',
    category: 'files',
    state: 'failed',
    worker_spec: {
      instance_type: 'docker',
      image: 'posts_mpv-remote-process',
      command: ['find', '$SEARCH_PATH', '-name', '$PATTERN'],
      env: {
        SEARCH_PATH: '/etc',
        PATTERN: '*'
      },
      working_dir: '/app',
    },
    status: {
      exitCode: 1,
      error: 'find: /invalid/path: No such file or directory',
    },
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: '4',
    name: 'Información del Sistema',
    description: 'Obtiene información detallada sobre el sistema operativo y hardware',
    category: 'system',
    state: 'pending',
    worker_spec: {
      instance_type: 'docker',
      image: 'posts_mpv-remote-process',
      command: ['sh', '-c', 'uname -a && cat /proc/cpuinfo'],
      env: {
        LANG: 'es_ES.UTF-8',
        TERM: 'xterm-256color'
      },
      working_dir: '/app',
    },
    status: {},
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  }
]; 

export const useTaskStore = create<TaskStore>((set, get) => ({
  tasks: defaultTasks,
  selectedTask: null,
  taskOutput: '',
  ws: null,
  isConnected: false,

  setTasks: (tasks) => set({ tasks }),
  selectTask: (task) => set({ selectedTask: task }),
  clearTaskOutput: () => set({ taskOutput: '' }),

  connect: () => {
    const connectWebSocket = () => {
      if (globalWs?.readyState === WebSocket.OPEN) {
        console.log('WebSocket ya está conectado');
        set({ ws: globalWs, isConnected: true });
        return;
      }

      if (globalWs?.readyState === WebSocket.CONNECTING) {
        console.log('WebSocket conectándose...');
        return;
      }

      console.log('Conectando nuevo WebSocket...');
      const newWs = new WebSocket(WS_URL);
      globalWs = newWs;
      
      newWs.onopen = () => {
        console.log('WebSocket conectado');
        set({ ws: newWs, isConnected: true });
      };

      newWs.onclose = (event) => {
        console.log('WebSocket desconectado:', event.reason);
        set((state) => ({
          ws: null, 
          isConnected: false,
          taskOutput: state.taskOutput + '\n[SYSTEM] 🔌 Conexión WebSocket cerrada: ' + 
            (event.wasClean ? 'Cierre normal' : 'Cierre inesperado') + 
            (event.reason ? ` - ${event.reason}` : '') + '\n'
        }));
        
        globalWs = null;

        // Solo intentar reconectar si el cierre no fue limpio
        if (!event.wasClean) {
          console.log('Reconectando en 3 segundos...');
          setTimeout(connectWebSocket, 3000);
        }
      };

      newWs.onerror = (error) => {
        console.error('Error en WebSocket:', error);
        set((state) => ({
          taskOutput: state.taskOutput + '\n[ERROR] ⚠️ Error en la conexión WebSocket\n'
        }));
      };

      newWs.onmessage = (event) => {
        try {
          const message: WSMessage = JSON.parse(event.data);
          console.log('Mensaje WebSocket recibido:', message);
          
          switch (message.action) {
            case 'task_list':
              set({ tasks: message.payload });
              break;
            
            case 'task_output':
              const { task_id, output, status, is_error } = message.payload;
              set((state) => {
                // Formatear el mensaje según su contenido
                let formattedOutput = output;
                let icon = '';
                let prefix = '';

                if (output.includes('[WORKER CLIENT]')) {
                  icon = '🤖 ';
                  prefix = '[INFO]';
                } else if (output.includes('[RPC]')) {
                  icon = '📡 ';
                  prefix = '[DEBUG]';
                } else if (output.startsWith('Docker Config:')) {
                  icon = '🐳 ';
                  prefix = '[DEBUG]';
                } else if (output.includes('Contenedor')) {
                  icon = '📦 ';
                  prefix = '[INFO]';
                } else if (output.includes('Environment:')) {
                  icon = '🔧 ';
                  prefix = '[DEBUG]';
                } else if (output.includes('Ejecutando comando:')) {
                  icon = '⚡ ';
                  prefix = '[INFO]';
                } else if (output.includes('Process completed successfully')) {
                  icon = '✅ ';
                  prefix = '[SUCCESS]';
                } else if (output.includes('Process finished')) {
                  icon = '✅ ';
                  prefix = '[SUCCESS]';
                } else {
                  // Para la salida del comando, no añadimos prefijo
                  formattedOutput = output;
                }

                if (is_error) {
                  icon = '❌ ';
                  prefix = '[ERROR]';
                }

                const newOutput = prefix ? `\n${prefix} ${icon}${output}\n` : `${output}\n`;

                // Actualizar el estado de la tarea
                const updatedTasks = state.tasks.map(task => {
                  if (task.id === task_id) {
                    const newTask = { 
                      ...task,
                      status: { 
                        ...task.status,
                        error: is_error ? output : task.status.error 
                      }
                    };

                    // Actualizar el estado solo si es un estado válido
                    if (status) {
                      const normalizedStatus = status.toLowerCase();
                      if (['pending', 'running', 'completed', 'failed', 'finished'].includes(normalizedStatus)) {
                        newTask.state = normalizedStatus === 'finished' ? 'completed' : normalizedStatus;
                        
                        // Si el estado es FINISHED o completed, añadir timestamp
                        if (normalizedStatus === 'finished' || normalizedStatus === 'completed') {
                          newTask.updated_at = new Date().toISOString();
                        }
                      }
                    }

                    return newTask;
                  }
                  return task;
                });

                return {
                  taskOutput: state.taskOutput + newOutput,
                  tasks: updatedTasks
                };
              });
              break;
            
            case 'task_status':
              const { id, state: newState } = message.payload;
              set((state) => {
                let statusMessage = '';
                switch (newState.toLowerCase()) {
                  case 'completed':
                    statusMessage = '\n[SUCCESS] ✅ Tarea completada exitosamente\n';
                    break;
                  case 'failed':
                    statusMessage = '\n[ERROR] ❌ La tarea ha fallado\n';
                    break;
                  case 'running':
                    statusMessage = '\n[INFO] ⚡ Tarea en ejecución...\n';
                    break;
                  case 'pending':
                    statusMessage = '\n[INFO] ⏳ Tarea pendiente...\n';
                    break;
                  case 'finished':
                    statusMessage = '\n[SUCCESS] ✅ Tarea finalizada exitosamente\n';
                    break;
                }

                return {
                  tasks: state.tasks.map(task => 
                    task.id === id
                      ? { 
                          ...task, 
                          state: newState.toLowerCase() === 'finished' ? 'completed' : newState.toLowerCase(),
                          updated_at: new Date().toISOString()
                        }
                      : task
                  ),
                  taskOutput: state.taskOutput + statusMessage
                };
              });
              break;
            
            case 'task_error':
              const { error } = message.payload;
              set((state) => ({
                taskOutput: state.taskOutput + `\n[ERROR] ❌ ${error}\n`
              }));
              break;

            default:
              console.warn('Mensaje no manejado:', message);
          }
        } catch (error) {
          console.error('Error al procesar mensaje WebSocket:', error);
          set((state) => ({
            taskOutput: state.taskOutput + '\n[ERROR] ⚠️ Error al procesar mensaje del servidor\n'
          }));
        }
      };

      set({ ws: newWs });
    };
    connectWebSocket();
  },

  disconnect: () => {
    const { ws } = get();
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.close(1000, 'Desconexión normal');
    }
    globalWs = null;
    set({ ws: null, isConnected: false });
  },

  createTask: async (request: CreateTaskRequest) => {
    const { ws, isConnected } = get();
    if (!ws || !isConnected) {
      throw new Error('WebSocket no conectado');
    }

    // Generar un nombre de contenedor válido
    const containerName = `task-${request.name
      .toLowerCase()
      .replace(/[^a-z0-9]/g, '-')
      .replace(/-+/g, '-')
      .replace(/^-|-$/g, '')}-${Date.now()}`;

    const taskReq = {
      name: containerName,
      display_name: request.name,
      image: request.image,
      command: request.command || [],
      env: request.env || {},
      working_dir: request.working_dir || '/',
      instance_type: request.instance_type,
      container_name: containerName
    };

    const message: WSMessage = {
      action: 'create_task',
      payload: taskReq
    };

    ws.send(JSON.stringify(message));
    set({ taskOutput: '' });
  }
}));

// Conectar WebSocket al iniciar la aplicación
if (typeof window !== 'undefined') {
  useTaskStore.getState().connect();
}
