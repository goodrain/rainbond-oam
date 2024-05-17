// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltr.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltr.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package export

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond-oam/pkg/cpk"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/goodrain/rainbond-oam/pkg/util/image"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path"
	"strings"
)

func init() {
	AddCreateFileDir(writeApplicationYml)
	AddCreateFileDir(writeFilesDir)
	AddCreateFileDir(writeIconsDir)
	AddCreateFileDir(writeScreeenshotsDir)
	AddCreateFileDir(packageJson)
}

type cpkExporter struct {
	logger      *logrus.Logger
	ram         v1alpha1.RainbondApplicationConfig
	imageClient image.Client
	mode        string
	homePath    string
	exportPath  string
}

func (r *cpkExporter) Export() (*Result, error) {
	r.logger.Infof("start export app %s to ram app spec", r.ram.AppName)
	// Delete the old application group directory and then regenerate the application package
	if err := PrepareExportDir(r.exportPath); err != nil {
		r.logger.Errorf("prepare export dir failure %s", err.Error())
		return nil, err
	}
	r.logger.Infof("success prepare export dir")
	packName := "null-app"
	if len(r.ram.Components) > 0 {
		component := r.ram.Components[0]
		dirName := fmt.Sprintf("cpk.rbd.%v_v%v_%v", component.K8SComponentName, r.ram.AppVersion, component.Arch)
		packName = fmt.Sprintf("%v.cpk", dirName)
		component.DeployVersion = r.ram.AppVersion
		for _, create := range createList {
			if err := create(r.exportPath, component, r.imageClient); err != nil {
				logrus.Errorf("create fun failure: %v", err)
				return nil, err
			}
		}
	}
	r.logger.Infof("success write ram spec file")
	name, err := PackagingBZip2(packName, r.homePath, r.exportPath)
	if err != nil {
		err = fmt.Errorf("Failed to package app %s: %s ", packName, err.Error())
		r.logger.Error(err)
		return nil, err
	}
	r.logger.Infof("success export app " + r.ram.AppName)
	return &Result{PackagePath: path.Join(r.homePath, name), PackageName: name}, nil
}

func (r *cpkExporter) writeMetaFile() error {
	// remove component and plugin image hub info
	if r.mode == "offline" {
		for i := range r.ram.Components {
			r.ram.Components[i].AppImage = v1alpha1.ImageInfo{}
		}
		for i := range r.ram.Plugins {
			r.ram.Plugins[i].PluginImage = v1alpha1.ImageInfo{}
		}
	}
	meta, err := json.MarshalIndent(r.ram, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal ram meta config failure %s", err.Error())
	}
	if err := ioutil.WriteFile(path.Join(r.exportPath, "metadata.json"), meta, 0755); err != nil {
		return fmt.Errorf("write ram app meta config file failure %s", err.Error())
	}
	return nil
}

var createList []func(p string, r *v1alpha1.Component, imageClient image.Client) error

func AddCreateFileDir(fun func(p string, r *v1alpha1.Component, imageClient image.Client) error) {
	createList = append(createList, fun)
}

func writeApplicationYml(exportPath string, r *v1alpha1.Component, imageClient image.Client) error {
	buf := []byte("application.yml")
	file, err := os.Create(path.Join(exportPath, "application.yml"))
	if err != nil {
		return fmt.Errorf("create cpk application yaml failure: %s", err.Error())
	}
	defer file.Close()
	_, err = io.WriteString(file, string(buf))
	if err != nil {
		return fmt.Errorf("write cpk application yaml failure: %s", err.Error())
	}
	return nil
}

const fileListTemplate = `F,/image.json,%d,0666,%s
F,/image/%s,%d,0666,%s
D,/image,%d,0777`

