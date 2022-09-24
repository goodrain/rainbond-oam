// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package export

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/goodrain/rainbond-oam/pkg/util"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type dockerComposeExporter struct {
	logger     *logrus.Logger
	ram        v1alpha1.RainbondApplicationConfig
	ctr        ContainerdAPI
	homePath   string
	exportPath string
}

func (d *dockerComposeExporter) Export() (*Result, error) {

	d.logger.Infof("start export app %s to docker compose app spec", d.ram.AppName)
	// Delete the old application group directory and then regenerate the application package
	if err := PrepareExportDir(d.exportPath); err != nil {
		d.logger.Errorf("prepare export dir failure %s", err.Error())
		return nil, err
	}

	d.logger.Infof("success prepare export dir")
	// Save components attachments
	if err := d.saveComponents(); err != nil {
		return nil, err
	}
	d.logger.Infof("success save components")
	// build docker-compose.yaml
	if err := d.buildDockerComposeYaml(); err != nil {
		return nil, err
	}
	d.logger.Infof("success build docker compose yaml spec")
	// build run.sh shell
	if err := d.buildStartScript(); err != nil {
		return nil, err
	}
	d.logger.Infof("success build start script")
	// packaging
	name, err := d.packaging()
	if err != nil {
		return nil, err
	}
	d.logger.Infof("success export app " + d.ram.AppName)
	return &Result{PackagePath: path.Join(d.homePath, name), PackageName: name}, nil
}

// saveComponents Bulk export of mirrored mode, lower disk footprint for the entire package
func (d *dockerComposeExporter) saveComponents() error {
	dockerCompose := newDockerCompose(d.ram)
	var componentImageNames []string
	for _, component := range d.ram.Components {
		componentName := component.ServiceCname
		componentEnName := dockerCompose.GetServiceName(component.ServiceShareID)
		serviceDir := fmt.Sprintf("%s/%s", d.exportPath, componentEnName)
		os.MkdirAll(serviceDir, 0755)
		volumes := component.ServiceVolumeMapList
		if volumes != nil && len(volumes) > 0 {
			for _, v := range volumes {
				if v.VolumeType == v1alpha1.ConfigFileVolumeType {
					err := exportComponentConfigFile(serviceDir, v)
					if err != nil {
						d.logger.Errorf("error exporting config file: %v", err)
						return err
					}
				}
			}
		}
		if component.ShareImage != "" {
			// app is image type
			localImageName, err := pullImage(d.ctr, component, d.logger)
			if err != nil {
				return err
			}
			d.logger.Infof("pull component %s image success", componentName)
			componentImageNames = append(componentImageNames, localImageName)
		}
	}
	start := time.Now()
	//ctx := context.Background()
	//err := docker.MultiImageSave(ctx, d.client, fmt.Sprintf("%s/component-images.tar", d.exportPath), componentImageNames...)
	w, err := os.Create(fmt.Sprintf("%s/component-images.tar", d.exportPath))
	if err != nil {
		logrus.Errorf("Failed to create file(%v) : %s", componentImageNames, err)
		return err
	}
	defer w.Close()
	err = saveImage(d.ctr, w, componentImageNames)
	if err != nil {
		logrus.Errorf("Failed to save image(%v) : %s", componentImageNames, err)
		return err
	}
	d.logger.Infof("save component images success, Take %s time", time.Now().Sub(start))
	return nil
}

