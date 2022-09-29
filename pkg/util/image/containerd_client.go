package image

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd"
	ctrcontent "github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/progress"
	"github.com/containerd/containerd/platforms"
	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"os"
	"sync"
	"text/tabwriter"
	"time"
)

type containerdImageCliImpl struct {
	client *containerd.Client
}

//ImageSave save image to tar file
// destination destination file name eg. /tmp/xxx.tar
func (c *containerdImageCliImpl) ImageSave(destination string, images []string) error {
	var exportOpts []archive.ExportOpt
	for _, image := range images {
		ref, err := refdocker.ParseDockerRef(image)
		if err != nil {
			logrus.Errorf("parse image %s error %s", image, err.Error())
			continue
		}
		exportOpts = append(exportOpts, archive.WithImage(c.client.ImageService(), ref.String()))
	}
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	w, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer w.Close()
	return c.client.Export(ctx, w, exportOpts...)
}

func (c *containerdImageCliImpl) ImagePull(image string, username, password string, timeout int) (*ocispec.ImageConfig, error) {
	named, err := refdocker.ParseDockerRef(image)
	if err != nil {
		return nil, err
	}
	reference := named.String()
	ongoing := ctrcontent.NewJobs(reference)
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	pctx, stopProgress := context.WithCancel(ctx)
	progress := make(chan struct{})

	go func() {
		ctrcontent.ShowProgress(pctx, ongoing, c.client.ContentStore(), os.Stdout)
		close(progress)
	}()
	h := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if desc.MediaType != images.MediaTypeDockerSchema1Manifest {
			ongoing.Add(desc)
		}
		return nil, nil
	})
	defaultTLS := &tls.Config{
		InsecureSkipVerify: true,
	}
	hostOpt := config.HostOptions{}
	hostOpt.DefaultTLS = defaultTLS
	hostOpt.Credentials = func(host string) (string, string, error) {
		return username, password, nil
	}
	Tracker := docker.NewInMemoryTracker()
	options := docker.ResolverOptions{
		Tracker: Tracker,
		Hosts:   config.ConfigureHosts(pctx, hostOpt),
	}

	platformMC := platforms.Ordered([]ocispec.Platform{platforms.DefaultSpec()}...)
	opts := []containerd.RemoteOpt{
		containerd.WithImageHandler(h),
		//nolint:staticcheck
		containerd.WithSchema1Conversion, //lint:ignore SA1019 nerdctl should support schema1 as well.
		containerd.WithPlatformMatcher(platformMC),
		containerd.WithResolver(docker.NewResolver(options)),
	}
	var img containerd.Image
	img, err = c.client.Pull(pctx, reference, opts...)
	stopProgress()
	if err != nil {
		return nil, err
	}
	<-progress
	return getImageConfig(ctx, img)
}

func getImageConfig(ctx context.Context, image containerd.Image) (*ocispec.ImageConfig, error) {
	desc, err := image.Config(ctx)
	if err != nil {
		return nil, err
	}
	switch desc.MediaType {
	case ocispec.MediaTypeImageConfig, images.MediaTypeDockerSchema2Config:
		var ocispecImage ocispec.Image
		b, err := content.ReadBlob(ctx, image.ContentStore(), desc)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(b, &ocispecImage); err != nil {
			return nil, err
		}
		return &ocispecImage.Config, nil
	default:
		return nil, fmt.Errorf("unknown media type %q", desc.MediaType)
	}
}

