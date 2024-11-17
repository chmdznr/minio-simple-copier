package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ProjectMinioConfig struct {
	Source MinioConfig `yaml:"source"`
	DestType DestinationType `yaml:"destType"`
	Dest   *MinioConfig `yaml:"dest,omitempty"`
	Local  *LocalConfig `yaml:"local,omitempty"`
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

	// Debug: Print loaded config
	log.Printf("Loaded config: %+v", config)

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

func (c *FileConfig) GetProjectConfig(projectName string) (ProjectConfig, bool) {
	minioConfig, exists := c.Projects[projectName]
	if !exists {
		return ProjectConfig{}, false
	}

	// Debug: Print project config
	log.Printf("Project config for %s: %+v", projectName, minioConfig)

	config := ProjectConfig{
		ProjectName: projectName,
		SourceMinio: minioConfig.Source,
		DestType:    minioConfig.DestType,
	}

	switch minioConfig.DestType {
	case DestinationMinio:
		if minioConfig.Dest != nil {
			config.DestMinio = *minioConfig.Dest
		}
	case DestinationLocal:
		if minioConfig.Local != nil {
			config.DestLocal = *minioConfig.Local
		}
	}

	// Debug: Print final config
	log.Printf("Final config for %s: %+v", projectName, config)

	return config, true
}

func (c *FileConfig) SetProjectConfig(projectName string, config ProjectConfig) {
	minioConfig := ProjectMinioConfig{
		Source:   config.SourceMinio,
		DestType: config.DestType,
	}

	switch config.DestType {
	case DestinationMinio:
		minioConfig.Dest = &config.DestMinio
	case DestinationLocal:
		minioConfig.Local = &config.DestLocal
	}

	c.Projects[projectName] = minioConfig
}
