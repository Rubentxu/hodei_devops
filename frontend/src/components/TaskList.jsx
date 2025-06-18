import React from 'react';
import TaskItem from './TaskItem';
import { useAppContext } from '../context/AppContext';

function TaskList({ sendMessage }) { // sendMessage passed as prop
  const { state } = useAppContext();
  const { tasks, isLoading, error } = state;

  if (isLoading) {
    return <p>Loading tasks...</p>;
  }

  if (error) {
    return <p style={{ color: 'red' }}>Error loading tasks: {error}</p>;
  }

  if (!tasks || tasks.length === 0) {
    return <p>No tasks found.</p>;
  }

  return (
    <div>
      <h2>Task List</h2>
      {tasks.map(task => (
        <TaskItem key={task.id} task={task} sendMessage={sendMessage} />
      ))}
    </div>
  );
}

export default TaskList;
