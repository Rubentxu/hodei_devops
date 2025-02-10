import React from 'react';
import { Button } from "./ui/button"
import { Input } from "./ui/input"
import { Label } from "./ui/label"
import { Plus, Trash } from "@phosphor-icons/react"

interface EnvVar {
  key: string;
  value: string;
}

interface EnvVarsEditorProps {
  envVars: Record<string, string>;
  onChange: (envVars: Record<string, string>) => void;
}

export const EnvVarsEditor: React.FC<EnvVarsEditorProps> = ({ envVars, onChange }) => {
  const envVarsArray = Object.entries(envVars).map(([key, value]) => ({ key, value }));

  const handleAddVar = () => {
    const newEnvVars = { ...envVars, '': '' };
    onChange(newEnvVars);
  };

  const handleRemoveVar = (key: string) => {
    const newEnvVars = { ...envVars };
    delete newEnvVars[key];
    onChange(newEnvVars);
  };

  const handleVarChange = (oldKey: string, newKey: string, value: string) => {
    const newEnvVars = { ...envVars };
    if (oldKey !== newKey) {
      delete newEnvVars[oldKey];
    }
    newEnvVars[newKey] = value;
    onChange(newEnvVars);
  };

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <Label>Variables de Entorno</Label>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={handleAddVar}
        >
          <Plus className="w-4 h-4 mr-2" />
          Añadir Variable
        </Button>
      </div>
      <div className="space-y-2">
        {envVarsArray.map(({ key, value }, index) => (
          <div key={index} className="flex gap-2 items-start">
            <div className="flex-1">
              <Input
                placeholder="NOMBRE_VARIABLE"
                value={key}
                onChange={(e) => handleVarChange(key, e.target.value, value)}
              />
            </div>
            <div className="flex-1">
              <Input
                placeholder="valor"
                value={value}
                onChange={(e) => handleVarChange(key, key, e.target.value)}
              />
            </div>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => handleRemoveVar(key)}
            >
              <Trash className="w-4 h-4 text-red-500" />
            </Button>
          </div>
        ))}
      </div>
    </div>
  );
};
