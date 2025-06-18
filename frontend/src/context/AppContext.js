import React, { createContext, useReducer, useContext } from 'react';

const AppContext = createContext();

const initialState = {
  tasks: [], // Example: [{id: '1', name: 'My Task', status: 'running'}]
  isLoading: true,
  error: null,
  webSocket: {
    isConnected: false,
    lastMessage: null,
    error: null,
  }
};

// Action types
const ACTION_TYPES = {
  SET_TASKS: 'SET_TASKS',
  ADD_TASK: 'ADD_TASK',
  REMOVE_TASK: 'REMOVE_TASK', // Or mark as stopped/completed
  UPDATE_TASK: 'UPDATE_TASK',
  SET_LOADING: 'SET_LOADING',
  SET_ERROR: 'SET_ERROR',
  SET_WEBSOCKET_STATE: 'SET_WEBSOCKET_STATE',
};

function appReducer(state, action) {
  switch (action.type) {
    case ACTION_TYPES.SET_LOADING:
      return { ...state, isLoading: action.payload };
    case ACTION_TYPES.SET_ERROR:
      return { ...state, error: action.payload, isLoading: false };
    case ACTION_TYPES.SET_TASKS:
      return { ...state, tasks: action.payload, isLoading: false, error: null };
    case ACTION_TYPES.ADD_TASK:
      // Avoid adding duplicates if task already exists by ID
      if (state.tasks.find(task => task.id === action.payload.id)) {
         return {
             ...state,
             tasks: state.tasks.map(task => task.id === action.payload.id ? action.payload : task)
         };
      }
      return { ...state, tasks: [...state.tasks, action.payload] };
    case ACTION_TYPES.REMOVE_TASK: // Assuming payload is task ID
      return { ...state, tasks: state.tasks.filter(task => task.id !== action.payload) };
    case ACTION_TYPES.UPDATE_TASK: // Assuming payload is the updated task object
      return {
        ...state,
        tasks: state.tasks.map(task => (task.id === action.payload.id ? action.payload : task)),
      };
    case ACTION_TYPES.SET_WEBSOCKET_STATE:
      return { ...state, webSocket: { ...state.webSocket, ...action.payload } };
    default:
      return state;
  }
}

export function AppProvider({ children }) {
  const [state, dispatch] = useReducer(appReducer, initialState);
  return (
    <AppContext.Provider value={{ state, dispatch, ACTION_TYPES }}>
      {children}
    </AppContext.Provider>
  );
}

export function useAppContext() {
  const context = useContext(AppContext);
  if (context === undefined) {
    throw new Error('useAppContext must be used within an AppProvider');
  }
  return context;
}
