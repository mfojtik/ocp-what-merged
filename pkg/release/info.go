package release

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema2"
	dockerarchive "github.com/docker/docker/pkg/archive"
	imagereference "github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/library-go/pkg/image/registryclient"
)

type Options struct {
	repository distribution.Repository
	registry   *registryclient.Context
}

func New(ctx context.Context) (*Options, error) {
	repoURL, err := url.Parse("https://quay.io")
	if err != nil {
		return nil, err
	}
	registry := registryclient.NewContext(http.DefaultTransport, http.DefaultTransport)
	repository, err := registry.Repository(ctx, repoURL, "openshift-release-dev/ocp-release", true)
	if err != nil {
		return nil, err
	}
	return &Options{
		repository: repository,
		registry:   registry,
	}, nil
}

func (o *Options) ListReleaseTags(ctx context.Context) ([]string, error) {
	tags, err := o.repository.Tags(ctx).All(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i] < tags[j]
	})
	return tags, nil
}

func (o *Options) GetReleaseTagReference(ctx context.Context, tagName string) (*imagereference.DockerImageReference, error) {
	tag, err := o.repository.Tags(ctx).Get(ctx, tagName)
	if err != nil {
		return nil, err
	}
	return &imagereference.DockerImageReference{
		Registry:  "quay.io",
		Namespace: "openshift-release-dev",
		Name:      "ocp-release",
		Tag:       tagName,
		ID:        tag.Digest.String(),
	}, nil
}

func (o *Options) GetLatestReleaseTag(ctx context.Context) (*imagereference.DockerImageReference, error) {
	tags, err := o.ListReleaseTags(ctx)
	if err != nil {
		return nil, err
	}
	return o.GetReleaseTagReference(ctx, tags[len(tags)-1])
}

func (o *Options) GetPayloadRepositories(ctx context.Context, ref imagereference.DockerImageReference) ([]string, error) {
	srcManifest, _, err := firstManifest(ctx, ref, o.repository, (&FilterOptions{DefaultOSFilter: true}).Include)
	if err != nil {
		return nil, fmt.Errorf("unable to find manifest in %s: %v", ref, err)
	}
	manifest, ok := srcManifest.(*schema2.DeserializedManifest)
	if !ok {
		return nil, fmt.Errorf("manifest is not schema v2 manifest")
	}

	imageReferences := bytes.NewBuffer([]byte{})

	for _, l := range manifest.Layers {
		//  TODO: hack: skip layers that are large for release manifests to save time
		if l.Size > 10000000 {
			continue
		}
		r, err := o.repository.Blobs(ctx).Open(ctx, l.Digest)
		if err != nil {
			return nil, err
		}
		rc, err := dockerarchive.DecompressStream(r)
		if err != nil {
			return nil, err
		}
		tr := tar.NewReader(rc)
		for {
			hdr, err := tr.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			if !strings.Contains(hdr.Name, "release-manifests/image-references") {
				continue
			}
			if _, err := io.Copy(imageReferences, tr); err != nil {
				return nil, err
			}
		}
		if err := rc.Close(); err != nil {
			return nil, err
		}
		if err := r.Close(); err != nil {
			return nil, err
		}
	}

	var stream ImageStream
	if err := json.Unmarshal(imageReferences.Bytes(), &stream); err != nil {
		return nil, err
	}

	var result []string
	for _, t := range stream.Spec.Tags {
		if len(t.Annotations) == 0 {
			continue
		}
		found := false
		source, ok := t.Annotations["io.openshift.build.source-location"]
		if !ok || len(source) == 0 {
			continue
		}
		for _, r := range result {
			if r == source {
				found = true
				break
			}
		}
		if !found {
			result = append(result, source)
		}
	}
	sort.Strings(result)
	return result, nil
}
