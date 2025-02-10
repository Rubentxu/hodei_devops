import React, { useState } from 'react';
import { Card, TextField, Button, Box, Text, Select } from '@radix-ui/themes';
import { useTaskStore } from '../store/useTaskStore';
import { CreateTaskRequest } from '../types/task';

export const CreateTaskForm: React.FC = () => {
  const { createTask } = useTaskStore();
  const [formData, setFormData] = useState<CreateTaskRequest>({
    name: '',
    image: '',
    command: [],
    env: {},
    working_dir: '',
    instance_type: 'docker',
  });

  const [envPairs, setEnvPairs] = useState<{ key: string; value: string }[]>([
    { key: '', value: '' },
  ]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    
    // Process environment variables
    const env: Record<string, string> = {};
    envPairs.forEach(pair => {
      if (pair.key && pair.value) {
        env[pair.key] = pair.value;
      }
    });

    // Process command array
    const command = formData.command?.join(' ').split(' ').filter(Boolean) || [];

    createTask({
      ...formData,
      command,
      env,
    });
  };

  const handleEnvChange = (index: number, field: 'key' | 'value', value: string) => {
    const newEnvPairs = [...envPairs];
    newEnvPairs[index][field] = value;

    // Add new pair if last pair is being filled
    if (index === envPairs.length - 1 && (value !== '')) {
      newEnvPairs.push({ key: '', value: '' });
    }

    setEnvPairs(newEnvPairs);
  };

  return (
    <Card size="2">
      <form onSubmit={handleSubmit}>
        <Box mb="4">
          <Text as="label" size="2" mb="1" weight="bold">
            Task Name
          </Text>
          <TextField.Root>
            <TextField.Input
              placeholder="Enter task name"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              required
            />
          </TextField.Root>
        </Box>

        <Box mb="4">
          <Text as="label" size="2" mb="1" weight="bold">
            Image
          </Text>
          <TextField.Root>
            <TextField.Input
              placeholder="Enter image name"
              value={formData.image}
              onChange={(e) => setFormData({ ...formData, image: e.target.value })}
              required
            />
          </TextField.Root>
        </Box>

        <Box mb="4">
          <Text as="label" size="2" mb="1" weight="bold">
            Command
          </Text>
          <TextField.Root>
            <TextField.Input
              placeholder="Enter command (space-separated)"
              value={formData.command?.join(' ')}
              onChange={(e) => setFormData({ 
                ...formData, 
                command: e.target.value.split(' ').filter(Boolean)
              })}
            />
          </TextField.Root>
        </Box>

        <Box mb="4">
          <Text as="label" size="2" mb="1" weight="bold">
            Working Directory
          </Text>
          <TextField.Root>
            <TextField.Input
              placeholder="Enter working directory"
              value={formData.working_dir}
              onChange={(e) => setFormData({ ...formData, working_dir: e.target.value })}
            />
          </TextField.Root>
        </Box>

        <Box mb="4">
          <Text as="label" size="2" mb="1" weight="bold">
            Instance Type
          </Text>
          <Select.Root 
            value={formData.instance_type} 
            onValueChange={(value) => setFormData({ ...formData, instance_type: value })}
          >
            <Select.Trigger />
            <Select.Content>
              <Select.Item value="docker">Docker</Select.Item>
              <Select.Item value="kubernetes">Kubernetes</Select.Item>
            </Select.Content>
          </Select.Root>
        </Box>

        <Box mb="4">
          <Text as="label" size="2" mb="1" weight="bold">
            Environment Variables
          </Text>
          {envPairs.map((pair, index) => (
            <Box key={index} display="flex" gap="2" mb="2">
              <TextField.Root>
                <TextField.Input
                  placeholder="Key"
                  value={pair.key}
                  onChange={(e) => handleEnvChange(index, 'key', e.target.value)}
                />
              </TextField.Root>
              <TextField.Root>
                <TextField.Input
                  placeholder="Value"
                  value={pair.value}
                  onChange={(e) => handleEnvChange(index, 'value', e.target.value)}
                />
              </TextField.Root>
            </Box>
          ))}
        </Box>

        <Box>
          <Button type="submit">
            Create Task
          </Button>
        </Box>
      </form>
    </Card>
  );
};
