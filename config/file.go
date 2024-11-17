package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ProjectMinioConfig struct {
	Source MinioConfig `yaml:"source"`
	Dest   MinioConfig `yaml:"dest"`
}

type FileConfig struct {
	Projects map[string]ProjectMinioConfig `yaml:"projects"`
}

func LoadConfig(projectsDir string) (*FileConfig, error) {
	configPath := filepath.Join(projectsDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return &FileConfig{
				Projects: make(map[string]ProjectMinioConfig),
			}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config FileConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Projects == nil {
		config.Projects = make(map[string]ProjectMinioConfig)
	}

	return &config, nil
}

func SaveConfig(projectsDir string, config *FileConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(projectsDir, "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *FileConfig) GetProjectConfig(projectName string) (ProjectMinioConfig, bool) {
	config, exists := c.Projects[projectName]
	return config, exists
}

func (c *FileConfig) SetProjectConfig(projectName string, config ProjectMinioConfig) {
	c.Projects[projectName] = config
}
