package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

type Config = aws.Config

func NewConfig(accessKeyID string, secretAccessKey string, region ...string) *Config {
	ctx := context.Background()
	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")))
	if len(region) == 1 {
		opts = append(opts, config.WithRegion(region[0]))
	}

	c, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return &c

}
