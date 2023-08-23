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
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/goodrain/rainbond-oam/pkg/util/image"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"path"
)

type ramExporter struct {
	logger      *logrus.Logger
	ram         v1alpha1.RainbondApplicationConfig
	imageClient image.Client
	mode        string
	homePath    string
	exportPath  string
}

func (r *ramExporter) Export() (*Result, error) {
	r.logger.Infof("start export app %s to ram app spec", r.ram.AppName)
	// Delete the old application group directory and then regenerate the application package
	if err := PrepareExportDir(r.exportPath); err != nil {
		r.logger.Errorf("prepare export dir failure %s", err.Error())
		return nil, err
	}
	r.logger.Infof("success prepare export dir")
	if r.mode == "offline" {
		// Save components attachments
		if len(r.ram.Components) > 0 {
			if err := SaveComponents(r.ram, r.imageClient, r.exportPath, r.logger, []string{}); err != nil {
				return nil, err
			}
			r.logger.Infof("success save components")
		}
		if len(r.ram.Plugins) > 0 {
			if err := SavePlugins(r.ram, r.imageClient, r.exportPath, r.logger); err != nil {
				return nil, err
			}
			r.logger.Infof("success save plugins")
		}
	}
	if err := r.writeMetaFile(); err != nil {
		return nil, err
	}
	r.logger.Infof("success write ram spec file")
	// packaging
	packageName := fmt.Sprintf("%s-%s-ram.tar.gz", r.ram.AppName, r.ram.AppVersion)
	name, err := Packaging(packageName, r.homePath, r.exportPath)
	if err != nil {
		err = fmt.Errorf("Failed to package app %s: %s ", packageName, err.Error())
		r.logger.Error(err)
		return nil, err
	}
	r.logger.Infof("success export app " + r.ram.AppName)
	return &Result{PackagePath: path.Join(r.homePath, name), PackageName: name}, nil
}

func (r *ramExporter) writeMetaFile() error {
	// remove component and plugin image hub info
	if r.mode == "offline" {
		for i := range r.ram.Components {
			r.ram.Components[i].AppImage = v1alpha1.ImageInfo{}
		}
		for i := range r.ram.Plugins {
			r.ram.Plugins[i].PluginImage = v1alpha1.ImageInfo{}
		}
	}
	meta, err := json.Marshal(r.ram)
	if err != nil {
		return fmt.Errorf("marshal ram meta config failure %s", err.Error())
	}
	if err := ioutil.WriteFile(path.Join(r.exportPath, "metadata.json"), meta, 0755); err != nil {
		return fmt.Errorf("write ram app meta config file failure %s", err.Error())
	}
	return nil
}
