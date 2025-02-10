import React, { useEffect, useRef } from 'react';
import { Button, Card, Flex, ScrollArea, Text } from '@radix-ui/themes';
import { useTaskStore } from '../store/useTaskStore';
import { ArrowLeft, Copy, Terminal as TerminalIcon } from '@phosphor-icons/react';
import { useNavigate } from 'react-router-dom';

// Tema de colores inspirado en terminales modernas
const theme = {
  background: '#011627',
  foreground: '#d6deeb',
  black: '#011627',
  red: '#EF5350',
  green: '#22da6e',
  yellow: '#addb67',
  blue: '#82AAFF',
  magenta: '#c792ea',
  cyan: '#21c7a8',
  white: '#ffffff',
  brightBlack: '#575656',
  brightRed: '#ef5350',
  brightGreen: '#22da6e',
  brightYellow: '#ffeb95',
  brightBlue: '#82AAFF',
  brightMagenta: '#c792ea',
  brightCyan: '#7fdbca',
  brightWhite: '#ffffff',
};

const TaskTerminal: React.FC = () => {
  const navigate = useNavigate();
  const { selectedTask, taskOutput, clearTaskOutput } = useTaskStore();
  const consoleRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!selectedTask) {
      navigate('/');
      return;
    }

    if (consoleRef.current) {
      consoleRef.current.scrollTop = consoleRef.current.scrollHeight;
    }
  }, [selectedTask, taskOutput, navigate]);

  const handleBack = () => {
    clearTaskOutput();
    navigate('/');
  };

  const handleCopyOutput = () => {
    if (taskOutput) {
      navigator.clipboard.writeText(taskOutput);
    }
  };

  const getLineStyle = (line: string) => {
    const lowerLine = line.toLowerCase();
    const styles: React.CSSProperties = {
      fontFamily: 'JetBrains Mono, Fira Code, Menlo, Monaco, Consolas, monospace',
      fontSize: '14px',
      lineHeight: '1.6',
      display: 'flex',
      alignItems: 'flex-start',
      gap: '8px',
      position: 'relative',
      paddingLeft: line.startsWith('>') ? '24px' : '4px',
    };

    // Añadir un prompt estilo terminal
    if (line.startsWith('>')) {
      styles.color = theme.brightCyan;
    } else if (line.startsWith('$')) {
      styles.color = theme.brightMagenta;
    }
    // Colores según el tipo de mensaje
    else if (lowerLine.includes('error')) {
      styles.color = theme.brightRed;
    } else if (lowerLine.includes('warning')) {
      styles.color = theme.brightYellow;
    } else if (lowerLine.includes('success') || lowerLine.includes('completed')) {
      styles.color = theme.brightGreen;
    } else if (lowerLine.includes('info') || lowerLine.startsWith('[info]')) {
      styles.color = theme.brightBlue;
    } else if (lowerLine.includes('debug') || lowerLine.startsWith('[debug]')) {
      styles.color = theme.brightBlack;
    } else if (lowerLine.includes('running') || lowerLine.includes('executing')) {
      styles.color = theme.brightMagenta;
    } else {
      styles.color = theme.foreground;
    }

    return styles;
  };

  if (!selectedTask) return null;

  return (
    <Flex direction="column" gap="3">
      <Card style={{
        backgroundColor: theme.background,
        border: `1px solid ${theme.brightBlack}`,
        borderRadius: '8px',
      }}>
        <Flex justify="between" align="center">
          <Flex align="center" gap="2">
            <Button variant="ghost" onClick={handleBack} style={{
              color: theme.foreground,
            }}>
              <ArrowLeft weight="bold" />
              Volver
            </Button>
            <TerminalIcon size={24} weight="bold" style={{ color: theme.brightCyan }} />
            <Text size="5" weight="bold" style={{ color: theme.foreground }}>
              {selectedTask.display_name || selectedTask.name}
            </Text>
          </Flex>
          <Flex align="center" gap="3">
            <Text style={{ color: theme.foreground }}>
              Estado: <span style={{ 
                color: selectedTask.state === 'completed' ? theme.brightGreen : 
                       selectedTask.state === 'running' ? theme.brightBlue :
                       selectedTask.state === 'failed' ? theme.brightRed : 
                       theme.foreground,
                fontWeight: 'bold'
              }}>{selectedTask.state}</span>
            </Text>
            <Button variant="soft" onClick={handleCopyOutput} style={{
              backgroundColor: theme.brightBlack,
              color: theme.foreground,
            }}>
              <Copy weight="bold" />
              Copiar Output
            </Button>
          </Flex>
        </Flex>
      </Card>

      <Card style={{ 
        height: 'calc(100vh - 200px)', 
        backgroundColor: theme.background,
        border: `1px solid ${theme.brightBlack}`,
        borderRadius: '8px',
        boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)',
      }}>
        <ScrollArea style={{ height: '100%' }}>
          <div 
            ref={consoleRef}
            style={{ 
              padding: '16px',
              backgroundColor: theme.background,
            }}
          >
            {taskOutput ? (
              taskOutput.split('\n').map((line, index) => (
                <div key={index} style={getLineStyle(line)}>
                  {line.startsWith('>') && (
                    <span style={{ color: theme.brightMagenta, marginRight: '8px' }}>❯</span>
                  )}
                  {line}
                </div>
              ))
            ) : (
              <Text style={{ 
                color: theme.brightBlack,
                fontStyle: 'italic',
                display: 'flex',
                alignItems: 'center',
                gap: '8px'
              }}>
                <TerminalIcon size={16} />
                Esperando la salida de la tarea...
              </Text>
            )}
            {selectedTask.state === 'completed' && (
              <div style={{ 
                color: theme.brightGreen,
                marginTop: '16px',
                borderTop: `1px solid ${theme.brightBlack}`,
                paddingTop: '16px',
                display: 'flex',
                alignItems: 'center',
                gap: '8px'
              }}>
                ✓ Tarea completada exitosamente
              </div>
            )}
            {selectedTask.state === 'failed' && (
              <div style={{ 
                color: theme.brightRed,
                marginTop: '16px',
                borderTop: `1px solid ${theme.brightBlack}`,
                paddingTop: '16px',
                display: 'flex',
                alignItems: 'center',
                gap: '8px'
              }}>
                ✗ La tarea ha fallado: {selectedTask.status.error}
              </div>
            )}
          </div>
        </ScrollArea>
      </Card>
    </Flex>
  );
};

export default TaskTerminal;
