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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	cdocker "github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
	"github.com/goodrain/rainbond-oam/pkg/export"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/goodrain/rainbond-oam/pkg/util"
	"github.com/goodrain/rainbond-oam/pkg/util/docker"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

//AppLocalImport import
type AppLocalImport interface {
	Import(filePath string, hubInfo v1alpha1.ImageInfo) (*v1alpha1.RainbondApplicationConfig, error)
}

//New new
func New(logger *logrus.Logger, ctr export.ContainerdAPI, homeDir string) AppLocalImport {
	return &ramImport{
		logger:  logger,
		ctr:     ctr,
		homeDir: homeDir,
	}
}

type ramImport struct {
	logger  *logrus.Logger
	ctr     export.ContainerdAPI
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
			rc, err := os.Open(f)
			if err != nil {
				return nil, err
			}
			//if err := docker.ImageLoad(r.client, f); err != nil {
			if _, err := r.ctr.ContainerdClient.Import(r.ctr.CCtx, rc); err != nil {
				logrus.Errorf("load image from file %s failure %s", f, err.Error())
			}
			rc.Close()
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
		image, err := r.ctr.ImageService.Get(r.ctr.CCtx, com.ShareImage)
		if err != nil {
			return nil, err
		}
		image.Name = newImageName
		r.logger.Infof("-----------------1-----------------")
		if _, err = r.ctr.ImageService.Create(r.ctr.CCtx, image); err != nil {
			// If user has specified force and the image already exists then
			// delete the original image and attempt to create the new one
			if errdefs.IsAlreadyExists(err) {
				if err = r.ctr.ImageService.Delete(r.ctr.CCtx, newImageName); err != nil {
					return nil, err
				}
				if _, err = r.ctr.ImageService.Create(r.ctr.CCtx, image); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
		r.logger.Infof("start push image %s", newImageName)
		defaultTLS := &tls.Config{
			InsecureSkipVerify: true,
		}

		hostOpt := config.HostOptions{}
		hostOpt.DefaultTLS = defaultTLS
		hostOpt.Credentials = func(host string) (string, string, error) {
			return hubInfo.HubUser, hubInfo.HubPassword, nil
		}
		options := cdocker.ResolverOptions{
			Tracker: cdocker.NewInMemoryTracker(),
			Hosts:   config.ConfigureHosts(r.ctr.CCtx, hostOpt),
		}
		r.logger.Infof("-----------------2-----------------")
		err = r.ctr.ContainerdClient.Push(r.ctr.CCtx, image.Name, image.Target, containerd.WithResolver(cdocker.NewResolver(options)))
		if err != nil {
			return nil, fmt.Errorf("push image %v err:%v", image.Name, err)
		}
		r.logger.Infof("push image %s success", newImageName)
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
		image, err := r.ctr.ImageService.Get(r.ctr.CCtx, plugin.ShareImage)
		if err != nil {
			return nil, err
		}
		image.Name = newImageName
		r.logger.Infof("-----------------3-----------------")
		if _, err = r.ctr.ImageService.Create(r.ctr.CCtx, image); err != nil {
			// If user has specified force and the image already exists then
			// delete the original image and attempt to create the new one
			if errdefs.IsAlreadyExists(err) {
				if err = r.ctr.ImageService.Delete(r.ctr.CCtx, newImageName); err != nil {
					return nil, err
				}
				if _, err = r.ctr.ImageService.Create(r.ctr.CCtx, image); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}

		defaultTLS := &tls.Config{
			InsecureSkipVerify: true,
		}

		hostOpt := config.HostOptions{}
		hostOpt.DefaultTLS = defaultTLS
		hostOpt.Credentials = func(host string) (string, string, error) {
			return hubInfo.HubUser, hubInfo.HubPassword, nil
		}
		options := cdocker.ResolverOptions{
			Tracker: cdocker.NewInMemoryTracker(),
			Hosts:   config.ConfigureHosts(r.ctr.CCtx, hostOpt),
		}

		r.logger.Infof("-----------------4-----------------")
		err = r.ctr.ContainerdClient.Push(r.ctr.CCtx, image.Name, image.Target, containerd.WithResolver(cdocker.NewResolver(options)))
		if err != nil {
			return nil, err
		}
		r.logger.Infof("push image %s success", newImageName)
		ram.Plugins[i].PluginImage = hubInfo
		ram.Plugins[i].ShareImage = newImageName
	}
	return &ram, nil
}
