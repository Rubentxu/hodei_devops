import {
  ArrowsClockwise,
  Play,
  Trash,
  Plus,
  Gear,
  Cards,
  CheckCircle,
  Desktop,
  MagnifyingGlass,
  Timer,
  WarningCircle,
  XCircle,
  Database,
  Cloud,
  GitBranch,
  Package,
  TestTube,
  Rocket,
  Network,
  Files
} from "@phosphor-icons/react";
import * as ScrollArea from '@radix-ui/react-scroll-area';
import { Button, Card, Flex, Grid, Box, Text, Heading, IconButton, Badge } from '@radix-ui/themes';
import '@radix-ui/themes/styles.css';
import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTaskStore } from '../store/useTaskStore';
import { InstanceType, State, Task, TaskCategory } from '../types/task';
import './TaskList.css';

type FilterType = TaskCategory | 'All';

const TaskList = () => {
  const { tasks = [], selectTask, stopTask, createTask, isConnected, connect, disconnect } = useTaskStore();
  const [searchQuery, setSearchQuery] = useState('');
  const [activeFilter, setActiveFilter] = useState<FilterType>('All');
  const [viewMode, setViewMode] = useState<'table' | 'cards'>('cards');
  const [flippedCards, setFlippedCards] = useState<Set<string>>(new Set());
  const navigate = useNavigate();

  const handleTaskClick = (task: Task) => {
    if (isConnected) {
      selectTask(task);
      navigate('/terminal');
    }
  };

  const handleStopTask = (taskId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (isConnected) {
      stopTask(taskId);
    }
  };

  const handleRunTask = async (task: Task, e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();

    try {
      // Crear la petición de tarea en el formato esperado por el backend
      const request: CreateTaskRequest = {
        name: task.name,
        image: task.worker_spec.image,
        command: task.worker_spec.command || [],
        env: task.worker_spec.env || {},
        working_dir: task.worker_spec.working_dir || '/',
        instance_type: task.worker_spec.instance_type
      };

      await createTask(request);
      selectTask(task);
      navigate('/terminal');
    } catch (error) {
      console.error('Error al ejecutar la tarea:', error);
    }
  };

  const handleRestartTask = (task: Task, e: React.MouseEvent) => {
    e.stopPropagation();
    if (isConnected && (task.state === 'completed' || task.state === 'failed')) {
      createTask({
        name: task.name,
        worker_spec: task.worker_spec,
      });
    }
  };

  const handleCardFlip = (taskId: string, e: React.MouseEvent) => {
    // Evitar volteo si se hace clic en el botón
    if ((e.target as HTMLElement).closest('button')) return;
    
    setFlippedCards(prev => {
      const newFlipped = new Set(prev);
      if (newFlipped.has(taskId)) {
        newFlipped.delete(taskId);
      } else {
        newFlipped.add(taskId);
      }
      return newFlipped;
    });
  };

  const getStatusIcon = (state: State) => {
    switch (state) {
      case 'completed':
        return <CheckCircle size={24} weight="fill" color="var(--green-9)" />;
      case 'running':
        return <Timer size={24} weight="fill" color="var(--blue-9)" />;
      case 'failed':
        return <XCircle size={24} weight="fill" color="var(--red-9)" />;
      default:
        return <Timer size={24} weight="duotone" color="var(--gray-9)" />;
    }
  };

  const getInstanceTypeIcon = (type: InstanceType) => {
    switch (type) {
      case 'small':
        return <Desktop size={24} weight="fill" color="var(--green-9)" />;
      case 'medium':
        return <Desktop size={24} weight="fill" color="var(--amber-9)" />;
      case 'large':
        return <Desktop size={24} weight="fill" color="var(--red-9)" />;
      default:
        return <Desktop size={24} weight="duotone" color="var(--gray-9)" />;
    }
  };

  const getCategoryIcon = (category: TaskCategory) => {
    const iconProps = { size: 180, weight: "fill" };
  
    switch (category) {
      case 'system':
        return <Gear {...iconProps} color="var(--amber-9)" />;
      case 'files':
        return <Files {...iconProps} color="var(--blue-9)" />;
      case 'network':
        return <Network {...iconProps} color="var(--purple-9)" />;
      case 'database':
        return <Database {...iconProps} color="var(--green-9)" />;
      case 'docker':
        return <Package {...iconProps} color="var(--blue-9)" />;
      case 'kubernetes':
        return <Cloud {...iconProps} color="var(--sky-9)" />;
      case 'git':
        return <GitBranch {...iconProps} color="var(--orange-9)" />;
      case 'build':
        return <Package {...iconProps} color="var(--yellow-9)" />;
      case 'test':
        return <TestTube {...iconProps} color="var(--violet-9)" />;
      case 'deploy':
        return <Rocket {...iconProps} color="var(--red-9)" />;
      default:
        return <Gear {...iconProps} color="var(--gray-9)" />;
    }
  };

  const filteredTasks = tasks.filter(task => {
    const matchesSearch = task.name.toLowerCase().includes(searchQuery.toLowerCase());
    const matchesFilter = activeFilter === 'All' || task.worker_spec.instance_type === activeFilter;
    return matchesSearch && matchesFilter;
  });

  useEffect(() => {
    // Conectar al WebSocket cuando el componente se monta
    connect();

    // Desconectar cuando el componente se desmonta
    return () => {
      disconnect();
    };
  }, [connect, disconnect]);

  return (
    <Flex direction="column" gap="4">
      {!isConnected && (
        <Card variant="surface">
          <Flex gap="2" align="center" wrap="wrap">
            <WarningCircle size={24} weight="fill" color="var(--amber-9)" />
            <Text color="amber">El servicio remoto no está disponible. Mostrando datos de ejemplo.</Text>
            <Button variant="soft" onClick={() => window.location.reload()}>
              Reintentar conexión
            </Button>
          </Flex>
        </Card>
      )}

      {/* Search, Filters and View Toggle */}
      <Flex direction={{ initial: 'column', sm: 'row' }} gap="4" align={{ sm: 'center' }}>
        <Text.Root 
          size={{ initial: '2', sm: '1' }}
          style={{ flex: 1 }}
          placeholder="Search jobs..." 
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
        >
          <Text.Slot>
            <MagnifyingGlass size={16} />
          </Text.Slot>
        </Text.Root>

        <Flex gap="2" wrap="wrap">
          {(['All', 'small', 'medium', 'large'] as const).map((filter) => (
            <Button 
              key={filter}
              size={{ initial: '1', sm: '2' }}
              variant={activeFilter === filter ? 'solid' : 'soft'}
              onClick={() => setActiveFilter(filter)}
            >
              {filter}
            </Button>
          ))}
          <Button
            size={{ initial: '1', sm: '2' }}
            variant="soft"
            onClick={() => setViewMode(prev => prev === 'table' ? 'cards' : 'table')}
          >
            {viewMode === 'table' ? <Cards size={16} /> : <Table size={16} />}
          </Button>
        </Flex>
      </Flex>

      {/* Tasks List */}
      <ScrollArea.Root className="ScrollAreaRoot">
        <ScrollArea.Viewport className="ScrollAreaViewport">
          {viewMode === 'table' ? (
            <Grid gap="4">
              {filteredTasks.map((task) => (
                <Box key={task.id}>
                  <Card
                    className={`task-card ${flippedCards.has(task.id) ? 'flipped' : ''}`}
                  >
                    <Box className="card-face card-front">
                      <Box className="task-content">
                        <Flex className="task-header" justify="between">
                          <Heading size="3">{task.name}</Heading>
                          <IconButton
                            onClick={() => handleCardFlip(task.id)}
                            variant="ghost"
                          >
                            <ArrowsClockwise />
                          </IconButton>
                        </Flex>
                        <Box className="task-body">
                          <Text as="p" size="2" color="gray">
                            {task.description}
                          </Text>
                          <Flex gap="2" mt="2">
                            <Badge variant="soft" color={getStatusColor(task.state)}>
                              {task.state}
                            </Badge>
                            <Badge variant="soft">
                              {task.worker_spec.instance_type}
                            </Badge>
                          </Flex>
                        </Box>
                        <Flex className="task-footer" gap="2">
                          <Button
                            variant="soft"
                            onClick={() => navigate(`/console/${task.id}`)}
                          >
                            Ver Consola
                          </Button>
                          <Button
                            variant="soft"
                            color="red"
                            onClick={(e) => handleStopTask(task.id, e)}
                          >
                            Detener
                          </Button>
                        </Flex>
                      </Box>
                    </Box>
                    <Box className="card-face card-back">
                      <Box className="task-content">
                        <Flex className="task-header" justify="between">
                          <Heading size="3">Detalles de la Tarea</Heading>
                          <IconButton
                            onClick={() => handleCardFlip(task.id)}
                            variant="ghost"
                          >
                            <ArrowsClockwise />
                          </IconButton>
                        </Flex>
                        <Box className="task-body">
                          <Grid gap="2">
                            <Text as="p" size="2" weight="bold">
                              ID: <Text>{task.id}</Text>
                            </Text>
                            <Text as="p" size="2" weight="bold">
                              Imagen: <Text>{task.worker_spec.image}</Text>
                            </Text>
                            <Text as="p" size="2" weight="bold">
                              Comando:{" "}
                              <Text>{task.worker_spec.command?.join(" ")}</Text>
                            </Text>
                            <Text as="p" size="2" weight="bold">
                              Variables de Entorno:
                            </Text>
                            {task.worker_spec.env && (
                              <Box>
                                {Object.entries(task.worker_spec.env).map(
                                  ([key, value]) => (
                                    <Text key={key} as="p" size="2">
                                      {key}: {value}
                                    </Text>
                                  )
                                )}
                              </Box>
                            )}
                          </Grid>
                        </Box>
                      </Box>
                    </Box>
                  </Card>
                </Box>
              ))}
            </Grid>
          ) : (
            <Flex gap="3" wrap="wrap">
              {filteredTasks.map((task) => (
                <div
                  key={task.id}
                  className={`task-card ${flippedCards.has(task.id) ? 'flipped' : ''}`}
                  onClick={(e) => handleCardFlip(task.id, e)}
                >
                  <div>
                    <div className="card-face card-front">
                      <div className="category-icon-container">
                        {getCategoryIcon(task.category)}
                      </div>
                      
                      <div className="task-content">
                        <div className="task-header">
                          {getStatusIcon(task.state)}
                          <Text size="1" color="gray">
                            ID: {task.id}
                          </Text>
                        </div>
                        
                        <div className="task-body">
                          <Text weight="medium" size="5">{task.display_name || task.name}</Text>
                          
                          <Flex direction="column" gap="2">
                            <Text size="2" color="gray">
                              {task.description}
                            </Text>
                            
                            <Flex align="center" gap="2">
                              <Text size="2" weight="bold">Tipo:</Text>
                              <Text size="2">{task.worker_spec.instance_type}</Text>
                            </Flex>
                          </Flex>
                        </div>

                        <div className="task-footer">
                          <Flex justify="end">
                            <Button 
                              size="2"
                              variant="solid" 
                              color="green"
                              onClick={(e) => handleRunTask(task, e)}
                            >
                              <Play size={16} weight="bold" />
                              Iniciar
                            </Button>
                          </Flex>
                        </div>
                      </div>
                    </div>

                    <div className="card-face card-back">
                      <div className="task-content">
                        <div className="task-header">
                          <Text weight="medium" size="5">Especificaciones del Worker</Text>
                        </div>
                        
                        <div className="task-body">
                          <Flex direction="column" gap="3">
                            <Flex direction="column" gap="2">
                              <Text weight="bold" size="2">Imagen:</Text>
                              <Text size="2">{task.worker_spec.image}</Text>
                            </Flex>

                            <Flex direction="column" gap="2">
                              <Text weight="bold" size="2">Comando:</Text>
                              <Text size="2" style={{ wordBreak: 'break-all' }}>
                                {task.worker_spec.command?.join(' ')}
                              </Text>
                            </Flex>

                            {task.worker_spec.working_dir && (
                              <Flex direction="column" gap="2">
                                <Text weight="bold" size="2">Directorio:</Text>
                                <Text size="2">{task.worker_spec.working_dir}</Text>
                              </Flex>
                            )}

                            {task.worker_spec.env && Object.keys(task.worker_spec.env).length > 0 && (
                              <Flex direction="column" gap="2">
                                <Text weight="bold" size="2">Variables de Entorno:</Text>
                                {Object.entries(task.worker_spec.env).map(([key, value]) => (
                                  <Text size="2" key={key}>
                                    <strong>{key}:</strong> {value}
                                  </Text>
                                ))}
                              </Flex>
                            )}

                            {task.status?.exitCode !== undefined && (
                              <Flex direction="column" gap="2">
                                <Text weight="bold" size="2">Código de salida:</Text>
                                <Text size="2" color={task.status.exitCode === 0 ? "green" : "red"}>
                                  {task.status.exitCode}
                                </Text>
                              </Flex>
                            )}

                            {task.status?.error && (
                              <Flex direction="column" gap="2">
                                <Text weight="bold" size="2">Error:</Text>
                                <Text size="2" color="red">{task.status.error}</Text>
                              </Flex>
                            )}
                          </Flex>
                        </div>

                        <div className="task-footer">
                          <Flex justify="end">
                            <Button 
                              size="2"
                              variant="solid" 
                              color="green"
                              onClick={(e) => handleRunTask(task, e)}
                            >
                              <Play size={16} weight="bold" />
                              Iniciar
                            </Button>
                          </Flex>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </Flex>
          )}
        </ScrollArea.Viewport>
        <ScrollArea.Scrollbar 
          className="ScrollAreaScrollbar" 
          orientation="horizontal"
        >
          <ScrollArea.Thumb className="ScrollAreaThumb" />
        </ScrollArea.Scrollbar>
      </ScrollArea.Root>
    </Flex>
  );
};

export default TaskList;
