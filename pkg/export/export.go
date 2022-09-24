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
	"context"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/sirupsen/logrus"
	"path"
)

//AppLocalExport export local package
type AppLocalExport interface {
	Export() (*Result, error)
}

//Result export result
type Result struct {
	PackagePath   string
	PackageName   string
	PackageFormat string
}

type ContainerdAPI struct {
	ImageService     images.Store
	CCtx             context.Context
	ContainerdClient *containerd.Client
}

//AppFormat app spec format
type AppFormat string

var (
	//RAM -
	RAM AppFormat = "ram"
	//DC -
	DC AppFormat = "docker-compose"
	//SC -
	SLG AppFormat = "slug"
)

//New new exporter
func New(format AppFormat, homePath string, ram v1alpha1.RainbondApplicationConfig, ctr ContainerdAPI, logger *logrus.Logger) AppLocalExport {
	switch format {
	case RAM:
		return &ramExporter{
			logger:     logger,
			ram:        ram,
			ctr:        ctr,
			mode:       "offline",
			homePath:   homePath,
			exportPath: path.Join(homePath, fmt.Sprintf("%s-%s-ram", ram.AppName, ram.AppVersion)),
		}
	case DC:
		return &dockerComposeExporter{
			logger:     logger,
			ram:        ram,
			ctr:        ctr,
			homePath:   homePath,
			exportPath: path.Join(homePath, fmt.Sprintf("%s-%s-dockercompose", ram.AppName, ram.AppVersion)),
		}
	case SLG:
		return &slugExporter{
			logger:     logger,
			ram:        ram,
			ctr:        ctr,
			mode:       "offline",
			homePath:   homePath,
			exportPath: path.Join(homePath, fmt.Sprintf("%s-%s-slug", ram.AppName, ram.AppVersion)),
		}
	default:
		panic("not support app format")
	}
}
