package eci

import (
	"fmt"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/eci-20180808/v3/client"
)

//type Pod = client.CreateContainerGroupRequest

type Pod struct {
	ContainerGroupId *string
	*client.CreateContainerGroupRequest
}
type eci struct {
	regionId string
	endpoint string
	*client.Client
}

func NewEci(accessKeyId, accessKeySecret, regionId string) *eci {

	endpoint := "eci." + regionId + ".aliyuncs.com"

	cli, err := client.NewClient(&openapi.Config{AccessKeyId: &accessKeyId, AccessKeySecret: &accessKeySecret, Endpoint: &endpoint})

	if err != nil {
		fmt.Println(err)
		return nil
	}
	return &eci{
		regionId: regionId,
		endpoint: endpoint,
		Client:   cli,
	}

}

func (e *eci) ModifyPod(pod *Pod) {

	p := &client.UpdateContainerGroupRequest{}
	p.ContainerGroupId = pod.ContainerGroupId

	for _, c := range pod.Container {

		var EnvironmentVar []*client.UpdateContainerGroupRequestContainerEnvironmentVar

		for _, v := range c.EnvironmentVar {
			EnvironmentVar = append(EnvironmentVar, &client.UpdateContainerGroupRequestContainerEnvironmentVar{

				Key:   v.Key,
				Value: v.Value,
			})

		}
		var Port []*client.UpdateContainerGroupRequestContainerPort

		for _, v := range c.Port {
			Port = append(Port, &client.UpdateContainerGroupRequestContainerPort{Port: v.Port, Protocol: v.Protocol})

		}

		var VolumeMount []*client.UpdateContainerGroupRequestContainerVolumeMount

		for _, v := range c.VolumeMount {

			VolumeMount = append(VolumeMount, &client.UpdateContainerGroupRequestContainerVolumeMount{

				MountPath:        v.MountPath,
				Name:             v.Name,
				MountPropagation: v.MountPropagation,
				SubPath:          v.SubPath, // add SubPath *string `json:"SubPath,omitempty" xml:"SubPath,omitempty"`
			})
		}

		p.Container = append(p.Container, &client.UpdateContainerGroupRequestContainer{
			Arg:            c.Arg,
			Command:        c.Command,
			EnvironmentVar: EnvironmentVar,
			Name:           c.Name,
			Image:          c.Image,
			WorkingDir:     c.WorkingDir,
			Memory:         c.Memory,
			Cpu:            c.Cpu,
			Port:           Port,
			VolumeMount:    VolumeMount,
		})

	}
	for _, c := range pod.Volume {
		p.Volume = append(p.Volume, &client.UpdateContainerGroupRequestVolume{

			Name: c.Name,
			Type: c.Type,
			FlexVolume: &client.UpdateContainerGroupRequestVolumeFlexVolume{

				Driver:  c.FlexVolume.Driver,
				FsType:  c.FlexVolume.FsType,
				Options: c.FlexVolume.Options,
			},
		})

	}
	p.RegionId = pod.RegionId

	result, err := e.UpdateContainerGroup(p)
	fmt.Println(result, err)
}
func (e *eci) GetPod(name string) *Pod {

	result, err := e.DescribeContainerGroups(&client.DescribeContainerGroupsRequest{RegionId: &e.regionId, ContainerGroupName: &name})
	if err != nil {
		fmt.Println(err)
		return nil
	}

	if len(result.Body.ContainerGroups) != 1 {
		return nil
	}

	p := result.Body.ContainerGroups[0]
	//fmt.Println(p)

	pod := &Pod{
		CreateContainerGroupRequest: &client.CreateContainerGroupRequest{},

		//DescribeContainerGroupsResponseBodyContainerGroupsContainers
		//CreateContainerGroupRequestContainer

	}
	pod.ContainerGroupName = p.ContainerGroupName
	pod.ContainerGroupId = p.ContainerGroupId
	pod.Memory = p.Memory
	pod.Cpu = p.Cpu
	pod.RegionId = p.RegionId
	pod.ZoneId = p.ZoneId
	pod.VSwitchId = p.VSwitchId
	pod.SecurityGroupId = p.SecurityGroupId

	for _, c := range p.Containers {

		var EnvironmentVar []*client.CreateContainerGroupRequestContainerEnvironmentVar
		for _, v := range c.EnvironmentVars {

			EnvironmentVar = append(EnvironmentVar, &client.CreateContainerGroupRequestContainerEnvironmentVar{

				Key:      v.Key,
				Value:    v.Value,
				FieldRef: (*client.CreateContainerGroupRequestContainerEnvironmentVarFieldRef)(v.ValueFrom.FieldRef),
			})

		}

		var Port []*client.CreateContainerGroupRequestContainerPort
		for _, v := range c.Ports {
			Port = append(Port, &client.CreateContainerGroupRequestContainerPort{Port: v.Port, Protocol: v.Protocol})

		}

		var VolumeMount []*client.CreateContainerGroupRequestContainerVolumeMount

		for _, v := range c.VolumeMounts {

			VolumeMount = append(VolumeMount, &client.CreateContainerGroupRequestContainerVolumeMount{

				MountPath:        v.MountPath,
				Name:             v.Name,
				MountPropagation: v.MountPropagation,
				SubPath:          v.SubPath, // add SubPath *string `json:"SubPath,omitempty" xml:"SubPath,omitempty"`
			})
		}

		pod.Container = append(pod.Container, &client.CreateContainerGroupRequestContainer{
			Name:           c.Name,
			Image:          c.Image,
			Arg:            c.Args,
			Command:        c.Commands,
			Port:           Port,
			EnvironmentVar: EnvironmentVar,
			WorkingDir:     c.WorkingDir,
			VolumeMount:    VolumeMount,
			//LivenessProbe:  &client.CreateContainerGroupRequestContainerLivenessProbe{Exec: &client.CreateContainerGroupRequestContainerLivenessProbeExec{Command: c.LivenessProbe.Execs}},
			//ReadinessProbe: &client.CreateContainerGroupRequestContainerReadinessProbe{Exec: &client.CreateContainerGroupRequestContainerReadinessProbeExec{Command: c.ReadinessProbe.Execs}},
		})

		for _, c := range p.Volumes {

			pod.Volume = append(pod.Volume, &client.CreateContainerGroupRequestVolume{
				Name: c.Name,
				Type: c.Type,
				FlexVolume: &client.CreateContainerGroupRequestVolumeFlexVolume{
					Driver:  c.FlexVolumeDriver,
					FsType:  c.FlexVolumeFsType,
					Options: c.FlexVolumeOptions,
				},
			})
		}

	}

	return pod

}