func (d *dockerComposeExporter) buildDockerComposeYaml() error {
	y := &DockerComposeYaml{
		Version:  "2.1",
		Volumes:  make(map[string]GlobalVolume, 5),
		Services: make(map[string]*Service, 5),
	}
	dockerCompose := newDockerCompose(d.ram)

	for _, app := range d.ram.Components {
		shareImage := app.ShareImage
		shareUUID := app.ServiceShareID
		volumes := dockerCompose.GetServiceVolumes(shareUUID)
		appName := dockerCompose.GetServiceName(shareUUID)

		// environment variables
		envs := make(map[string]string, 10)
		if len(app.Ports) > 0 {
			// The first port here maybe not as the same as the first one original
			port := app.Ports[0]
			envs["PORT"] = fmt.Sprintf("%d", port.ContainerPort)
		}
		envs["MEMORY_SIZE"] = GetMemoryType(app.ExtendMethodRule.InitMemory)
		for _, item := range append(app.Envs, app.ServiceConnectInfoMapList...) {
			envs[item.AttrName] = item.AttrValue
			if item.AttrValue == "**None**" {
				envs[item.AttrName] = util.NewUUID()[:8]
			}
		}
		var depServices []string
		for _, item := range app.DepServiceMapList {
			serviceKey := item.DepServiceKey
			depEnvs := getPublicEnvByKey(serviceKey, d.ram.Components)
			for k, v := range depEnvs {
				if v == "**None**" {
					v = util.NewUUID()[:8]
				}
				envs[k] = v
			}
			for _, app := range d.ram.Components {
				if serviceKey == app.ComponentKey || serviceKey == app.ServiceShareID {
					depServices = append(depServices, dockerCompose.GetServiceName(app.ServiceShareID))
				}
			}
		}

		for key, value := range envs {
			// env rendering
			envs[key] = util.ParseVariable(value, envs)
		}

		service := &Service{
			Image:         shareImage,
			ContainerName: appName,
			Restart:       "always",
			NetworkMode:   "host",
			Volumes:       volumes,
			Command:       app.Cmd,
			Environment:   envs,
		}
		service.Loggin.Driver = "json-file"
		service.Loggin.Options.MaxSize = "5m"
		service.Loggin.Options.MaxFile = "2"
		if len(depServices) > 0 {
			service.DependsOn = depServices
		}

		y.Services[appName] = service
	}

	y.Volumes = dockerCompose.GetGlobalVolumes()
	content, err := yaml.Marshal(y)
	if err != nil {
		d.logger.Error("Failed to build yaml file: ", err)
		return err
	}

	err = ioutil.WriteFile(fmt.Sprintf("%s/docker-compose.yaml", d.exportPath), content, 0644)
	if err != nil {
		d.logger.Error("Failed to create yaml file: ", err)
		return err
	}
	return nil
}

func (d *dockerComposeExporter) buildStartScript() error {
	if err := ioutil.WriteFile(path.Join(d.exportPath, "run.sh"), []byte(runScritShell), 0755); err != nil {
		d.logger.Errorf("write run shell script failure %s", err.Error())
		return err
	}
	return nil
}

func (d *dockerComposeExporter) packaging() (string, error) {
	packageName := fmt.Sprintf("%s-%s-dockercompose.tar.gz", d.ram.AppName, d.ram.AppVersion)

	cmd := exec.Command("tar", "-czf", path.Join(d.homePath, packageName), path.Base(d.exportPath))
	cmd.Dir = d.homePath
	if err := cmd.Run(); err != nil {
		err = fmt.Errorf("Failed to package app %s: %s ", packageName, err.Error())
		d.logger.Error(err)
		return "", err
	}
	return packageName, nil
}

//DockerComposeYaml -
type DockerComposeYaml struct {
	Version  string                  `yaml:"version"`
	Volumes  map[string]GlobalVolume `yaml:"volumes,omitempty"`
	Services map[string]*Service     `yaml:"services,omitempty"`
}

