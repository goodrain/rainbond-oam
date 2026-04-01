package image

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/platforms"
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
