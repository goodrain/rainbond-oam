package image

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/remotes/docker"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type staticImageStore struct {
	image images.Image
}

type memoryProvider struct {
	blobs map[digest.Digest][]byte
}

type memoryReaderAt struct {
	data []byte
}

func (s staticImageStore) Get(ctx context.Context, name string) (images.Image, error) {
	return s.image, nil
}

func (s staticImageStore) List(ctx context.Context, filters ...string) ([]images.Image, error) {
	return []images.Image{s.image}, nil
}

func (s staticImageStore) Create(ctx context.Context, image images.Image) (images.Image, error) {
	return image, nil
}

func (s staticImageStore) Update(ctx context.Context, image images.Image, fieldpaths ...string) (images.Image, error) {
	return image, nil
}

func (s staticImageStore) Delete(ctx context.Context, name string, opts ...images.DeleteOpt) error {
	return nil
}

func (p memoryProvider) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	data, ok := p.blobs[desc.Digest]
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}
	return &memoryReaderAt{data: data}, nil
}

func (r *memoryReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[off:])
	if int(off)+n >= len(r.data) {
		return n, io.EOF
	}
	return n, nil
}

func (r *memoryReaderAt) Close() error {
	return nil
}

func (r *memoryReaderAt) Size() int64 {
	return int64(len(r.data))
}

func writeBlob(t *testing.T, blobs map[digest.Digest][]byte, payload []byte, mediaType string) ocispec.Descriptor {
	t.Helper()
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Size:      int64(len(payload)),
		Digest:    digest.FromBytes(payload),
	}
	blobs[desc.Digest] = payload
	return desc
}

func TestPlainHTTPRegistryErrorIsRecognized(t *testing.T) {
	err := errors.New(`failed to do request: Head "https://172.16.0.231:8081/v2/library/nginx/manifests/latest": http: server gave HTTP response to HTTPS client`)

	if !isPlainHTTPRegistryError(err) {
		t.Fatalf("expected plain HTTP registry error to be recognized")
	}
}

func TestPlainHTTPRegistryErrorIgnoresOtherPullErrors(t *testing.T) {
	for _, err := range []error{
		nil,
		errors.New("pull access denied"),
		errors.New("manifest unknown"),
		errors.New("context deadline exceeded"),
	} {
		if isPlainHTTPRegistryError(err) {
			t.Fatalf("expected %v not to trigger HTTP fallback", err)
		}
	}
}

func TestContainerdResolverOptionsCanUsePlainHTTP(t *testing.T) {
	opts := containerdResolverOptions(context.Background(), docker.NewInMemoryTracker(), newContainerdHostOptions("demo", "password", "http"))

	hosts, err := opts.Hosts("172.16.0.231:8081")
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected one registry host, got %d", len(hosts))
	}
	if hosts[0].Scheme != "http" {
		t.Fatalf("expected resolver scheme http, got %q", hosts[0].Scheme)
	}
	if hosts[0].Host != "172.16.0.231:8081" {
		t.Fatalf("expected registry host to be preserved, got %q", hosts[0].Host)
	}
}