func writeFilesDir(exportPath string, r *v1alpha1.Component, imageClient image.Client) error {
	err := os.MkdirAll(path.Join(exportPath, "files/image"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("create cpk files image dir failure: %s", err.Error())
	}
	ijData, err := getFileImageJsonData(r)
	if err != nil {
		return err
	}
	buf := []byte(ijData)
	jsonFile, err := os.Create(path.Join(exportPath, "files", "image.json"))
	if err != nil {
		return fmt.Errorf("create cpk files image json failure: %s", err.Error())
	}
	defer jsonFile.Close()
	_, err = io.WriteString(jsonFile, string(buf))
	if err != nil {
		return fmt.Errorf("write cpk files image json failure: %s", err.Error())
	}

	imageJsonFileSize, err := getDirORFileSize(jsonFile)
	if err != nil {
		return err
	}
	imageJsonFileSha, err := getFileSha1(jsonFile)
	if err != nil {
		return err
	}

	_, err = imageClient.ImagePull(r.ShareImage, r.AppImage.HubUser, r.AppImage.HubPassword, 30)
	if err != nil {
		return err
	}
	componentImageNames := []string{r.ShareImage}
	imageTarName := fmt.Sprintf("/cpk.rbd.%v_%v.tar", r.K8SComponentName, r.DeployVersion)

	err = imageClient.ImageSave(path.Join(exportPath, "files/image", imageTarName), componentImageNames)
	if err != nil {
		return err
	}

	imageTarSize := int64(0)
	imageTarSha := "0"
	imageDirSize := 0

	data := fmt.Sprintf(fileListTemplate, imageJsonFileSize, imageJsonFileSha, imageTarName, imageTarSize, imageTarSha, imageDirSize)
	buf = []byte(data)
	listfile, err := os.Create(path.Join(exportPath, "filelist"))
	if err != nil {
		return fmt.Errorf("create cpk filelist failure: %s", err.Error())
	}
	defer listfile.Close()
	_, err = io.WriteString(listfile, string(buf))
	if err != nil {
		return fmt.Errorf("write cpk filelist failure: %s", err.Error())
	}
	return nil
}

func writeIconsDir(exportPath string, r *v1alpha1.Component, imageClient image.Client) error {
	err := os.MkdirAll(path.Join(exportPath, "icons"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("create cpk icons dir failure: %s", err.Error())
	}
	err = CopyFile("/run/rainbond.png", path.Join(exportPath, "icons", fmt.Sprintf("cpk.rbd.%v.png", r.K8SComponentName)))
	if err != nil {
		return fmt.Errorf("copy rainbond.png failure: %s", err.Error())
	}
	return nil
}

func packageJson(exportPath string, r *v1alpha1.Component, imageClient image.Client) error {
	pjStruct := cpk.PackageJSONCPK{
		Architecture:   r.Arch,
		Browser:        cpk.Browser{},
		Category:       "application",
		Classification: "L0",
		Count:          5,
		Description:    "由 Rainbond 好雨云平台导出",
		Genericname:    r.K8SComponentName,
		Glibc:          "",
		Id:             fmt.Sprintf("cpk.rbd.%v", r.K8SComponentName),
		Name:           r.K8SComponentName,
		News:           "由 Rainbond 好雨云平台导出",
		Os:             "all",
		Permission:     cpk.Permission{},
		Runtime:        "",
		Scripts:        cpk.Scripts{},
		Search:         "",
		//需要可配置，前期不需要
		Secret: "",
		//最后获取
		Size: "",
		//
		Start:   "/",
		Summary: "由 Rainbond 好雨云平台导出",
		Todo:    "",
		Type:    "web",
		Vendor: cpk.Vendor{
			Description: "",
			Email:       "",
			Homepage:    "rainbond.com",
			Name:        "rbd",
			Telephone:   "",
		},
		Version: r.DeployVersion,
		Web:     cpk.Web{},
	}

	pJsonBuf, err := json.MarshalIndent(pjStruct, "", "    ")
	file, err := os.Create(path.Join(exportPath, "package.json"))
	if err != nil {
		return fmt.Errorf("create cpk package json failure: %s", err.Error())
	}
	defer file.Close()
	_, err = io.WriteString(file, string(pJsonBuf))
	if err != nil {
		return fmt.Errorf("write cpk package json failure: %s", err.Error())
	}
	return nil
}

func writeScreeenshotsDir(exportPath string, r *v1alpha1.Component, imageClient image.Client) error {
	err := os.MkdirAll(path.Join(exportPath, "screenshots"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("create cpk screenshots dir failure: %s", err.Error())
	}
	err = CopyFile("/run/jt.png", path.Join(exportPath, "screenshots", fmt.Sprintf("cpk.rbd.%v_1.png", r.K8SComponentName)))
	if err != nil {
		return fmt.Errorf("copy jt.png failure: %s", err.Error())
	}
	return nil
}

func getFileSha1(file *os.File) (string, error) {
	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("Error copying file to hash:", err)
	}
	hashInBytes := hash.Sum(nil)
	// 将哈希值转换为16进制字符串，得到的就是完整的40位十六进制字符串
	FileSha := fmt.Sprintf("%x", hashInBytes)
	return FileSha, nil
}

func getDirORFileSize(file *os.File) (int64, error) {
	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("Error getting file info:", err)
	}
	// 文件大小以字节为单位
	return info.Size(), nil
}

func getFileImageJsonData(component *v1alpha1.Component) (string, error) {
	var apps []cpk.Apps
	//处理 cpu
	cpu := float64(1)
	if component.CPU != 0 {
		floatCPU := float64(component.CPU) / 1000
		cpu = math.Ceil(floatCPU)
	}
	//处理存储
	var volumes []cpk.Volume
	for _, vo := range component.ServiceVolumeMapList {
		volumes = append(volumes, cpk.Volume{
			ContainerPath: vo.VolumeMountPath,
			HostPath:      path.Join(component.K8SComponentName, vo.VolumeMountPath),
			Mode:          "RW",
		})
	}
	//处理port
	var ports []cpk.PortMapping
	for _, port := range component.Ports {
		if port.Protocol != "tcp" && port.Protocol != "udp" {
			port.Protocol = "tcp"
		}
		ports = append(ports, cpk.PortMapping{
			ContainerPort: port.ContainerPort,
			HostPort:      0,
			Labels:        map[string]string{},
			Name:          fmt.Sprintf("app_%v", port.ContainerPort),
			Protocol:      port.Protocol,
			ServicePort:   0,
		})
	}
	if len(ports) == 0 {
		ports = append(ports, cpk.PortMapping{
			ContainerPort: 80,
			HostPort:      0,
			Labels:        map[string]string{},
			Name:          "app_80",
			Protocol:      "tcp",
			ServicePort:   0,
		})
	}
	//处理环境变量
	parameters := make([]cpk.Parameter, 10)
	for _, env := range component.Envs {
		parameters = append(parameters, cpk.Parameter{
			Key:   env.AttrName,
			Value: env.AttrValue,
		})
	}
	//处理docker字段
	doc := cpk.Docker{
		ForcePullImage: false,
		Image:          component.ShareImage,
		Network:        "BRIDGE",
		Parameters:     parameters,
		PortMappings:   ports,
		Privileged:     false,
	}
	//处理健康检测
	var healthChecks []cpk.HealthChecks
	for _, hc := range component.Probes {
		pa := hc.Path
		hc.Scheme = strings.ToUpper(hc.Scheme)
		if hc.Scheme == "CMD" {
			pa = hc.Cmd
			hc.Scheme = "COMMAND"
		}
		healthChecks = append(healthChecks, cpk.HealthChecks{
			GracePeriodSeconds:     hc.InitialDelaySecond,
			IgnoreHttp1Xx:          false,
			IntervalSeconds:        hc.PeriodSecond,
			MaxConsecutiveFailures: hc.FailureThreshold,
			Path:                   pa,
			PortIndex:              hc.Port,
			Protocol:               hc.Scheme,
			TimeoutSeconds:         hc.TimeoutSecond,
		})
	}
	if len(healthChecks) == 0 {
		hcPort := ports[0]
		healthChecks = append(healthChecks, cpk.HealthChecks{
			GracePeriodSeconds:     300,
			IgnoreHttp1Xx:          false,
			IntervalSeconds:        60,
			MaxConsecutiveFailures: 3,
			Path:                   "",
			PortIndex:              hcPort.ContainerPort,
			Protocol:               "TCP",
			TimeoutSeconds:         20,
		})
	}
	cpkID := fmt.Sprintf("/cpk.rbd.%v-%v", component.K8SComponentName, component.DeployVersion)
	appID := cpkID + cpkID
	if component.Cmd == "start web" {
		component.Cmd = ""
	}
	apps = append(apps, cpk.Apps{
		CMD:         component.Cmd,
		Constraints: [][]string{},
		Container: cpk.Container{
			Docker:  doc,
			Type:    "DOCKER",
			Volumes: volumes,
		},
		Cpus:         cpu,
		Dependencies: []string{},
		Disk:         0,
		HealthChecks: healthChecks,
		ID:           appID,
		Instances:    component.ExtendMethodRule.StepNode,
		Labels:       component.Labels,
		ENV:          map[string]string{},
		Mem:          component.Memory,
	})
	ijCPK := cpk.ImageJsonCPK{
		Apps: apps,
		Id:   cpkID,
	}
	ijCPKJson, err := json.MarshalIndent(ijCPK, "", "    ")
	return string(ijCPKJson), err
}

// CopyFile 复制源文件到目标文件
func CopyFile(src, dst string) error {
	// 打开源文件
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		// 无法复制非文件类型的对象
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	// 创建目标文件
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	// 使用io.Copy复制文件内容
	_, err = io.Copy(destination, source)
	return err
}
