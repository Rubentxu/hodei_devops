import React, { useEffect } from 'react';
import './App.css'; // Assuming basic styling
import TaskList from './components/TaskList';
import TaskForm from './components/TaskForm';
import useWebSocket from './hooks/useWebSocket';
import { useAppContext } from './context/AppContext'; // Corrected import path

function App() {
  const { state, dispatch, ACTION_TYPES } = useAppContext();
  const { isConnected, lastMessage, error: wsError, sendMessage } = useWebSocket();

  useEffect(() => {
    dispatch({
      type: ACTION_TYPES.SET_WEBSOCKET_STATE,
      payload: { isConnected, error: wsError },
    });
  }, [isConnected, wsError, dispatch, ACTION_TYPES]);

  useEffect(() => {
    if (lastMessage) {
      try {
        const message = JSON.parse(lastMessage);
        console.log('Parsed WebSocket message:', message);

        // Example message handling logic (adjust based on actual backend messages)
        // The backend swagger mentions 'create_task', 'stop_task', 'list_tasks' as actions for /ws
        // We will assume the server sends messages that reflect these actions or their results.
        // For instance, after a 'list_tasks' request, the server might send a message like:
        // { type: 'TASK_LIST', payload: [{id: '1', name: 'Task 1', status: 'running'}, ...] }
        // Or when a task is updated:
        // { type: 'TASK_UPDATED', payload: {id: '1', name: 'Task 1', status: 'stopped'} }

        if (message.type === 'TASK_LIST') {
          dispatch({ type: ACTION_TYPES.SET_TASKS, payload: message.payload });
        } else if (message.type === 'TASK_CREATED') {
          dispatch({ type: ACTION_TYPES.ADD_TASK, payload: message.payload });
        } else if (message.type === 'TASK_UPDATED') {
          dispatch({ type: ACTION_TYPES.UPDATE_TASK, payload: message.payload });
        } else if (message.type === 'TASK_REMOVED') { // Or TASK_STOPPED leading to removal
          dispatch({ type: ACTION_TYPES.REMOVE_TASK, payload: message.payload.id });
        }
        // Add more conditions as per actual messages from backend
      } catch (e) {
        console.error('Failed to parse WebSocket message or dispatch action:', e);
        dispatch({ type: ACTION_TYPES.SET_ERROR, payload: 'Error processing message from server.' });
      }
    }
  }, [lastMessage, dispatch, ACTION_TYPES]);

  // Function to request initial task list when connected
  useEffect(() => {
    if (isConnected) {
      sendMessage({ action: 'list_tasks' });
      // Set loading state until tasks are received or an error occurs
      dispatch({ type: ACTION_TYPES.SET_LOADING, payload: true });
    }
  }, [isConnected, sendMessage, dispatch, ACTION_TYPES]);

  return (
    <div className="App">
      <header className="App-header">
        <h1>Task Orchestrator Frontend</h1>
      </header>
      <main>
        {state.webSocket.isConnected ? (
          <>
            <TaskForm sendMessage={sendMessage} />
            {state.isLoading && <p>Loading tasks...</p>}
            {state.error && <p style={{color: 'red'}}>{state.error}</p>}
            {!state.isLoading && !state.error && <TaskList sendMessage={sendMessage} />}
          </>
        ) : (
          <p>Connecting to WebSocket...</p>
        )}
        {state.webSocket.error && (
          <p style={{ color: 'red' }}>WebSocket Connection Error. Try refreshing.</p>
        )}
      </main>
    </div>
  );
}

export default App;
