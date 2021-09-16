package release

import (
	"context"
	"strings"
	"testing"
)

func TestRelease(t *testing.T) {
	o, err := New(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	latest, err := o.GetLatestReleaseTag(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("latest: %+v", latest.Tag)
	sources, err := o.GetPayloadRepositories(context.TODO(), *latest)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("sources: %s", strings.Join(sources, "\n"))
}
