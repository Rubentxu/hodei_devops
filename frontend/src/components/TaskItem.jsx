import React from 'react';
import { useAppContext } from '../context/AppContext'; // For dispatching actions locally if needed

// Assuming task object has at least: id, name, status
function TaskItem({ task, sendMessage }) {
  const { dispatch, ACTION_TYPES } = useAppContext();

  const handleStopTask = () => {
    if (window.confirm(`Are you sure you want to stop task: ${task.name}?`)) {
      sendMessage({
        action: 'stop_task', // As per API docs
        payload: { id: task.id }
      });
      // Optionally, optimistically update UI or wait for WebSocket message
      // For example, disable the button or change status locally:
      // dispatch({ type: ACTION_TYPES.UPDATE_TASK, payload: { ...task, status: 'stopping...' } });
    }
  };

  // Basic styling for the task item
  const itemStyle = {
    border: '1px solid #ccc',
    padding: '10px',
    marginBottom: '5px',
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center'
  };

  const taskNameStyle = {
    fontWeight: 'bold',
  };

  const taskStatusStyle = (status) => ({
    fontStyle: 'italic',
    color: status === 'running' ? 'green' : (status === 'stopped' || status === 'failed' ? 'red' : 'gray')
  });

  return (
    <div style={itemStyle}>
      <div>
        <p style={taskNameStyle}>Name: {task.name || 'N/A'}</p>
        <p>ID: {task.id}</p>
        <p>Status: <span style={taskStatusStyle(task.status)}>{task.status || 'N/A'}</span></p>
        {/* Display other task details as available and needed */}
      </div>
      <button
        onClick={handleStopTask}
        disabled={task.status !== 'running' && task.status !== 'pending'} // Example: only allow stopping running/pending tasks
      >
        Stop Task
      </button>
    </div>
  );
}

export default TaskItem;
