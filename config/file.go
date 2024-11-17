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

	// Make deep copies of all structs
	config := ProjectConfig{
		ProjectName: projectName,
		SourceMinio: MinioConfig{
			Endpoint:        minioConfig.Source.Endpoint,
			AccessKeyID:     minioConfig.Source.AccessKeyID,
			SecretAccessKey: minioConfig.Source.SecretAccessKey,
			UseSSL:         minioConfig.Source.UseSSL,
			BucketName:     minioConfig.Source.BucketName,
		},
		DestType:     minioConfig.DestType,
		DatabasePath: filepath.Join("projects", projectName, "files.db"),
	}

	switch minioConfig.DestType {
	case DestinationMinio:
		if minioConfig.Dest != nil {
			config.DestMinio = MinioConfig{
				Endpoint:        minioConfig.Dest.Endpoint,
				AccessKeyID:     minioConfig.Dest.AccessKeyID,
				SecretAccessKey: minioConfig.Dest.SecretAccessKey,
				UseSSL:         minioConfig.Dest.UseSSL,
				BucketName:     minioConfig.Dest.BucketName,
			}
		}
	case DestinationLocal:
		if minioConfig.Local != nil {
			config.DestLocal = LocalConfig{
				Path: minioConfig.Local.Path,
			}
		}
	}

	// Debug: Print final config
	log.Printf("Final config for %s: %+v", projectName, config)

	return config, true
}

func (c *FileConfig) SetProjectConfig(projectName string, config ProjectConfig) {
	minioConfig := ProjectMinioConfig{
		Source: MinioConfig{
			Endpoint:        config.SourceMinio.Endpoint,
			AccessKeyID:     config.SourceMinio.AccessKeyID,
			SecretAccessKey: config.SourceMinio.SecretAccessKey,
			UseSSL:         config.SourceMinio.UseSSL,
			BucketName:     config.SourceMinio.BucketName,
		},
		DestType: config.DestType,
	}

	switch config.DestType {
	case DestinationMinio:
		minioConfig.Dest = &MinioConfig{
			Endpoint:        config.DestMinio.Endpoint,
			AccessKeyID:     config.DestMinio.AccessKeyID,
			SecretAccessKey: config.DestMinio.SecretAccessKey,
			UseSSL:         config.DestMinio.UseSSL,
			BucketName:     config.DestMinio.BucketName,
		}
	case DestinationLocal:
		minioConfig.Local = &LocalConfig{
			Path: config.DestLocal.Path,
		}
	}

	c.Projects[projectName] = minioConfig
}
