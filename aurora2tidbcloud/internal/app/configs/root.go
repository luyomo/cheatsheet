package configs

import (
	"fmt"

	"github.com/pelletier/go-toml"
)

type Config struct {
	SourceDB   SourceConfig `toml:"source_db" json:"RDSConn,omitempty"`
	TargetDB   TargetConfig `toml:"target_db"`
	LambdaVPC  LambdaVPC    `toml:"lambdavpc"`
	BucketInfo BucketInfo   `toml:"bucket_info" json:"S3"`
	TiDBCloud  TiDBCloud    `toml:"tidbcloud"`
}

type SourceConfig struct {
	Host              string `toml:"host" json:"rds_host,omitempty"`
	Port              int    `toml:"port" json:"rds_port,omitempty"`
	User              string `toml:"user" json:"rds_user,omitempty"`
	Password          string `toml:"password" json:"rds_password,omitempty"`
	DB                string `toml:"db" json:"rds_db,omitempty"`
	AuroraClusterName string `toml:"aurora_cluster_name" json:"aurora_cluster_name"`
}

type TargetConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	DB       string `toml:"db"`
}

type LambdaVPC struct {
	VpcID           string   `toml:"vpcid"`
	SecurityGroupID string   `toml:"security_group_id"`
	Subnets         []string `toml:"subnets"`
}

type BucketInfo struct {
	BucketName string `toml:"bucket_name,omitempty" json:"BucketName,omitempty"`
	S3Key      string `toml:"s3key" json:"S3Key,omitempty"`
}

type TiDBCloud struct {
	ProjectID   string `toml:"project_id,omitempty" json:"ProjectId,omitempty"`
	ClusterName string `toml:"cluster_name,omitempty" json:"ClusterName,omitempty"`
}

func ReadConfigFile(configFile string) (*Config, error) {
	config := &Config{}

	// Open the TOML file
	file, err := toml.LoadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file(%s): %v", configFile, err)
	}

	// Unmarshal the TOML data into the Config struct
	if err := file.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file: %v", err)
	}

	return config, nil
}
