import { useState, useEffect, useRef } from 'react';

const WEBSOCKET_URL = 'ws://localhost:8080/ws'; // Assuming default backend URL

function useWebSocket() {
  const [socket, setSocket] = useState(null);
  const [isConnected, setIsConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState(null);
  const [error, setError] = useState(null);

  const clientIdRef = useRef(`client-${Math.random().toString(36).substring(2, 15)}`);

  useEffect(() => {
    const ws = new WebSocket(`${WEBSOCKET_URL}?client_id=${clientIdRef.current}`);

    ws.onopen = () => {
      console.log('WebSocket connected');
      setSocket(ws);
      setIsConnected(true);
      setError(null);
    };

    ws.onmessage = (event) => {
      console.log('WebSocket message received:', event.data);
      setLastMessage(event.data);
    };

    ws.onerror = (err) => {
      console.error('WebSocket error:', err);
      setError(err);
      setIsConnected(false);
    };

    ws.onclose = () => {
      console.log('WebSocket disconnected');
      setSocket(null);
      setIsConnected(false);
    };

    // Cleanup function to close WebSocket on component unmount
    return () => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
    };
  }, []); // Empty dependency array means this effect runs once on mount

  const sendMessage = (message) => {
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify(message));
    } else {
      console.error('WebSocket is not connected.');
    }
  };

  return {
    isConnected,
    lastMessage,
    error,
    sendMessage,
  };
}

export default useWebSocket;
