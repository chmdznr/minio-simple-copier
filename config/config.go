package config

type MinioConfig struct {
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"accesskeyid"`
	SecretAccessKey string `yaml:"secretaccesskey"`
	UseSSL          bool   `yaml:"usessl"`
	BucketName      string `yaml:"bucketname"`
	FolderPath      string `yaml:"folderpath"`  // e.g., "naskah-keluar/"
}

type DestinationType string

const (
	DestinationMinio DestinationType = "minio"
	DestinationLocal DestinationType = "local"
)

type LocalConfig struct {
	Path string `yaml:"path"`
}

type ProjectConfig struct {
	ProjectName  string          `yaml:"projectname"`
	SourceMinio  MinioConfig     `yaml:"sourceminio"`
	DestType     DestinationType `yaml:"desttype"`
	DestMinio    MinioConfig     `yaml:"destminio"`
	DestLocal    LocalConfig     `yaml:"destlocal"`
	DatabasePath string          `yaml:"databasepath"`
}