//Service service
type Service struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name,omitempty"`
	Restart       string            `yaml:"restart,omitempty"`
	NetworkMode   string            `yaml:"network_mode,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	Command       string            `yaml:"command,omitempty"`
	Environment   map[string]string `yaml:"environment,omitempty"`
	DependsOn     []string          `yaml:"depends_on,omitempty"`
	Loggin        struct {
		Driver  string `yaml:"driver,omitempty"`
		Options struct {
			MaxSize string `yaml:"max-size,omitempty"`
			MaxFile string `yaml:"max-file,omitempty"`
		}
	} `yaml:"logging,omitempty"`
}

//GlobalVolume -
type GlobalVolume struct {
	External bool `yaml:"external"`
}

type dockerCompose struct {
	ram            v1alpha1.RainbondApplicationConfig
	globalVolumes  []string
	serviceVolumes map[string][]string
	serviceNames   map[string]string
}

func newDockerCompose(ram v1alpha1.RainbondApplicationConfig) *dockerCompose {
	dc := &dockerCompose{
		ram: ram,
	}
	dc.build()
	return dc
}

func (d *dockerCompose) build() {
	// Important! serviceNames is always first
	d.serviceNames = d.buildServiceNames()
	d.serviceVolumes, d.globalVolumes = d.buildVolumes()
}

func (d *dockerCompose) buildServiceNames() map[string]string {
	names := make(map[string]string)
	set := make(map[string]struct{})
	for _, cpt := range d.ram.Components {
		name := composeName(cpt.ServiceCname)
		// make sure every name is unique
		if _, exists := set[name]; exists {
			name += "-" + util.NewUUID()[0:4]
		}
		set[name] = struct{}{}
		names[cpt.ServiceShareID] = name
	}
	return names
}

// build service volumes and global volumes
func (d *dockerCompose) buildVolumes() (map[string][]string, []string) {
	logrus.Debugf("start building volumes for %s", d.ram.AppName)

	var volumeMaps = make(map[string]string)
	var volumeList []string
	componentVolumes := make(map[string][]string)
	for _, cpt := range d.ram.Components {
		serviceName := d.GetServiceName(cpt.ServiceShareID)

		var volumes []string
		// own volumes
		for _, vol := range cpt.ServiceVolumeMapList {
			svolume, composeVolume, isConfig := d.buildVolume(serviceName, &vol)
			volumes = append(volumes, svolume)
			if composeVolume != "" {
				if !isConfig {
					volumeList = append(volumeList, composeVolume)
				}
				volumeMaps[cpt.ServiceShareID+vol.VolumeName] = composeVolume
			}
		}
		componentVolumes[cpt.ServiceShareID] = volumes
	}
	for _, cpt := range d.ram.Components {
		// dependent volumes
		for _, dvol := range cpt.MntReleationList {
			vol := volumeMaps[dvol.ShareServiceUUID+dvol.VolumeName]
			if vol == "" {
				logrus.Warningf("[dockerCompose] [buildVolumes] dependent volume(%s/%s) not found", dvol.ShareServiceUUID, dvol.VolumeName)
				continue
			}
			componentVolumes[cpt.ServiceShareID] = append(componentVolumes[cpt.ServiceShareID], fmt.Sprintf("%s:%s", vol, dvol.VolumeMountDir))
		}
	}
	return componentVolumes, volumeList
}

func (d *dockerCompose) buildVolume(serviceName string, volume *v1alpha1.ComponentVolume) (string, string, bool) {
	volumePath := volume.VolumeMountPath
	if volume.VolumeType == "config-file" {
		configFilePath := "./" + path.Join(serviceName, volume.VolumeMountPath)
		return fmt.Sprintf("%s:%s", configFilePath, volumePath), configFilePath, true
	}
	// make sure every volumeName is unique
	volumeName := serviceName + "_" + volume.VolumeName
	return fmt.Sprintf("%s:%s", volumeName, volumePath), volumeName, false
}

// GetServiceVolumes -
func (d *dockerCompose) GetServiceVolumes(shareServiceUUID string) []string {
	return d.serviceVolumes[shareServiceUUID]
}

// GetGlobalVolumes -
func (d *dockerCompose) GetGlobalVolumes() map[string]GlobalVolume {
	globalVolumes := make(map[string]GlobalVolume)
	for _, vol := range d.globalVolumes {
		globalVolumes[vol] = GlobalVolume{
			External: false,
		}
	}
	return globalVolumes
}

// GetServiceName -
func (d *dockerCompose) GetServiceName(shareServiceUUID string) string {
	return d.serviceNames[shareServiceUUID]
}

func findDepVolume(allVolumes map[string]v1alpha1.ComponentVolumeList, key, volumeName string) *v1alpha1.ComponentVolume {
	vols := allVolumes[key]
	// find related volume
	var volume *v1alpha1.ComponentVolume
	for _, vol := range vols {
		if vol.VolumeName == volumeName {
			volume = &vol
			break
		}
	}
	return volume
}

func getPublicEnvByKey(serviceKey string, apps []*v1alpha1.Component) map[string]string {
	envs := make(map[string]string, 5)
	for _, app := range apps {
		if app.ComponentKey == serviceKey || app.ServiceShareID == serviceKey {
			for _, item := range app.ServiceConnectInfoMapList {
				envs[item.AttrName] = item.AttrValue
			}
			break
		}
	}
	return envs
}

var runScritShell = `#!/bin/bash
cd $(dirname $0)
cmd="$1"
[[ x$cmd == x ]] && cmd=start

eprint() {
  echo -e "\033[0;37;41m $* \033[0m"
}

iprint() {
  echo -e "\033[0;37;42m $* \033[0m"
}

check::dependency() {
  which docker &>/dev/null || {
    eprint 'Not found docker command!'

    install::docker || {
      eprint 'Failed to install docker!'
      return 11
    }

    iprint 'successful install docker!'
  }

  which docker-compose &>/dev/null || {
    eprint 'Not found docker-compose command!'

    install::docker-compose || {
      eprint 'Failed to install docker-compose!'
      return 13
    }

    iprint 'successful install docker-compose!'
  }

  return 0
}

install::docker() {
  curl -fsSL https://get.docker.com -o get-docker.sh &&
    sh get-docker.sh &&
    which docker &>/dev/null &&
    systemctl start docker &&
    systemctl enable docker
}

install::docker-compose() {
  curl -L "https://github.com/docker/compose/releases/download/1.24.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
  chmod +x /usr/local/bin/docker-compose
  which docker-compose &>/dev/null
}

import::image() {
  docker load -i component-images.tar
}

start() {
  import::image
  docker-compose -f docker-compose.yaml up -d
}

stop() {
  docker-compose -f docker-compose.yaml down
}

main() {
  check::dependency || exit $?

  eval "$cmd"
}

main
`
