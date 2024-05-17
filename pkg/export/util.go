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
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/goodrain/rainbond-oam/pkg/util/image"
	"github.com/mozillazg/go-pinyin"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
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

func SaveComponents(ram v1alpha1.RainbondApplicationConfig, imageClient image.Client, exportPath string, logger *logrus.Logger, dependentImages []string) error {
	var componentImageNames []string
	for _, component := range ram.Components {
		componentName := unicode2zh(component.ServiceCname)
		if component.ShareImage != "" {
			// app is image type
			_, err := imageClient.ImagePull(component.ShareImage, component.AppImage.HubUser, component.AppImage.HubPassword, 30)
			if err != nil {
				return err
			}
			logger.Infof("pull component %s image success", componentName)
			componentImageNames = append(componentImageNames, component.ShareImage)
		}
	}
	start := time.Now()
	for _, dependentImage := range dependentImages {
		if dependentImage == "" {
			continue
		}
		componentImageNames = append(componentImageNames, dependentImage)
	}
	err := imageClient.ImageSave(fmt.Sprintf("%s/component-images.tar", exportPath), componentImageNames)
	if err != nil {
		logrus.Errorf("Failed to save image(%v) : %s", componentImageNames, err)
		return err
	}
	logger.Infof("save component images success, Take %s time", time.Now().Sub(start))
	return nil
}

func SavePlugins(ram v1alpha1.RainbondApplicationConfig, imageClient image.Client, exportPath string, logger *logrus.Logger) error {
	var pluginImageNames []string
	for _, plugin := range ram.Plugins {
		if plugin.ShareImage != "" {
			// app is image type
			_, err := imageClient.ImagePull(plugin.ShareImage, plugin.PluginImage.HubUser, plugin.PluginImage.HubPassword, 30)
			if err != nil {
				return err
			}
			logger.Infof("pull plugin %s image success", plugin.PluginName)
			pluginImageNames = append(pluginImageNames, plugin.ShareImage)
		}
	}
	start := time.Now()
	err := imageClient.ImageSave(fmt.Sprintf("%s/plugin-images.tar", exportPath), pluginImageNames)
	if err != nil {
		logrus.Errorf("Failed to save image(%v) : %s", pluginImageNames, err)
		return err
	}
	logger.Infof("save plugin images success, Take %s time", time.Now().Sub(start))
	return nil
}

func Packaging(packageName, homePath, exportPath string) (string, error) {
	cmd := exec.Command("tar", "-czf", path.Join(homePath, packageName), path.Base(exportPath))
	cmd.Dir = homePath
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return packageName, nil
}

func PackagingBZip2(packageName, homePath, exportPath string) (string, error) {
	cmd := exec.Command("tar", "-C", path.Base(exportPath), "-cjf", path.Join(homePath, packageName), ".")
	cmd.Dir = homePath
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return packageName, nil
}