func (p *Pod) findContainer(name string) *client.CreateContainerGroupRequestContainer {

	for _, c := range p.Container {
		if *c.Name == name {
			return c
		}
	}
	return nil

}

func (p *Pod) AddEnvironmentVar(ContainerName, k, v string) *Pod {

	p.findContainer(ContainerName).EnvironmentVar = append(p.findContainer(ContainerName).EnvironmentVar, &client.CreateContainerGroupRequestContainerEnvironmentVar{Key: &k, Value: &v})
	return p
}

func (p *Pod) SetImage(ContainerName, image string) {

	p.findContainer(ContainerName).Image = &image

}

func (p *Pod) SetPodName(name string) {
	p.ContainerGroupName = &name

}
func (e *eci) CreatePod(name, image string) (*string, error) {

	p := &Pod{
		CreateContainerGroupRequest: &client.CreateContainerGroupRequest{

			ContainerGroupName: &name,
		},
	}
	container := p.Container[0]
	container.Name = &name
	container.Image = &image

	result, err := e.CreateContainerGroup(p.CreateContainerGroupRequest)
	return result.Body.ContainerGroupId, err

}

func (e *eci) CreatePodFrom(pod *Pod) (*string, error) {

	result, err := e.CreateContainerGroup(pod.CreateContainerGroupRequest)
	return result.Body.ContainerGroupId, err

}
func (e *eci) DeletePodWithId(id string) error {

	_, err := e.DeleteContainerGroup(&client.DeleteContainerGroupRequest{ContainerGroupId: &id})
	return err

}
func (e *eci) DeletePod(name string) error {
	p := e.GetPod(name)
	if *p.ContainerGroupName != name {
		return fmt.Errorf("not find pod")
	}

	_, err := e.DeleteContainerGroup(&client.DeleteContainerGroupRequest{ContainerGroupId: p.ContainerGroupId, RegionId: p.RegionId})
	return err

}