// ImageLoad load image from  tar file
// destination destination file name eg. /tmp/xxx.tar
func (c *containerdImageCliImpl) ImageLoad(tarFile string) error {
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	reader, err := os.OpenFile(tarFile, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer reader.Close()
	if _, err = c.client.Import(ctx, reader); err != nil {
		return err
	}
	return nil
}

func (c *containerdImageCliImpl) ImagePush(image, user, pass string, timeout int) error {
	named, err := refdocker.ParseDockerRef(image)
	if err != nil {
		return err
	}
	reference := named.String()
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	img, err := c.client.ImageService().Get(ctx, reference)
	if err != nil {
		return errors.Wrap(err, "unable to resolve image to manifest")
	}
	desc := img.Target
	cs := c.client.ContentStore()
	if manifests, err := images.Children(ctx, cs, desc); err == nil && len(manifests) > 0 {
		matcher := platforms.NewMatcher(platforms.DefaultSpec())
		for _, manifest := range manifests {
			if manifest.Platform != nil && matcher.Match(*manifest.Platform) {
				if _, err := images.Children(ctx, cs, manifest); err != nil {
					return errors.Wrap(err, "no matching manifest")
				}
				desc = manifest
				break
			}
		}
	}
	NewTracker := docker.NewInMemoryTracker()
	options := docker.ResolverOptions{
		Tracker: NewTracker,
	}
	hostOptions := config.HostOptions{
		DefaultTLS: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	hostOptions.Credentials = func(host string) (string, string, error) {
		return user, pass, nil
	}
	options.Hosts = config.ConfigureHosts(ctx, hostOptions)
	resolver := docker.NewResolver(options)
	ongoing := newPushJobs(NewTracker)

	eg, ctx := errgroup.WithContext(ctx)
	// used to notify the progress writer
	doneCh := make(chan struct{})
	eg.Go(func() error {
		defer close(doneCh)
		jobHandler := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			ongoing.add(remotes.MakeRefKey(ctx, desc))
			return nil, nil
		})

		ropts := []containerd.RemoteOpt{
			containerd.WithResolver(resolver),
			containerd.WithImageHandler(jobHandler),
		}
		return c.client.Push(ctx, reference, desc, ropts...)
	})
	eg.Go(func() error {
		var (
			ticker = time.NewTicker(100 * time.Millisecond)
			fw     = progress.NewWriter(os.Stdout)
			start  = time.Now()
			done   bool
		)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fw.Flush()
				tw := tabwriter.NewWriter(fw, 1, 8, 1, ' ', 0)
				ctrcontent.Display(tw, ongoing.status(), start)
				tw.Flush()
				if done {
					fw.Flush()
					return nil
				}
			case <-doneCh:
				done = true
			case <-ctx.Done():
				done = true // allow ui to update once more
			}
		}
	})
	return nil
}

type pushjobs struct {
	jobs    map[string]struct{}
	ordered []string
	tracker docker.StatusTracker
	mu      sync.Mutex
}

func newPushJobs(tracker docker.StatusTracker) *pushjobs {
	return &pushjobs{
		jobs:    make(map[string]struct{}),
		tracker: tracker,
	}
}

func (j *pushjobs) add(ref string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if _, ok := j.jobs[ref]; ok {
		return
	}
	j.ordered = append(j.ordered, ref)
	j.jobs[ref] = struct{}{}
}

func (j *pushjobs) status() []ctrcontent.StatusInfo {
	j.mu.Lock()
	defer j.mu.Unlock()

	statuses := make([]ctrcontent.StatusInfo, 0, len(j.jobs))
	for _, name := range j.ordered {
		si := ctrcontent.StatusInfo{
			Ref: name,
		}

		status, err := j.tracker.GetStatus(name)
		if err != nil {
			si.Status = "waiting"
		} else {
			si.Offset = status.Offset
			si.Total = status.Total
			si.StartedAt = status.StartedAt
			si.UpdatedAt = status.UpdatedAt
			if status.Offset >= status.Total {
				if status.UploadUUID == "" {
					si.Status = "done"
				} else {
					si.Status = "committing"
				}
			} else {
				si.Status = "uploading"
			}
		}
		statuses = append(statuses, si)
	}

	return statuses
}

//ImageTag change docker image tag
func (c *containerdImageCliImpl) ImageTag(source, target string, timeout int) error {
	srcNamed, err := refdocker.ParseDockerRef(source)
	if err != nil {
		return err
	}
	srcImage := srcNamed.String()
	targetNamed, err := refdocker.ParseDockerRef(target)
	if err != nil {
		return err
	}
	targetImage := targetNamed.String()
	logrus.Infof(fmt.Sprintf("change image tag：%s -> %s", srcImage, targetImage))
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	imageService := c.client.ImageService()
	image, err := imageService.Get(ctx, srcImage)
	if err != nil {
		logrus.Errorf("imagetag imageService Get error: %s", err.Error())
		return err
	}
	image.Name = targetImage
	if _, err = imageService.Create(ctx, image); err != nil {
		if errdefs.IsAlreadyExists(err) {
			if err = imageService.Delete(ctx, image.Name); err != nil {
				logrus.Errorf("imagetag imageService Delete error: %s", err.Error())
				return err
			}
			if _, err = imageService.Create(ctx, image); err != nil {
				logrus.Errorf("imageService Create error: %s", err.Error())
				return err
			}
		} else {
			logrus.Errorf("imagetag imageService Create error: %s", err.Error())
			return err
		}
	}
	logrus.Info("change image tag success")
	return nil
}
