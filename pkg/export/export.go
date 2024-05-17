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
	"github.com/containerd/containerd"
	dockercli "github.com/docker/docker/client"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/goodrain/rainbond-oam/pkg/util/image"
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

//AppFormat app spec format
type AppFormat string

var (
	//RAM -
	RAM AppFormat = "ram"
	//DC -
	DC AppFormat = "docker-compose"
	//SC -
	SLG AppFormat = "slug"
	//HELM
	HELM AppFormat = "helm-chart"
	//CPK
	CPK AppFormat = "cpk"
)

//New new exporter
func New(format AppFormat, homePath string, ram v1alpha1.RainbondApplicationConfig, containerdCli *containerd.Client, dockerCli *dockercli.Client, logger *logrus.Logger) (AppLocalExport, error) {
	imageClient, err := image.NewClient(containerdCli, dockerCli)
	if err != nil {
		logger.Errorf("create image client error: %v", err)
		return nil, err
	}
	switch format {
	case RAM:
		return &ramExporter{
			logger:      logger,
			ram:         ram,
			imageClient: imageClient,
			mode:        "offline",
			homePath:    homePath,
			exportPath:  path.Join(homePath, fmt.Sprintf("%s-%s-ram", ram.AppName, ram.AppVersion)),
		}, nil
	case DC:
		return &dockerComposeExporter{
			logger:      logger,
			ram:         ram,
			imageClient: imageClient,
			homePath:    homePath,
			exportPath:  path.Join(homePath, fmt.Sprintf("%s-%s-dockercompose", ram.AppName, ram.AppVersion)),
		}, nil
	case SLG:
		return &slugExporter{
			logger:      logger,
			ram:         ram,
			imageClient: imageClient,
			mode:        "offline",
			homePath:    homePath,
			exportPath:  path.Join(homePath, fmt.Sprintf("%s-%s-slug", ram.AppName, ram.AppVersion)),
		}, nil
	case HELM:
		return &helmChartExporter{
			logger:      logger,
			ram:         ram,
			imageClient: imageClient,
			mode:        "offline",
			homePath:    homePath,
			exportPath:  path.Join(homePath, fmt.Sprintf("%s-%s-helm", ram.AppName, ram.AppVersion)),
		}, nil
	case CPK:
		return &cpkExporter{
			logger:      logger,
			ram:         ram,
			imageClient: imageClient,
			mode:        "offline",
			homePath:    homePath,
			exportPath:  path.Join(homePath, fmt.Sprintf("%s-%s-cpk", ram.AppName, ram.AppVersion)),
		}, nil
	default:
		panic("not support app format")
	}
}
