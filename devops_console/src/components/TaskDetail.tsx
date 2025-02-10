import React from 'react';
import { Card, Text, Box, ScrollArea, Code } from '@radix-ui/themes';
import { useTaskStore } from '../store/useTaskStore';

export const TaskDetail: React.FC = () => {
  const { selectedTask } = useTaskStore();

  if (!selectedTask) {
    return (
      <Card size="2">
        <Text>Select a task to see details</Text>
      </Card>
    );
  }

  return (
    <Card size="2">
      <Box mb="4">
        <Text as="div" size="2" weight="bold" mb="1">
          Task ID:
        </Text>
        <Text as="div" color="gray">
          {selectedTask.task_id}
        </Text>
      </Box>

      <Box mb="4">
        <Text as="div" size="2" weight="bold" mb="1">
          Status:
        </Text>
        <Text 
          as="div"
          color={
            selectedTask.is_error ? 'red' : 
            selectedTask.status === 'completed' ? 'green' : 
            'blue'
          }
        >
          {selectedTask.status}
        </Text>
      </Box>

      {selectedTask.completed_at && (
        <Box mb="4">
          <Text as="div" size="2" weight="bold" mb="1">
            Completed At:
          </Text>
          <Text as="div" color="gray">
            {new Date(selectedTask.completed_at).toLocaleString()}
          </Text>
        </Box>
      )}

      {selectedTask.output && (
        <Box mb="4">
          <Text as="div" size="2" weight="bold" mb="1">
            Output:
          </Text>
          <ScrollArea style={{ height: '200px' }}>
            <Code>
              {selectedTask.output}
            </Code>
          </ScrollArea>
        </Box>
      )}

      {selectedTask.error && (
        <Box mb="4">
          <Text as="div" size="2" weight="bold" mb="1" color="red">
            Error:
          </Text>
          <Text as="div" color="red">
            {selectedTask.error}
          </Text>
        </Box>
      )}

      {selectedTask.exit_code && (
        <Box>
          <Text as="div" size="2" weight="bold" mb="1">
            Exit Code:
          </Text>
          <Text as="div" color={selectedTask.exit_code === '0' ? 'green' : 'red'}>
            {selectedTask.exit_code}
          </Text>
        </Box>
      )}
    </Card>
  );
};
