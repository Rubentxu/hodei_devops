import React, { useState, useEffect } from 'react';
import { Theme, Button, Card, Flex, Grid, Box, Text, Heading, TextField } from '@radix-ui/react';
import * as ScrollArea from '@radix-ui/react-scroll-area';
import * as Label from '@radix-ui/react-label';
import * as Select from '@radix-ui/react-select';
import { EnvVarsEditor } from "./EnvVarsEditor";
import { useTaskStore } from "../store/useTaskStore";
import '@radix-ui/themes/styles.css';

interface TaskMessage {
  action: string;
  payload: any;
}

interface TaskRequest {
  name: string;
  description?: string;
  category?: string;
  image: string;
  command: string[];
  env: Record<string, string>;
  working_dir: string;
  instance_type: string;
  timeout?: number;
}

interface TaskResponse {
  id: string;
  name: string;
  description?: string;
  category?: string;
  state: string;
  worker_spec: {
    instance_type: string;
    image: string;
    command: string[];
    env: Record<string, string>;
    working_dir: string;
  };
  status: {
    exitCode?: number;
  };
  created_at: string;
  updated_at: string;
}

export default function TaskConsole() {
  const { ws, isConnected } = useTaskStore();
  const [messages, setMessages] = useState<string[]>([]);
  const [taskName, setTaskName] = useState('');
  const [taskDescription, setTaskDescription] = useState('');
  const [taskImage, setTaskImage] = useState('posts_mpv-remote-process');
  const [taskCommand, setTaskCommand] = useState('ls -la');
  const [instanceType, setInstanceType] = useState('docker');
  const [workingDir, setWorkingDir] = useState('/app');
  const [envVars, setEnvVars] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState('');

  useEffect(() => {
    if (!ws) return;

    const handleMessage = (event: MessageEvent) => {
      const response = JSON.parse(event.data);
      let message = '';

      if (response.action === 'task_output') {
        message = `[Output] ${response.payload.output}`;
      } else if (response.action === 'task_completed') {
        message = `[Completed] Task ${response.payload.task_id} completed with status: ${response.payload.status}`;
      } else if (response.action === 'task_error') {
        message = `[Error] ${response.payload.error} (Code: ${response.payload.exit_code})`;
        setFormError(response.payload.error);
      } else if (response.action === 'error') {
        message = `[Error] ${response.payload.message}`;
      }

      setMessages(prev => [...prev, message]);
    };

    ws.addEventListener('message', handleMessage);

    return () => {
      ws.removeEventListener('message', handleMessage);
    };
  }, [ws]);

  const validateForm = () => {
    if (!taskName.trim()) {
      setFormError('Task name is required');
      return false;
    }
    if (!taskImage.trim()) {
      setFormError('Image is required');
      return false;
    }
    if (!taskCommand.trim()) {
      setFormError('Command is required');
      return false;
    }
    if (!instanceType) {
      setFormError('Instance type is required');
      return false;
    }
    setFormError('');
    return true;
  };

  const createTask = () => {
    if (!ws || !isConnected) return;
    if (!validateForm()) return;

    const taskRequest: TaskRequest = {
      name: taskName.trim(),
      description: taskDescription.trim(),
      category: 'custom',
      image: taskImage.trim(),
      command: taskCommand.split(' ').filter(cmd => cmd.trim() !== ''),
      env: envVars,
      working_dir: workingDir.trim(),
      instance_type: instanceType,
      timeout: 300 // 5 minutos por defecto
    };

    const message: TaskMessage = {
      action: 'create_task',
      payload: taskRequest
    };

    try {
      ws.send(JSON.stringify(message));
      setMessages(prev => [...prev, `[Sent] Creating task: ${taskName}`]);
      setFormError('');
    } catch (error) {
      setFormError('Error al enviar la tarea: ' + (error as Error).message);
      console.error('Error sending task:', error);
    }
  };

  const clearConsole = () => {
    setMessages([]);
  };

  return (
    <Card>
      <Box p="4">
        <Heading size="4">Configuración de Tarea</Heading>
        <Text as="p" color="gray" size="2">
          Configure los parámetros de la tarea a ejecutar
        </Text>
      </Box>
      
      <Box p="4">
        <Grid gap="4">
          <Grid columns="2" gap="4">
            <Box>
              <Label.Root htmlFor="taskName">Nombre</Label.Root>
              <TextField.Root>
                <TextField.Input
                  id="taskName"
                  value={taskName}
                  onChange={(e) => setTaskName(e.target.value)}
                />
              </TextField.Root>
            </Box>
            <Box>
              <Label.Root htmlFor="taskDescription">Descripción</Label.Root>
              <TextField.Root>
                <TextField.Input
                  id="taskDescription"
                  value={taskDescription}
                  onChange={(e) => setTaskDescription(e.target.value)}
                />
              </TextField.Root>
            </Box>
          </Grid>

          <Grid columns="2" gap="4">
            <Box>
              <Label.Root htmlFor="taskImage">Imagen</Label.Root>
              <TextField.Root>
                <TextField.Input
                  id="taskImage"
                  value={taskImage}
                  onChange={(e) => setTaskImage(e.target.value)}
                />
              </TextField.Root>
            </Box>
            <Box>
              <Label.Root htmlFor="instanceType">Tipo de Instancia</Label.Root>
              <Select.Root value={instanceType} onValueChange={setInstanceType}>
                <Select.Trigger>
                  <Select.Value placeholder="Selecciona un tipo..." />
                </Select.Trigger>
                <Select.Portal>
                  <Select.Content>
                    <Select.Viewport>
                      <Select.Item value="docker">
                        <Select.ItemText>Docker</Select.ItemText>
                      </Select.Item>
                      <Select.Item value="kubernetes">
                        <Select.ItemText>Kubernetes</Select.ItemText>
                      </Select.Item>
                    </Select.Viewport>
                  </Select.Content>
                </Select.Portal>
              </Select.Root>
            </Box>
          </Grid>

          <Grid columns="2" gap="4">
            <Box>
              <Label.Root htmlFor="command">Comando</Label.Root>
              <TextField.Root>
                <TextField.Input
                  id="command"
                  value={taskCommand}
                  onChange={(e) => setTaskCommand(e.target.value)}
                />
              </TextField.Root>
            </Box>
            <Box>
              <Label.Root htmlFor="workingDir">Directorio de Trabajo</Label.Root>
              <TextField.Root>
                <TextField.Input
                  id="workingDir"
                  value={workingDir}
                  onChange={(e) => setWorkingDir(e.target.value)}
                />
              </TextField.Root>
            </Box>
          </Grid>

          <Box>
            <Label.Root>Variables de Entorno</Label.Root>
            <EnvVarsEditor
              envVars={envVars}
              onChange={setEnvVars}
            />
          </Box>

          {formError && (
            <Text color="red" size="2">
              {formError}
            </Text>
          )}

          <ScrollArea.Root className="ScrollAreaRoot">
            <ScrollArea.Viewport className="ScrollAreaViewport">
              {messages.map((msg, index) => (
                <Text key={index} as="pre" size="2" style={{ fontFamily: 'monospace' }}>
                  {msg}
                </Text>
              ))}
            </ScrollArea.Viewport>
            <ScrollArea.Scrollbar
              className="ScrollAreaScrollbar"
              orientation="vertical"
            >
              <ScrollArea.Thumb className="ScrollAreaThumb" />
            </ScrollArea.Scrollbar>
          </ScrollArea.Root>
        </Grid>
      </Box>
      
      <Flex p="4" gap="2" justify="end">
        <Button onClick={clearConsole}>
          Limpiar Consola
        </Button>
        <Button onClick={createTask} disabled={!isConnected}>
          Crear Tarea
        </Button>
      </Flex>
    </Card>
  );
}
