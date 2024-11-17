package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type FileConfig struct {
	Projects map[string]ProjectMinioConfig `yaml:"projects"`
}

func LoadConfig(projectsDir string) (*FileConfig, error) {
	configPath := filepath.Join(projectsDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &FileConfig{
			Projects: make(map[string]ProjectMinioConfig),
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
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

func (f *FileConfig) GetProjectConfig(projectName string) (*ProjectConfig, error) {
	minioConfig, ok := f.Projects[projectName]
	if !ok {
		return nil, fmt.Errorf("project %s not found", projectName)
	}

	// Convert from old format to new format
	config := &ProjectConfig{
		ProjectName: projectName,
		SourceMinio: MinioConfig{
			Endpoint:        minioConfig.Source.Endpoint,
			AccessKeyID:     minioConfig.Source.AccessKeyID,
			SecretAccessKey: minioConfig.Source.SecretAccessKey,
			UseSSL:         minioConfig.Source.UseSSL,
			BucketName:     minioConfig.Source.BucketName,
			FolderPath:     minioConfig.Source.FolderPath,
		},
		DestType: minioConfig.DestType,
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
				FolderPath:     minioConfig.Dest.FolderPath,
			}
		}
	case DestinationLocal:
		if minioConfig.Local != nil {
			config.DestLocal = LocalConfig{
				Path: minioConfig.Local.Path,
			}
		}
	}

	return config, nil
}

func (f *FileConfig) SetProjectConfig(projectName string, cfg ProjectConfig) {
	if f.Projects == nil {
		f.Projects = make(map[string]ProjectMinioConfig)
	}

	// Convert from new format to old format
	minioConfig := ProjectMinioConfig{
		Source: MinioConfig{
			Endpoint:        cfg.SourceMinio.Endpoint,
			AccessKeyID:     cfg.SourceMinio.AccessKeyID,
			SecretAccessKey: cfg.SourceMinio.SecretAccessKey,
			UseSSL:         cfg.SourceMinio.UseSSL,
			BucketName:     cfg.SourceMinio.BucketName,
			FolderPath:     cfg.SourceMinio.FolderPath,
		},
		DestType: cfg.DestType,
	}

	switch cfg.DestType {
	case DestinationMinio:
		minioConfig.Dest = &MinioConfig{
			Endpoint:        cfg.DestMinio.Endpoint,
			AccessKeyID:     cfg.DestMinio.AccessKeyID,
			SecretAccessKey: cfg.DestMinio.SecretAccessKey,
			UseSSL:         cfg.DestMinio.UseSSL,
			BucketName:     cfg.DestMinio.BucketName,
			FolderPath:     cfg.DestMinio.FolderPath,
		}
	case DestinationLocal:
		minioConfig.Local = &LocalConfig{
			Path: cfg.DestLocal.Path,
		}
	}

	f.Projects[projectName] = minioConfig
}
