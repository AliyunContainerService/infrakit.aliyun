package instance

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	log "github.com/Sirupsen/logrus"

	"github.com/denverdino/aliyungo/common"
	"github.com/denverdino/aliyungo/ecs"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/machine/libmachine/ssh"
)

const (
	// VolumeTag is the aliyun tag name used to associate unique identifiers (instance.VolumeID) with volumes.
	VolumeTag = "docker-infrakit-volume"
)

// Provisioner is an instance provisioner for Aliyun.
type Provisioner struct {
	Client        *ecs.Client
	Region        common.Region
	PrivateIPOnly bool
}

type properties struct {
	Region   string
	Retries  int
	Instance *types.Any
}

// NewInstancePlugin creates a new plugin that creates instances in Aliyun ECS.
func NewInstancePlugin(b *Builder) instance.Plugin {
	client := ecs.NewClient(b.accessKeyID, b.secretAccessKey)
	client.SetDebug(true)
	return &Provisioner{
		Client:        client,
		Region:        common.Region(b.Region),
		PrivateIPOnly: b.PrivateIPOnly}
}

func (p Provisioner) tagInstance(
	instanceID string,
	systemTags map[string]string,
	userTags map[string]string) error {

	tags := make(map[string]string)

	// Gather the tag keys in sorted order, to provide predictable tag order.  This is
	// particularly useful for tests.
	for k, v := range userTags {
		tags[k] = v
	}

	// System tags overwrite user tags.
	for k, v := range systemTags {
		tags[k] = v
	}

	addTagArgs := ecs.AddTagsArgs{
		RegionId:     p.Region,
		ResourceId:   instanceID,
		ResourceType: ecs.TagResourceInstance,
		Tag:          tags,
	}

	err := p.Client.AddTags(&addTagArgs)
	return err
}

// VendorInfo returns a vendor specific name and version
func (p Provisioner) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-aliyun",
			Version: "0.3.0",
		},
		URL: "https://github.com/AliyunContainerService/infrakit.aliyun",
	}
}

// Validate performs local checks to determine if the request is valid.
func (p Provisioner) Validate(req *types.Any) error {
	// TODO(wfarner): Implement
	return nil
}

type createInstanceRequest struct {
	CreateInstanceArgs ecs.CreateInstanceArgs
	Tags               map[string]string
}

const (
	timeout              = 300
	defaultUbuntuImageID = "ubuntu_160401_64_40G_cloudinit_20161115.vhd"
	internetChargeType   = "PayByTraffic"
)

func isUserDataSupported(args *ecs.CreateInstanceArgs) bool {
	return args.VSwitchId != ""
}

