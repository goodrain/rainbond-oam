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

package localimport

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/docker/docker/client"
	"github.com/goodrain/rainbond-oam/pkg/export"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/goodrain/rainbond-oam/pkg/util"
	"github.com/goodrain/rainbond-oam/pkg/util/docker"
	"github.com/sirupsen/logrus"
)

//AppLocalImport import
type AppLocalImport interface {
	Import(filePath string, hubInfo v1alpha1.ImageInfo) (*v1alpha1.RainbondApplicationConfig, error)
}

//New new
func New(logger *logrus.Logger, client *client.Client, homeDir string) AppLocalImport {
	return &ramImport{
		logger:  logger,
		client:  client,
		homeDir: homeDir,
	}
}

type ramImport struct {
	logger  *logrus.Logger
	client  *client.Client
	homeDir string
}

func (r *ramImport) Import(filePath string, hubInfo v1alpha1.ImageInfo) (*v1alpha1.RainbondApplicationConfig, error) {
	if hubInfo.HubURL == "" {
		return nil, fmt.Errorf("must define hub url")
	}
	r.logger.Infof("start import app by app file %s", filePath)
	if err := export.PrepareExportDir(r.homeDir); err != nil {
		r.logger.Errorf("prepare import dir failure %s", err.Error())
		return nil, err
	}
	ext := path.Ext(filePath)
	if ext == ".zip" {
		if err := util.Unzip(filePath, r.homeDir); err != nil {
			r.logger.Errorf("unzip file %s faile %s", filePath, err.Error())
			return nil, err
		}
	} else {
		if err := util.Untar(filePath, r.homeDir); err != nil {
			r.logger.Errorf("untar file %s faile %s", filePath, err.Error())
			return nil, err
		}
	}
	r.logger.Infof("prepare app meta file success")
	// read app meta config
	files, _ := ioutil.ReadDir(r.homeDir)
	if len(files) < 1 {
		return nil, fmt.Errorf("Failed to read files in tmp dir %s", r.homeDir)
	}
	metaFile, err := os.Open(path.Join(r.homeDir, files[0].Name(), "metadata.json"))
	if err != nil {
		return nil, fmt.Errorf("Failed to read files in tmp dir %s: %v", r.homeDir, err)
	}
	defer metaFile.Close()
	var ram v1alpha1.RainbondApplicationConfig
	if err := json.NewDecoder(metaFile).Decode(&ram); err != nil {
		return nil, fmt.Errorf("Failed to read meta file : %v", err)
	}
	// load all component images and plugin images
	//after v5.3 package
	l1, err := util.GetFileList(path.Join(r.homeDir, files[0].Name()), 1)
	if err != nil {
		return nil, err
	}
	//before v5.3 package
	l2, err := util.GetFileList(path.Join(r.homeDir, files[0].Name()), 2)
	if err != nil {
		return nil, err
	}
	allfiles := append(l1, l2...)
	for _, f := range allfiles {
		if strings.HasSuffix(f, ".tar") {
			if err := docker.ImageLoad(r.client, f); err != nil {
				return nil, fmt.Errorf("load image from file %s failure %s", f, err.Error())
			}
			r.logger.Infof("load image from file %s success", f)
		}
	}
	for _, com := range ram.Components {
		// new hub info
		newImageName, err := docker.NewImageName(com.ShareImage, hubInfo)
		if err != nil {
			r.logger.Errorf("parse image failure %s", err.Error())
			return nil, err
		}
		if err := docker.ImageTag(r.client, com.ShareImage, newImageName, 2); err != nil {
			//Compatibility History Version
			if strings.Contains(err.Error(), "No such image") {
				var saveImage string
				saveImage, err = docker.GetOldSaveImageName(com.ShareImage, false)
				if err != nil {
					return nil, err
				}
				err = docker.ImageTag(r.client, saveImage, newImageName, 2)
				if err != nil && strings.Contains(err.Error(), "No such image") {
					saveImage, err = docker.GetOldSaveImageName(com.ShareImage, true)
					if err != nil {
						return nil, err
					}
					err = docker.ImageTag(r.client, saveImage, newImageName, 2)
				}
			}
			if err != nil {
				logrus.Errorf("change image %s tag to %s failure %s", com.ShareImage, newImageName, err.Error())
				return nil, err
			}
		}
		if err := docker.ImagePush(r.client, newImageName, hubInfo.HubUser, hubInfo.HubPassword, 20); err != nil {
			logrus.Errorf("push image %s failure %s", newImageName, err.Error())
			return nil, err
		}
		com.AppImage = hubInfo
		com.ShareImage = newImageName
	}
	for i, plugin := range ram.Plugins {
		// new hub info
		newImageName, err := docker.NewImageName(plugin.ShareImage, hubInfo)
		if err != nil {
			r.logger.Errorf("parse image failure %s", err.Error())
			return nil, err
		}
		if err := docker.ImageTag(r.client, plugin.ShareImage, newImageName, 2); err != nil {
			//Compatibility History Version
			if strings.Contains(err.Error(), "No such image") {
				var saveImage string
				saveImage, err = docker.GetOldSaveImageName(plugin.ShareImage, false)
				if err != nil {
					return nil, err
				}
				err = docker.ImageTag(r.client, saveImage, newImageName, 2)
				if err != nil && strings.Contains(err.Error(), "No such image") {
					saveImage, err = docker.GetOldSaveImageName(plugin.ShareImage, true)
					if err != nil {
						return nil, err
					}
					err = docker.ImageTag(r.client, saveImage, newImageName, 2)
				}
			}
			if err != nil {
				logrus.Errorf("change image %s tag to %s failure %s", plugin.ShareImage, newImageName, err.Error())
				return nil, err
			}
		}
		if err := docker.ImagePush(r.client, newImageName, hubInfo.HubUser, hubInfo.HubPassword, 20); err != nil {
			logrus.Errorf("push image %s failure %s", newImageName, err.Error())
			return nil, err
		}
		ram.Plugins[i].PluginImage = hubInfo
		ram.Plugins[i].ShareImage = newImageName
	}
	return &ram, nil
}
