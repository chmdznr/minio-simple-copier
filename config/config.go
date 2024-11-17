package config

type MinioConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL         bool
	BucketName     string
}

type DestinationType string

const (
	DestinationMinio DestinationType = "minio"
	DestinationLocal DestinationType = "local"
)

type LocalConfig struct {
	Path string
}

type ProjectConfig struct {
	ProjectName    string
	SourceMinio    MinioConfig
	DestType       DestinationType
	DestMinio      MinioConfig    // Used when DestType is DestinationMinio
	DestLocal      LocalConfig    // Used when DestType is DestinationLocal
	DatabasePath   string
}
