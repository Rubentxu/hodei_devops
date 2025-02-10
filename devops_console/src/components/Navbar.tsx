import { List, Terminal } from "@phosphor-icons/react";
import { Box, Flex, IconButton, Text } from '@radix-ui/themes';
import React from 'react';
import ThemeSelector, { ThemeConfig } from './ThemeSelector';

interface NavbarProps {
  currentTheme: ThemeConfig;
  onThemeChange: (theme: ThemeConfig) => void;
  onToggleSidebar: () => void;
  isMobile: boolean;
}

const Navbar: React.FC<NavbarProps> = ({ 
  currentTheme, 
  onThemeChange, 
  onToggleSidebar,
  isMobile 
}) => {
  return (
    <Box style={{ 
      borderBottom: '1px solid var(--gray-a5)', 
      position: 'sticky',
      top: 0,
      zIndex: 1000,
      backgroundColor: 'var(--color-background)'
    }}>
      <Flex 
        px={{ initial: '3', sm: '4' }} 
        py={{ initial: '2', sm: '4' }} 
        justify="between" 
        align="center"
        gap="4"
      >
        <Flex align="center" gap={{ initial: '2', sm: '3' }}>
          <IconButton
            size={{ initial: '2', sm: '3' }}
            variant="ghost"
            onClick={onToggleSidebar}
            aria-label="Toggle sidebar"
          >
            <List size={28} weight="bold" />
          </IconButton>
          <Terminal size={32} weight="duotone" style={{ flexShrink: 0 }} />
          <Text 
            size={{ initial: '4', sm: '5' }} 
            weight="bold" 
            style={{ 
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis'
            }}
          >
            DevOps Console
          </Text>
        </Flex>
        
        <ThemeSelector
          currentTheme={currentTheme}
          onThemeChange={onThemeChange}
        />
      </Flex>
    </Box>
  );
};

export default Navbar;
