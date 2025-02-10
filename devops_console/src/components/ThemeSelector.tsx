import React from 'react';
import { Box, DropdownMenu, Flex, IconButton } from '@radix-ui/themes';
import { Palette } from "@phosphor-icons/react";

export type ThemeConfig = {
  name: string;
  appearance: 'light' | 'dark';
  accentColor: string;
  radius: 'none' | 'small' | 'medium' | 'large' | 'full';
  scaling: string;
};

const themes: ThemeConfig[] = [
  {
    name: 'Moderno Oscuro',
    appearance: 'dark',
    accentColor: 'cyan',
    radius: 'medium',
    scaling: '95%'
  },
  {
    name: 'Clásico Claro',
    appearance: 'light',
    accentColor: 'blue',
    radius: 'small',
    scaling: '100%'
  },
  {
    name: 'Neón Oscuro',
    appearance: 'dark',
    accentColor: 'lime',
    radius: 'full',
    scaling: '95%'
  },
  {
    name: 'Minimalista',
    appearance: 'light',
    accentColor: 'gray',
    radius: 'none',
    scaling: '90%'
  },
  {
    name: 'Corporativo',
    appearance: 'light',
    accentColor: 'indigo',
    radius: 'medium',
    scaling: '100%'
  },
  {
    name: 'Futurista',
    appearance: 'dark',
    accentColor: 'violet',
    radius: 'large',
    scaling: '95%'
  }
];

interface ThemeSelectorProps {
  currentTheme: ThemeConfig;
  onThemeChange: (theme: ThemeConfig) => void;
}

const ThemeSelector: React.FC<ThemeSelectorProps> = ({ currentTheme, onThemeChange }) => {
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger>
        <IconButton
          size="3"
          variant="ghost"
          aria-label="Selector de tema"
        >
          <Palette size={28} weight="duotone" />
        </IconButton>
      </DropdownMenu.Trigger>

      <DropdownMenu.Content>
        {themes.map((theme) => (
          <DropdownMenu.Item
            key={theme.name}
            onClick={() => onThemeChange(theme)}
          >
            <Flex align="center" gap="2">
              <Box
                style={{
                  width: 16,
                  height: 16,
                  borderRadius: '50%',
                  backgroundColor: `var(--${theme.accentColor}-9)`,
                }}
              />
              {theme.name}
              {currentTheme.name === theme.name && ' ✓'}
            </Flex>
          </DropdownMenu.Item>
        ))}
      </DropdownMenu.Content>
    </DropdownMenu.Root>
  );
};

export default ThemeSelector;
