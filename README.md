# InfraKit.Aliyun (Alibaba Cloud)

[InfraKit](https://github.com/docker/infrakit) plugins for creating and managing resources in Aliyun (Alibaba Cloud).

## Instance plugin

An InfraKit instance plugin is provided, which creates Aliyun ECS instances.

### Building and running

To build the Aliyun Instance plugin, run `make binaries`.  The plugin binary will be located at
`./build/infrakit-instance-aliyun`.

At a minimum, the plugin requires the Aliyun region, access keys to use. 

```console
$ build/infrakit-instance-aliyun --region cn-beijing --access-key-id xxxxxxxx --access-key-secret xxxxxxxx
INFO[0000] Starting plugin
INFO[0000] Listening on: unix:///run/infrakit/plugins/instance-aliyun.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/instance-aliyun.sock err= <nil>
```

TODO: instance metadata support

### Example

To continue with an example, we will use the [default](https://github.com/docker/infrakit/tree/master/cmd/group) Group
plugin:
```console
$ build/infrakit-group-default
INFO[0000] Starting discovery
INFO[0000] Starting plugin
INFO[0000] Starting
INFO[0000] Listening on: unix:///run/infrakit/plugins/group.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/group.sock err= <nil>
```

and the [Vanilla](https://github.com/docker/infrakit/tree/master/example/flavor/vanilla) Flavor plugin:.
```console
$ build/infrakit-flavor-vanilla
INFO[0000] Starting plugin
INFO[0000] Listening on: unix:///run/infrakit/plugins/flavor-vanilla.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/flavor-vanilla.sock err= <nil>
```

We will use a basic configuration that creates a single instance:
```console
$ cat << EOF > aliyun-vanilla.json
{
  "ID": "aliyun-example",
  "Properties": {
    "Allocation": {
      "Size": 1
    },
    "Instance": {
      "Plugin": "instance-aliyun",
      "Properties": {
        "CreateInstanceArgs": {
          "ImageId": "ubuntu1404_64_40G_cloudinit_20160727.raw",
          "InstanceType": "ecs.t1.small",
          "Password": "Just4Test"
        },
         "Tags": {
          "Name": "infrakit-example"
        }
      }
    },
    "Flavor": {
      "Plugin": "flavor-vanilla",
      "Properties": {
        "Init": [
          "sh -c \"echo 'Hello, World' > /hello\""
        ]
      }
    }
  }
}
EOF
```

For the structure of `CreateInstanceArgs`, please refer to [the document of Aliyun SDK for Go](https://godoc.org/github.com/denverdino/aliyungo/ecs#CreateInstanceArgs).


Finally, instruct the Group plugin to start watching the group:
```console
$ build/infrakit group commit aliyun-vanilla.json
watching aliyun-example
```

In the console running the Group plugin, we will see input like the following:
```
INFO[1208] Watching group 'aliyun-example'
INFO[1219] Adding 1 instances to group to reach desired 1
INFO[1219] Created instance i-ba0412a2 with tags map[infrakit.config_sha:dUBtWGmkptbGg29ecBgv1VJYzys= infrakit.group:aliyun-example]
```

Additionally, the CLI will report the newly-created instance:
```console
$ build/infrakit group inspect aliyun-example
ID                             	LOGICAL                        	TAGS
i-2zeio83xn1d01wmhewjj        	10.170.238.146                	Name=infrakit-example,infrakit.config_sha=dUBtWGmkptbGg29ecBgv1VJYzys=,infrakit.group=aliyun-example
```

Retrieve the IP address of the host from the Aliyun console, and use SSH to verify that our shell code ran:

```console
$ ssh ubuntu@55.55.55.55 cat /hello
Hello, World!
```

### Plugin properties

The plugin expects properties in the following format:
```json
{
  "Tags": {
  },
  "RunInstancesInput": {
  }
}
```

The `Tags` property is a string-string mapping of ECS instance tags to include on all instances that are created.


#### Aliyun API Credentials

The plugin can use API credentials from several sources.
- config file: TODO
- EC2 instance metadata: TODO
Additional credentials sources are supported, but are not generally recommended as they are less secure:
- command line arguments: `--session-token`, or  `--access-key-id` and `--access-key-secret`
- environment variables: TODO


