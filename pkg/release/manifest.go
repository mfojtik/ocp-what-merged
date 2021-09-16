package release

import (
	"context"
	"fmt"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	_ "github.com/docker/distribution/manifest/schema2"
	digest "github.com/opencontainers/go-digest"
	imagereference "github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/library-go/pkg/image/registryclient"
	"k8s.io/klog/v2"
)

type manifestLocation struct {
	Manifest     digest.Digest
	ManifestList digest.Digest
}

func (m manifestLocation) IsList() bool {
	return len(m.ManifestList) > 0
}

func (m manifestLocation) String() string {
	if m.IsList() {
		return fmt.Sprintf("manifest %s in manifest list %s", m.Manifest, m.ManifestList)
	}
	return fmt.Sprintf("manifest %s", m.Manifest)
}

type filterFunc func(*manifestlist.ManifestDescriptor, bool) bool

// firstManifest returns the first manifest at the request location that matches the filter function.
func firstManifest(ctx context.Context, from imagereference.DockerImageReference, repo distribution.Repository, filterFn filterFunc) (distribution.Manifest, manifestLocation, error) {
	manifests, err := repo.Manifests(ctx)
	if err != nil {
		return nil, manifestLocation{}, err
	}
	srcManifest, err := manifests.Get(ctx, digest.Digest(from.ID), distribution.WithTag(from.Tag))
	if err != nil {
		return nil, manifestLocation{}, fmt.Errorf("get %q failed: %v", "", err)
	}

	originalSrcDigest := srcManifest.References()[0].Digest
	srcManifests, srcManifest, srcDigest, err := processManifestList(ctx, originalSrcDigest, srcManifest, manifests, from, filterFn, false)
	if err != nil {
		return nil, manifestLocation{}, err
	}
	if len(srcManifests) == 0 {
		return nil, manifestLocation{}, fmt.Errorf("filtered all images from manifest list")
	}

	if srcDigest != originalSrcDigest {
		return srcManifest, manifestLocation{Manifest: srcDigest, ManifestList: originalSrcDigest}, nil
	}
	return srcManifest, manifestLocation{Manifest: srcDigest}, nil
}

func processManifestList(ctx context.Context, srcDigest digest.Digest, srcManifest distribution.Manifest, manifests distribution.ManifestService, ref imagereference.DockerImageReference, filterFn filterFunc, keepManifestList bool) ([]distribution.Manifest, distribution.Manifest, digest.Digest, error) {
	var srcManifests []distribution.Manifest
	switch t := srcManifest.(type) {
	case *manifestlist.DeserializedManifestList:
		manifestDigest := srcDigest
		manifestList := t

		filtered := make([]manifestlist.ManifestDescriptor, 0, len(t.Manifests))
		for _, manifest := range t.Manifests {
			if !filterFn(&manifest, len(t.Manifests) > 1) {
				klog.V(5).Infof("Skipping image %s for %#v from %s", manifest.Digest, manifest.Platform, ref)
				continue
			}
			klog.V(5).Infof("Including image %s for %#v from %s", manifest.Digest, manifest.Platform, ref)
			filtered = append(filtered, manifest)
		}

		if len(filtered) == 0 {
			return nil, nil, "", nil
		}

		// if we're filtering the manifest list, update the source manifest and digest
		if len(filtered) != len(t.Manifests) {
			var err error
			t, err = manifestlist.FromDescriptors(filtered)
			if err != nil {
				return nil, nil, "", fmt.Errorf("unable to filter source image %s manifest list: %v", ref, err)
			}
			_, body, err := t.Payload()
			if err != nil {
				return nil, nil, "", fmt.Errorf("unable to filter source image %s manifest list (bad payload): %v", ref, err)
			}
			manifestList = t
			manifestDigest, err := registryclient.ContentDigestForManifest(t, srcDigest.Algorithm())
			if err != nil {
				return nil, nil, "", err
			}
			klog.V(5).Infof("Filtered manifest list to new digest %s:\n%s", manifestDigest, body)
		}

		for i, manifest := range t.Manifests {
			childManifest, err := manifests.Get(ctx, manifest.Digest, distribution.WithManifestMediaTypes([]string{manifestlist.MediaTypeManifestList, "application/vnd.docker.distribution.manifest.v2+json"}))
			if err != nil {
				return nil, nil, "", fmt.Errorf("unable to retrieve source image %s manifest #%d from manifest list: %v", ref, i+1, err)
			}
			srcManifests = append(srcManifests, childManifest)
		}

		switch {
		case len(srcManifests) == 1 && !keepManifestList:
			manifestDigest, err := registryclient.ContentDigestForManifest(srcManifests[0], srcDigest.Algorithm())
			if err != nil {
				return nil, nil, "", err
			}
			klog.Warningf("Chose %s/%s manifest from the manifest list.", t.Manifests[0].Platform.OS, t.Manifests[0].Platform.Architecture)
			return srcManifests, srcManifests[0], manifestDigest, nil
		default:
			return append(srcManifests, manifestList), manifestList, manifestDigest, nil
		}

	default:
		return []distribution.Manifest{srcManifest}, srcManifest, srcDigest, nil
	}
}
