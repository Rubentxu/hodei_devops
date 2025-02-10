import { Box, Container, Flex, Theme } from '@radix-ui/themes'
import '@radix-ui/themes/styles.css'
import { useEffect, useState } from 'react'
import { Route, BrowserRouter as Router, Routes, Navigate } from 'react-router-dom'
import Navbar from './components/Navbar'
import Sidebar from './components/Sidebar'
import TaskList from './components/TaskList'
import TaskTerminal from './components/TaskTerminal'
import TaskConsole from './components/TaskConsole'
import { useTaskStore } from './store/useTaskStore'
import './styles/global.css'
import { ThemeConfig } from './components/ThemeSelector'

const defaultTheme: ThemeConfig = {
  name: 'Moderno Oscuro',
  appearance: 'dark',
  accentColor: 'cyan',
  radius: 'medium',
  scaling: '95%'
};

function App() {
  const { connect, disconnect } = useTaskStore()
  const [theme, setTheme] = useState<ThemeConfig>(() => {
    const savedTheme = localStorage.getItem('themeConfig');
    return savedTheme ? JSON.parse(savedTheme) : defaultTheme;
  });
  const [isSidebarOpen, setSidebarOpen] = useState(true);
  const [isMobile, setIsMobile] = useState(window.innerWidth < 768);

  useEffect(() => {
    connect()
    return () => disconnect()
  }, [])

  useEffect(() => {
    const handleResize = () => {
      setIsMobile(window.innerWidth < 768);
      if (window.innerWidth < 768) {
        setSidebarOpen(false);
      }
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  const handleThemeChange = (newTheme: ThemeConfig) => {
    setTheme(newTheme);
    localStorage.setItem('themeConfig', JSON.stringify(newTheme));
  };

  const toggleSidebar = () => {
    setSidebarOpen(prev => !prev);
  };

  return (
    <Router>
      <Theme 
        appearance={theme.appearance}
        accentColor={theme.accentColor}
        radius={theme.radius}
        scaling={theme.scaling}
      >
        <Box style={{ 
          minHeight: '100vh',
          backgroundColor: 'var(--color-background)',
        }}>
          <Navbar 
            currentTheme={theme}
            onThemeChange={handleThemeChange}
            onToggleSidebar={toggleSidebar}
            isMobile={isMobile}
          />
          <Flex>
            <Sidebar 
              isMobile={isMobile}
              isOpen={isSidebarOpen}
              onToggle={toggleSidebar}
            />
            <Box
              style={{
                flex: 1,
                marginLeft: isSidebarOpen ? '240px' : '60px',
                transition: 'margin-left 0.3s ease',
                width: '100%',
              }}
            >
              <Container size={{ initial: '1', sm: '2', md: '3', lg: '4' }}>
                <Box py={{ initial: '2', sm: '4' }} px={{ initial: '2', sm: '4' }}>
                  <Routes>
                    <Route path="/" element={<Navigate to="/tasks" replace />} />
                    <Route path="/tasks" element={<TaskList />} />
                    <Route path="/terminal" element={<TaskTerminal />} />
                    <Route path="/console" element={<TaskConsole />} />
                    <Route path="/analytics" element={<div>Analytics (próximamente)</div>} />
                    <Route path="/settings" element={<div>Settings (próximamente)</div>} />
                  </Routes>
                </Box>
              </Container>
            </Box>
          </Flex>
        </Box>
      </Theme>
    </Router>
  )
}

export default App