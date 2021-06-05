package imagegate

import (
	"errors"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
)

type Config struct {
	Port                int
	Region              string
	S3Endpoint          string
	RekognitionEndpoint string
	ViewIndex           bool
	Bucket              string
	KeyPrefix           string
}

func (cfg *Config) NewHander() (*Handler, error) {
	sess, err := cfg.newAwsSession()
	if err != nil {
		return nil, err
	}
	if cfg.Bucket == "" {
		return nil, errors.New("bucket is required")
	}
	return newHandler(sess, cfg.Bucket, cfg.KeyPrefix, cfg.ViewIndex), nil
}

func DefaultConfig() *Config {
	return &Config{
		Port:   8000,
		Region: os.Getenv("AWS_DEFAULT_REGION"),
	}
}

func (cfg *Config) newAwsSession() (*session.Session, error) {
	awsCfg := aws.NewConfig().WithRegion(cfg.Region)
	if cfg.S3Endpoint != "" || cfg.RekognitionEndpoint != "" {
		defaultResolver := endpoints.DefaultResolver()
		awsCfg.WithEndpointResolver(endpoints.ResolverFunc(
			func(service, region string, opts ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
				switch service {
				case endpoints.S3ServiceID:
					if cfg.S3Endpoint != "" {
						return endpoints.ResolvedEndpoint{
							URL:           cfg.S3Endpoint,
							SigningRegion: region,
						}, nil
					}
				case endpoints.RekognitionServiceID:
					if cfg.RekognitionEndpoint != "" {
						return endpoints.ResolvedEndpoint{
							URL:           cfg.RekognitionEndpoint,
							SigningRegion: region,
						}, nil
					}
				}
				return defaultResolver.EndpointFor(service, region, opts...)
			},
		))
	}
	return session.NewSessionWithOptions(session.Options{
		Config:            *awsCfg,
		SharedConfigState: session.SharedConfigEnable,
	})
}
