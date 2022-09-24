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
	"crypto/tls"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/mozillazg/go-pinyin"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// [a-zA-Z0-9._-]
func composeName(uText string) string {
	str := unicode2zh(uText)

	var res string
	for _, runeValue := range str {
		if unicode.Is(unicode.Han, runeValue) {
			// convert chinese to pinyin
			res += strings.Join(pinyin.LazyConvert(string(runeValue), nil), "")
			continue
		}
		matched, err := regexp.Match("[a-zA-Z0-9._-]", []byte{byte(runeValue)})
		if err != nil {
			logrus.Warningf("check if %s meets [a-zA-Z0-9._-]: %v", string(runeValue), err)
		}
		if !matched {
			res += "_"
			continue
		}
		res += string(runeValue)
	}
	logrus.Debugf("convert chinese %s to pinyin %s", str, res)
	return res
}

// unicode2zh 将unicode转为中文，并去掉空格
func unicode2zh(uText string) (context string) {
	for i, char := range strings.Split(uText, `\\u`) {
		if i < 1 {
			context = char
			continue
		}

		length := len(char)
		if length > 3 {
			pre := char[:4]
			zh, err := strconv.ParseInt(pre, 16, 32)
			if err != nil {
				context += char
				continue
			}

			context += fmt.Sprintf("%c", zh)

			if length > 4 {
				context += char[4:]
			}
		}

	}

	context = strings.TrimSpace(context)

	return context
}
func saveImage(ctr ContainerdAPI, w io.Writer, ImageNames []string) error {
	logrus.Infof("---------------------打包镜像----------------------")
	logrus.Infof("%v", ImageNames)
	var exportOpts []archive.ExportOpt
	for _, imageName := range ImageNames {
		exportOpts = append(exportOpts, archive.WithImage(ctr.ImageService, imageName))
	}
	err := ctr.ContainerdClient.Export(ctr.CCtx, w, exportOpts...)
	if err != nil {
		return err
	}
	return nil
}

func pullImage(ctr ContainerdAPI, component *v1alpha1.Component, log *logrus.Logger) (string, error) {
	// docker pull image-name
	//_, err := docker.ImagePull(client, component.ShareImage, component.AppImage.HubUser, component.AppImage.HubPassword, 30)
	defaultTLS := &tls.Config{
		InsecureSkipVerify: true,
	}

	hostOpt := config.HostOptions{}
	hostOpt.DefaultTLS = defaultTLS
	hostOpt.Credentials = func(host string) (string, string, error) {
		return component.AppImage.HubUser, component.AppImage.HubPassword, nil
	}
	options := docker.ResolverOptions{
		Tracker: docker.NewInMemoryTracker(),
		Hosts:   config.ConfigureHosts(ctr.CCtx, hostOpt),
	}

	pullOpts := []containerd.RemoteOpt{
		containerd.WithPullUnpack,
		containerd.WithResolver(docker.NewResolver(options)),
	}
	_, err := ctr.ContainerdClient.Pull(ctr.CCtx, component.ShareImage, pullOpts...)
	if err != nil {
		log.Errorf("plugin image %s by user %s failure %s", component.ShareImage, component.AppImage.HubUser, err.Error())
		return "", err
	}
	return component.ShareImage, nil
}

func pullPluginImage(ctr ContainerdAPI, plugin *v1alpha1.Plugin, log *logrus.Logger) (string, error) {
	// docker pull image-name
	//_, err := docker.ImagePull(client, plugin.ShareImage, plugin.PluginImage.HubUser, plugin.PluginImage.HubPassword, 30)
	defaultTLS := &tls.Config{
		InsecureSkipVerify: true,
	}

	hostOpt := config.HostOptions{}
	hostOpt.DefaultTLS = defaultTLS
	hostOpt.Credentials = func(host string) (string, string, error) {
		return plugin.PluginImage.HubUser, plugin.PluginImage.HubPassword, nil
	}
	options := docker.ResolverOptions{
		Tracker: docker.NewInMemoryTracker(),
		Hosts:   config.ConfigureHosts(ctr.CCtx, hostOpt),
	}

	pullOpts := []containerd.RemoteOpt{
		containerd.WithPullUnpack,
		containerd.WithResolver(docker.NewResolver(options)),
	}
	_, err := ctr.ContainerdClient.Pull(ctr.CCtx, plugin.ShareImage, pullOpts...)
	if err != nil {
		log.Errorf("plugin image %s by user %s failure %s", plugin.ShareImage, plugin.PluginImage.HubUser, err.Error())
		return "", err
	}
	return plugin.ShareImage, nil
}

// GetMemoryType returns the memory type based on the given memory size.
func GetMemoryType(memorySize int) string {
	memoryType := "small"
	if v, ok := memoryLabels[memorySize]; ok {
		memoryType = v
	}
	return memoryType
}

var memoryLabels = map[int]string{
	128:   "micro",
	256:   "small",
	512:   "medium",
	1024:  "large",
	2048:  "2xlarge",
	4096:  "4xlarge",
	8192:  "8xlarge",
	16384: "16xlarge",
	32768: "32xlarge",
	65536: "64xlarge",
}

//PrepareExportDir -
func PrepareExportDir(exportPath string) error {
	os.RemoveAll(exportPath)
	return os.MkdirAll(exportPath, 0755)
}

func exportComponentConfigFile(serviceDir string, v v1alpha1.ComponentVolume) error {
	serviceDir = strings.TrimRight(serviceDir, "/")
	filename := fmt.Sprintf("%s%s", serviceDir, v.VolumeMountPath)
	dir := path.Dir(filename)
	os.MkdirAll(dir, 0755)
	return ioutil.WriteFile(filename, []byte(v.FileConent), 0644)
}
