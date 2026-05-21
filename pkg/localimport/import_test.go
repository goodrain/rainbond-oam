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
	"os"
	"testing"

	"github.com/containerd/containerd"
	"github.com/docker/docker/client"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/sirupsen/logrus"
)

func TestRewriteComponentVMImageReferences(t *testing.T) {
	component := &v1alpha1.Component{
		ShareImage: "docker.io/library/vmexport-test:qcow2-v1",
		VM: &v1alpha1.VMTemplate{
			DiskLayout: []v1alpha1.VMDiskLayoutItem{
				{
					DiskKey:    "disk",
					DiskRole:   v1alpha1.VMDiskRoleRoot,
					SourceType: v1alpha1.VMDiskSourceRegistry,
					Image:      "docker.io/library/vmexport-test:qcow2-v1",
				},
				{
					DiskKey:    "data1",
					DiskRole:   v1alpha1.VMDiskRoleData,
					SourceType: v1alpha1.VMDiskSourceRegistry,
					Image:      "docker.io/library/other-data:v1",
				},
			},
		},
	}

	rewriteComponentVMImageReferences(component, component.ShareImage, "registry.example.com/team/vmexport-test:qcow2-v1")

	if got := component.VM.DiskLayout[0].Image; got != "registry.example.com/team/vmexport-test:qcow2-v1" {
		t.Fatalf("expected root disk image to be rewritten, got %s", got)
	}
	if got := component.VM.DiskLayout[1].Image; got != "docker.io/library/other-data:v1" {
		t.Fatalf("expected unrelated data disk image to stay unchanged, got %s", got)
	}
}

func TestRewriteComponentVMImageReferencesBackfillsEmptyRootDiskImage(t *testing.T) {
	component := &v1alpha1.Component{
		ShareImage: "docker.io/library/vmexport-test:qcow2-v1",
		VM: &v1alpha1.VMTemplate{
			DiskLayout: []v1alpha1.VMDiskLayoutItem{
				{
					DiskKey:    "disk",
					DiskRole:   v1alpha1.VMDiskRoleRoot,
					SourceType: v1alpha1.VMDiskSourceRegistry,
					Image:      "",
				},
			},
		},
	}

	rewriteComponentVMImageReferences(component, component.ShareImage, "registry.example.com/team/vmexport-test:qcow2-v1")

	if got := component.VM.DiskLayout[0].Image; got != "registry.example.com/team/vmexport-test:qcow2-v1" {
		t.Fatalf("expected empty root disk image to be backfilled, got %s", got)
	}
}

func TestImport(t *testing.T) {
	t.Skip("manual integration test")
	c, _ := client.NewEnvClient()
	im, _ := New(logrus.StandardLogger(), (*containerd.Client)(nil), c, "/tmp/ram/default")
	info, err := im.Import("/Users/barnett/Downloads/默认应用-1.0-ram.tar.gz", v1alpha1.ImageInfo{
		HubPassword: os.Getenv("PASS"),
		Namespace:   "test",
		HubURL:      "image.goodrain.com",
		HubUser:     "root",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", info)
}
