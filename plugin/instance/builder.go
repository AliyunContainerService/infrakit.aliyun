package instance

import (
	"errors"

	"github.com/denverdino/aliyungo/ecs"
	//"github.com/denverdino/aliyungo/common"
	"log"

	"github.com/docker/infrakit/pkg/spi/instance"

	"github.com/spf13/pflag"
)

type options struct {
	region          string
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	retries         int
}

// Builder is a ProvisionerBuilder that creates an Aliyun instance provisioner.
type Builder struct {
	Client          *ecs.Client
	Region          string
	PrivateIPOnly   bool
	accessKeyID     string
	secretAccessKey string
}

// Flags returns the flags required.
func (b *Builder) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("aliyun", pflag.PanicOnError)
	flags.StringVar(&b.Region, "region", "", "Aliyun region")
	flags.StringVar(&b.accessKeyID, "access-key-id", "", "Access key ID")
	flags.StringVar(&b.secretAccessKey, "access-key-secret", "", "Access key secret")
	flags.BoolVar(&b.PrivateIPOnly, "private-ip", true, "Using private ip only")

	//flags.IntVar(&b.options.retries, "retries", 5, "Number of retries for Aliyun API operations")
	return flags
}

// BuildInstancePlugin creates an instance Provisioner configured with the Flags.
func (b *Builder) BuildInstancePlugin(namespace map[string]string) (instance.Plugin, error) {
	if b.Client == nil {
		b.Client = ecs.NewClient(b.accessKeyID, b.secretAccessKey)
		if b.Region == "" {
			log.Println("region not specified, attempting to discover from Aliyun instance metadata")
			region, err := GetRegion()
			if err != nil {
				return nil, errors.New("Unable to determine region")
			}

			log.Printf("Defaulting to local region %s\n", region)
			b.Region = region
		}
	}

	return NewInstancePlugin(b), nil
}

type logger struct {
	logger *log.Logger
}

func (l logger) Log(args ...interface{}) {
	l.logger.Println(args...)
}
