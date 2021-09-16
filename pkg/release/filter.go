package release

import (
	"fmt"
	"regexp"

	"github.com/docker/distribution/manifest/manifestlist"
)

// FilterOptions assist in filtering out unneeded manifests from ManifestList objects.
type FilterOptions struct {
	FilterByOS      string
	DefaultOSFilter bool
	OSFilter        *regexp.Regexp
}

// Validate checks whether the flags are ready for use.
func (o *FilterOptions) Validate() error {
	pattern := o.FilterByOS
	if len(pattern) > 0 {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("--filter-by-os was not a valid regular expression: %v", err)
		}
		o.OSFilter = re
	}
	return nil
}

// IsWildcardFilter returns true if the filter regex is set to a wildcard
func (o *FilterOptions) IsWildcardFilter() bool {
	wildcardFilter := ".*"
	if o.FilterByOS == wildcardFilter {
		return true
	}
	return false
}

// Include returns true if the provided manifest should be included, or the first image if the user didn't alter the
// default selection and there is only one image.
func (o *FilterOptions) Include(d *manifestlist.ManifestDescriptor, hasMultiple bool) bool {
	if o.OSFilter == nil {
		return true
	}
	if o.DefaultOSFilter && !hasMultiple {
		return true
	}
	s := PlatformSpecString(d.Platform)
	return o.OSFilter.MatchString(s)
}

func PlatformSpecString(platform manifestlist.PlatformSpec) string {
	if len(platform.Variant) > 0 {
		return fmt.Sprintf("%s/%s/%s", platform.OS, platform.Architecture, platform.Variant)
	}
	return fmt.Sprintf("%s/%s", platform.OS, platform.Architecture)
}

// IncludeAll returns true if the provided manifest matches the filter, or all if there was no filter.
func (o *FilterOptions) IncludeAll(d *manifestlist.ManifestDescriptor, hasMultiple bool) bool {
	if o.OSFilter == nil {
		return true
	}
	s := PlatformSpecString(d.Platform)
	return o.OSFilter.MatchString(s)
}
