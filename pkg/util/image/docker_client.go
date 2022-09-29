package image

import (
	"context"
	dockercli "github.com/docker/docker/client"
	"github.com/goodrain/rainbond-oam/pkg/util/docker"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type dockerImageCliImpl struct {
	client *dockercli.Client
}

//ImageSave save image to tar file
// destination destination file name eg. /tmp/xxx.tar
func (d *dockerImageCliImpl) ImageSave(destination string, images []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return docker.MultiImageSave(ctx, d.client, destination, images...)
}

func (d *dockerImageCliImpl) ImagePull(image string, username, password string, timeout int) (*ocispec.ImageConfig, error) {
	img, err := docker.ImagePull(d.client, image, username, password, timeout)
	if err != nil {
		return nil, err
	}
	exportPorts := make(map[string]struct{})
	for port := range img.Config.ExposedPorts {
		exportPorts[string(port)] = struct{}{}
	}
	return &ocispec.ImageConfig{
		User:         img.Config.User,
		ExposedPorts: exportPorts,
		Env:          img.Config.Env,
		Entrypoint:   img.Config.Entrypoint,
		Cmd:          img.Config.Cmd,
		Volumes:      img.Config.Volumes,
		WorkingDir:   img.Config.WorkingDir,
		Labels:       img.Config.Labels,
		StopSignal:   img.Config.StopSignal,
	}, nil
}

func (d *dockerImageCliImpl) ImageLoad(tarFile string) error {
	return docker.ImageLoad(d.client, tarFile)
}

func (d *dockerImageCliImpl) ImagePush(image, user, pass string, timeout int) error {
	return docker.ImagePush(d.client, image, user, pass, timeout)
}

func (d *dockerImageCliImpl) ImageTag(source, target string, timeout int) error {
	return docker.ImageTag(d.client, source, target, timeout)
}
