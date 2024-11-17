package config

type MinioConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL         bool
	BucketName     string
}

type ProjectConfig struct {
	ProjectName    string
	SourceMinio    MinioConfig
	DestMinio      MinioConfig
	DatabasePath   string
}