func TestWaitForContainerdPushWaitsForPushCompletion(t *testing.T) {
	releasePush := make(chan struct{})
	pushStarted := make(chan struct{})
	progressDone := make(chan struct{})
	returned := make(chan error, 1)

	go func() {
		returned <- waitForContainerdPush(context.Background(),
			func(ctx context.Context) error {
				close(pushStarted)
				<-releasePush
				return nil
			},
			func(ctx context.Context, doneCh <-chan struct{}) error {
				<-doneCh
				close(progressDone)
				return nil
			},
		)
	}()

	<-pushStarted
	select {
	case err := <-returned:
		t.Fatalf("push wait returned before push completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	close(releasePush)
	select {
	case err := <-returned:
		if err != nil {
			t.Fatalf("expected push wait to succeed, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for push wait to return")
	}

	select {
	case <-progressDone:
	default:
		t.Fatal("expected progress callback to observe push completion")
	}
}

func TestWaitForContainerdPushReturnsPushError(t *testing.T) {
	want := errors.New("push failed")

	err := waitForContainerdPush(context.Background(),
		func(ctx context.Context) error {
			return want
		},
		func(ctx context.Context, doneCh <-chan struct{}) error {
			<-doneCh
			return nil
		},
	)

	if !errors.Is(err, want) {
		t.Fatalf("expected push error %v, got %v", want, err)
	}
}

func TestWaitForContainerdPushReturnsProgressError(t *testing.T) {
	want := errors.New("progress failed")

	err := waitForContainerdPush(context.Background(),
		func(ctx context.Context) error {
			return nil
		},
		func(ctx context.Context, doneCh <-chan struct{}) error {
			return want
		},
	)

	if !errors.Is(err, want) {
		t.Fatalf("expected progress error %v, got %v", want, err)
	}
}

func TestPushWithPlainHTTPFallbackRetriesWithHTTP(t *testing.T) {
	schemes := []string{}

	err := pushWithPlainHTTPFallback(func(defaultScheme string) error {
		schemes = append(schemes, defaultScheme)
		if len(schemes) == 1 {
			return errors.New(`failed to do request: Head "https://172.16.0.231:8081/v2/library/nginx/blobs/sha256:abc": http: server gave HTTP response to HTTPS client`)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if len(schemes) != 2 {
		t.Fatalf("expected two push attempts, got %d", len(schemes))
	}
	if schemes[0] != "" {
		t.Fatalf("expected first push to use default scheme, got %q", schemes[0])
	}
	if schemes[1] != "http" {
		t.Fatalf("expected second push to use http, got %q", schemes[1])
	}
}

func TestPushWithPlainHTTPFallbackDoesNotRetryOtherErrors(t *testing.T) {
	attempts := 0
	want := errors.New("unauthorized")

	err := pushWithPlainHTTPFallback(func(defaultScheme string) error {
		attempts++
		return want
	})

	if !errors.Is(err, want) {
		t.Fatalf("expected original push error %v, got %v", want, err)
	}
	if attempts != 1 {
		t.Fatalf("expected one push attempt, got %d", attempts)
	}
}

func TestBuildImageExportOptsAddsPlatformFilterForManifestLists(t *testing.T) {
	ctx := context.Background()
	currentPlatform := platforms.DefaultSpec()
	blobs := map[digest.Digest][]byte{}
	contentStore := memoryProvider{blobs: blobs}

	layerPayload := []byte("layer-data")
	layerDesc := writeBlob(t, blobs, layerPayload, ocispec.MediaTypeImageLayer)

	configPayload, err := json.Marshal(ocispec.Image{
		RootFS: ocispec.RootFS{
			Type:    "layers",
			DiffIDs: []digest.Digest{layerDesc.Digest},
		},
	})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	configDesc := writeBlob(t, blobs, configPayload, ocispec.MediaTypeImageConfig)

	manifestPayload, err := json.Marshal(ocispec.Manifest{
		Versioned: ocispecs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{layerDesc},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	matchingManifestDesc := writeBlob(t, blobs, manifestPayload, ocispec.MediaTypeImageManifest)
	matchingManifestDesc.Platform = &currentPlatform

	missingPayload := []byte("missing-arm64-manifest")
	otherArchitecture := "amd64"
	if currentPlatform.Architecture == "amd64" {
		otherArchitecture = "arm64"
	}
	missingManifestDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Size:      int64(len(missingPayload)),
		Digest:    digest.FromBytes(missingPayload),
		Platform:  &ocispec.Platform{Architecture: otherArchitecture, OS: currentPlatform.OS},
	}

	indexPayload, err := json.Marshal(ocispec.Index{
		Versioned: ocispecs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: []ocispec.Descriptor{matchingManifestDesc, missingManifestDesc},
	})
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	indexDesc := writeBlob(t, blobs, indexPayload, ocispec.MediaTypeImageIndex)

	imageStore := staticImageStore{
		image: images.Image{
			Name:   "registry.example.com/demo/nginx:alpine",
			Target: indexDesc,
		},
	}

	var withoutPlatform bytes.Buffer
	err = archive.Export(ctx, contentStore, &withoutPlatform,
		archive.WithImage(imageStore, "registry.example.com/demo/nginx:alpine"),
	)
	if err == nil {
		t.Fatal("expected export without platform filter to fail for missing non-matching manifest")
	}

	var withPlatform bytes.Buffer
	err = archive.Export(ctx, contentStore, &withPlatform,
		buildImageExportOpts(imageStore, []string{"registry.example.com/demo/nginx:alpine"})...,
	)
	if err != nil {
		t.Fatalf("expected export with platform filter to succeed, got %v", err)
	}

	if withPlatform.Len() == 0 {
		t.Fatal("expected exported archive to contain data")
	}
}
