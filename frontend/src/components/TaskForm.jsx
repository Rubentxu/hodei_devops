import React, { useState } from 'react';

function TaskForm({ sendMessage }) {
  const [taskName, setTaskName] = useState('');
  // Potentially add more fields for task creation if needed by backend
  // e.g., image, command, etc. For now, just a name.

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!taskName.trim()) {
      alert('Task name cannot be empty.');
      return;
    }
    // The backend swagger implies actions like 'create_task'
    // The payload structure needs to align with backend expectations.
    // Assuming a simple structure for now:
    const taskPayload = {
      // id will be assigned by backend
      name: taskName,
      // status will be set by backend, e.g., 'pending' or 'queued'
    };
    sendMessage({
      action: 'create_task', // As per API docs
      payload: taskPayload
    });
    setTaskName(''); // Clear input after sending
  };

  return (
    <form onSubmit={handleSubmit}>
      <h3>Create New Task</h3>
      <input
        type="text"
        value={taskName}
        onChange={(e) => setTaskName(e.target.value)}
        placeholder="Enter task name"
      />
      <button type="submit">Create Task</button>
    </form>
  );
}

export default TaskForm;
