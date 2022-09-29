package image

import (
	"fmt"
	"github.com/containerd/containerd"
	dockercli "github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const Namespace = "k8s.io"

type Client interface {
	ImageSave(destination string, images []string) error
	ImageLoad(tarFile string) error
	ImagePull(image string, username, password string, timeout int) (*ocispec.ImageConfig, error)
	ImagePush(image, user, pass string, timeout int) error
	ImageTag(source, target string, timeout int) error
}

func NewClient(client *containerd.Client, dockerCli *dockercli.Client) (c Client, err error) {
	if client != nil {
		return &containerdImageCliImpl{
			client: client,
		}, nil
	}
	if dockerCli != nil {
		return &dockerImageCliImpl{
			client: dockerCli,
		}, nil
	}
	return nil, fmt.Errorf("client is nil")
}