// Provision creates a new instance.
func (p Provisioner) Provision(spec instance.Spec) (*instance.ID, error) {

	if spec.Properties == nil {
		return nil, errors.New("Properties must be set")
	}

	request := createInstanceRequest{}
	err := json.Unmarshal(*spec.Properties, &request)
	if err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	request.CreateInstanceArgs.RegionId = p.Region

	volumeIDs := []string{}
	if spec.Attachments != nil && len(spec.Attachments) > 0 {

		volumeTags := make(map[string]string)
		volumes := []ecs.DiskItemType{}
		for _, attachment := range spec.Attachments {
			s := string(attachment.ID)
			volumeTags[VolumeTag] = s
			portableFlag := true
			result, _, err := p.Client.DescribeDisks(&ecs.DescribeDisksArgs{
				RegionId: p.Region,
				ZoneId:   request.CreateInstanceArgs.ZoneId,
				Portable: &portableFlag,
				Tag:      volumeTags,
			})
			if err != nil {
				return nil, errors.New("Failed while looking up volume")
			}
			volumes = append(volumes, result...)
		}

		if len(volumes) == len(spec.Attachments) {
			for _, volume := range volumes {
				volumeIDs = append(volumeIDs, volume.DiskId)
			}
		} else {
			return nil, fmt.Errorf(
				"Not all required volumes found to attach.  Wanted %v, found %v",
				spec.Attachments,
				volumes)
		}
	}

	// Set the default parameters
	if request.CreateInstanceArgs.ImageId == "" {
		request.CreateInstanceArgs.ImageId = defaultUbuntuImageID
	}
	request.CreateInstanceArgs.ClientToken = p.Client.GenerateClientToken()

	// Set the default internet charge type for classic network
	if request.CreateInstanceArgs.InternetChargeType == "" && request.CreateInstanceArgs.VSwitchId == "" && !p.PrivateIPOnly {
		request.CreateInstanceArgs.InternetChargeType = internetChargeType
		request.CreateInstanceArgs.InternetMaxBandwidthOut = 1
	}

	supportsUserData := isUserDataSupported(&request.CreateInstanceArgs)

	if spec.Init != "" && supportsUserData {
		request.CreateInstanceArgs.UserData = spec.Init
	}

	//TODO handle the cloud-init
	instanceID, err := p.Client.CreateInstance(&request.CreateInstanceArgs)
	if err != nil {
		return nil, err
	}

	id := instance.ID(instanceID)

	err = p.Client.WaitForInstance(instanceID, ecs.Stopped, timeout)
	if err != nil {
		return &id, err
	}

	if !p.PrivateIPOnly {
		if request.CreateInstanceArgs.VSwitchId == "" {
			// Allocate public IP address for classic network
			var ipAddress string
			ipAddress, err = p.Client.AllocatePublicIpAddress(instanceID)
			if err != nil {
				log.Warnf("Failed to allocate public IP address for instance %s: %v\n", instanceID, err)
			} else {
				log.Infof("Allocate publice IP address %s for instance %s successfully\n", ipAddress, instanceID)
			}
		}
	}

	err = p.tagInstance(instanceID, spec.Tags, request.Tags)
	if err != nil {
		return &id, err
	}

	for _, volumeID := range volumeIDs {
		err := p.Client.AttachDisk(&ecs.AttachDiskArgs{
			InstanceId: instanceID,
			DiskId:     volumeID,
			//Device:     "/dev/sdf", //TODO how to specify the device
		})
		if err != nil {
			return &id, err
		}
	}
	log.Infof("Start ECS instance %s ...\n", instanceID)
	p.Client.StartInstance(instanceID)
	if err != nil {
		log.Warnf("Failed to start ECS instance %s\n", instanceID)
		return nil, err
	}
	err = p.Client.WaitForInstance(instanceID, ecs.Running, timeout)
	if err == nil {
		// Using SSH to config the ECS instance if User Data is not supported
		if spec.Init != "" && !supportsUserData {
			instance, err := p.describeInstance(id)
			if err == nil {
				ipAddr := p.getInstanceIP(instance)
				var sshClient ssh.Client
				log.Infof("Config ECS %s with SSH ...", ipAddr)
				sshClient, err = getSSHClient(ipAddr, 22, "root", request.CreateInstanceArgs.Password)
				if err != nil {
					log.Warnf("Failed to get SSHClient: %v\n", err)
				} else {
					var output string
					output, err = sshClient.Output(spec.Init)
					if err != nil {
						log.Warnf("Failed to execute command '%s' with SSH: %v", spec.Init, err)
					} else {
						log.Infof("Execute command successfully with SSH: %s", output)
					}
				}
			}
			return &id, err
		}
	}

	return &id, err
}

func (p Provisioner) getInstanceIP(inst *ecs.InstanceAttributesType) string {
	if p.PrivateIPOnly {
		if inst.InnerIpAddress.IpAddress != nil && len(inst.InnerIpAddress.IpAddress) > 0 {
			return inst.InnerIpAddress.IpAddress[0]
		}

		if inst.VpcAttributes.PrivateIpAddress.IpAddress != nil && len(inst.VpcAttributes.PrivateIpAddress.IpAddress) > 0 {
			return inst.VpcAttributes.PrivateIpAddress.IpAddress[0]
		}
		return ""
	}
	if inst.PublicIpAddress.IpAddress != nil && len(inst.PublicIpAddress.IpAddress) > 0 {
		return inst.PublicIpAddress.IpAddress[0]
	}
	if len(inst.EipAddress.IpAddress) > 0 {
		return inst.EipAddress.IpAddress
	}
	return ""
}

