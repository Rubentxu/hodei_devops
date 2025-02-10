import {
    CaretLeft,
    CaretRight,
    ChartLine,
    Gear,
    House,
    ListChecks,
    Terminal
} from "@phosphor-icons/react";
import { Box, Button, Flex, Text } from '@radix-ui/themes';
import React from 'react';
import { Link, useLocation } from 'react-router-dom';

interface SidebarProps {
  isMobile: boolean;
  isOpen: boolean;
  onToggle: () => void;
}

const Sidebar: React.FC<SidebarProps> = ({ isMobile, isOpen, onToggle }) => {
  const location = useLocation();

  const menuItems = [
    { icon: <House size={24} />, label: 'Dashboard', path: '/' },
    { icon: <ListChecks size={24} />, label: 'Tasks', path: '/tasks' },
    { icon: <Terminal size={24} />, label: 'Console', path: '/console' },
    { icon: <ChartLine size={24} />, label: 'Analytics', path: '/analytics' },
    { icon: <Gear size={24} />, label: 'Settings', path: '/settings' },
  ];

  return (
    <Box
      style={{
        position: isMobile ? 'fixed' : 'sticky',
        top: isMobile ? 0 : '64px',
        left: 0,
        height: isMobile ? '100vh' : 'calc(100vh - 64px)',
        width: isOpen ? '240px' : '60px',
        backgroundColor: 'var(--color-background)',
        borderRight: '1px solid var(--gray-a5)',
        transition: 'width 0.3s ease',
        zIndex: 100,
        overflow: 'hidden',
      }}
    >
      <Flex direction="column" p="2">
        <Button
          variant="ghost"
          onClick={onToggle}
          style={{
            alignSelf: 'flex-end',
            padding: '8px',
            marginBottom: '16px'
          }}
        >
          {isOpen ? <CaretLeft size={24} /> : <CaretRight size={24} />}
        </Button>

        {menuItems.map((item) => (
          <Link
            key={item.path}
            to={item.path}
            style={{ textDecoration: 'none', color: 'inherit' }}
          >
            <Button
              variant={location.pathname === item.path ? 'solid' : 'ghost'}
              style={{
                width: '100%',
                justifyContent: isOpen ? 'flex-start' : 'center',
                marginBottom: '8px',
              }}
            >
              <Flex align="center" gap="3">
                {item.icon}
                {isOpen && <Text>{item.label}</Text>}
              </Flex>
            </Button>
          </Link>
        ))}
      </Flex>
    </Box>
  );
};

export default Sidebar;