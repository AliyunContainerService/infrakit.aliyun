package main

import (
	"os"

	"strings"

	"github.com/AliyunContainerService/infrakit.aliyun/plugin"
	"github.com/AliyunContainerService/infrakit.aliyun/plugin/instance"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/cli"
	instance_plugin "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/spf13/cobra"
)

func main() {

	builder := &instance.Builder{}

	var logLevel int
	var name string
	var namespaceTags []string
	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Aliyun instance plugin",
		Run: func(c *cobra.Command, args []string) {

			namespace := map[string]string{}
			for _, tagKV := range namespaceTags {
				keyAndValue := strings.Split(tagKV, "=")
				if len(keyAndValue) != 2 {
					log.Error("Namespace tags must be formatted as key=value")
					os.Exit(1)
				}

				namespace[keyAndValue[0]] = keyAndValue[1]
			}

			instancePlugin, err := builder.BuildInstancePlugin(namespace)
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			cli.SetLogLevel(logLevel)
			cli.RunPlugin(name, instance_plugin.PluginServer(instancePlugin))
		},
	}

	cmd.Flags().IntVar(&logLevel, "log", cli.DefaultLogLevel, "Logging level. 0 is least verbose. Max is 5")
	cmd.Flags().StringVar(&name, "name", "instance-aliyun", "Plugin name to advertise for discovery")
	cmd.Flags().StringSliceVar(
		&namespaceTags,
		"namespace-tags",
		[]string{},
		"A list of key=value resource tags to namespace all resources created")

	// TODO(chungers) - the exposed flags here won't be set in plugins, because plugin install doesn't allow
	// user to pass in command line args like containers with entrypoint.
	cmd.Flags().AddFlagSet(builder.Flags())

	cmd.AddCommand(plugin.VersionCommand())

	err := cmd.Execute()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