// Destroy terminates an existing instance.
func (p Provisioner) Destroy(id instance.ID) error {
	instanceID := string(id)
	log.Infof("Stopping instance %s ...", instanceID)
	err := p.Client.StopInstance(instanceID, false)
	if err != nil {
		log.Warnf("Failed to stop instance %s: %v", instanceID, err)
	} else {
		// Wait for stopped
		err := p.Client.WaitForInstance(instanceID, ecs.Stopped, timeout)
		if err != nil {
			log.Warnf("Failed to wait instance %s stopped: %v", instanceID, err)
		}
	}
	log.Infof("Deleting instance %s ...", instanceID)
	if err := p.Client.DeleteInstance(instanceID); err != nil {
		return fmt.Errorf("Failed to delete instance %s: %v", instanceID, err)
	}
	return err
}

func (p Provisioner) describeGroupRequest(tags map[string]string, pagination *common.Pagination) *ecs.DescribeInstancesArgs {
	if pagination == nil {
		pagination = &common.Pagination{}
	}

	return &ecs.DescribeInstancesArgs{
		RegionId:   p.Region,
		Tag:        tags,
		Pagination: *pagination,
	}
}

func (p Provisioner) describeInstances(tags map[string]string, pagination *common.Pagination) ([]instance.Description, error) {
	instances, paginationResult, err := p.Client.DescribeInstances(p.describeGroupRequest(tags, pagination))
	if err != nil {
		return nil, err
	}

	descriptions := []instance.Description{}
	for _, item := range instances {
		instanceTags := make(map[string]string)
		for _, tagItem := range item.Tags.Tag {
			instanceTags[tagItem.TagKey] = tagItem.TagValue
		}
		ipAddr := p.getInstanceIP(&item)
		descriptions = append(descriptions, instance.Description{
			ID:        instance.ID(item.InstanceId),
			LogicalID: (*instance.LogicalID)(&ipAddr),
			Tags:      instanceTags,
		})
	}

	if paginationResult != nil {
		pagination = paginationResult.NextPage()
		if pagination != nil {
			// There are more pages of results.
			remainingPages, err := p.describeInstances(tags, pagination)
			if err != nil {
				return nil, err
			}

			descriptions = append(descriptions, remainingPages...)
		}
	}

	return descriptions, nil
}

// DescribeInstances implements instance.Provisioner.DescribeInstances.
func (p Provisioner) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	return p.describeInstances(tags, nil)
}

func (p Provisioner) describeInstance(id instance.ID) (*ecs.InstanceAttributesType, error) {
	result, err := p.Client.DescribeInstanceAttribute(string(id))
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Label implements labeling the instances.
func (p Provisioner) Label(id instance.ID, labels map[string]string) error {

	output, err := p.describeInstance(id)
	if err != nil {
		return err
	}

	allTags := map[string]string{}
	for _, tagItem := range output.Tags.Tag {
		allTags[tagItem.TagKey] = tagItem.TagValue
	}

	_, merged := mergeTags(allTags, labels)

	args := ecs.AddTagsArgs{
		ResourceId:   string(id),
		ResourceType: ecs.TagResourceInstance,
		RegionId:     output.RegionId,
		Tag:          merged,
	}

	return p.Client.AddTags(&args)
}

// mergeTags merges multiple maps of tags, implementing 'last write wins' for colliding keys.
//
// Returns a sorted slice of all keys, and the map of merged tags.  Sorted keys are particularly useful to assist in
// preparing predictable output such as for tests.
func mergeTags(tagMaps ...map[string]string) ([]string, map[string]string) {

	keys := []string{}
	tags := map[string]string{}

	for _, tagMap := range tagMaps {
		for k, v := range tagMap {
			if _, exists := tags[k]; exists {
				log.Debugf("Ovewriting tag value for key %s", k)
			} else {
				keys = append(keys, k)
			}
			tags[k] = v
		}
	}

	sort.Strings(keys)

	return keys, tags
}
