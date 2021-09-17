# ocp-what-merged

## Overview

The purpose of this tool is to help OCP engineers list changes that were merged into OCP payloads in given time windows.
This helps to determine what changes potentially broke the CI system.
There are existing production tools that will do better job than this tool, like `oc adm release` that has flags to list
commits in the payload.

This tool, however, uses the GitHub API to get the commits and does not require cloning all the release repositories locally.

## Install

`go install github.com/mfojtik/ocp-what-merged/cmd/ocp-what-merged`

Alternatively, you can clone this repo and just run `make` to get the binary build yourself.

## Usage

> :warning: Make sure you provide valid [GitHub token](https://github.com/settings/tokens) using `GITHUB_TOKEN` environment variable, because we will make a lot of API requests.

* `ocp-what-merged` - gives you list of changes that were merged to the latest payload in last 24h
* `ocp-what-merged -since 48h` - same, but for last 2 days
* `ocp-what-merged -branch release-4.8` - changes for last 24h in the 4.8.z release branch (but using repository list from latest OCP)
* `ocp-what-merged -tag 4.8.9-x86_64 -branch release-4.6` - same as above, but use the repository list for 4.8.9 release
* `ocp-what-merged -tags -tag 4.8` - will get you list of all available 4.8.z tags to use, so you don't have to list them via quay.io

### Example

```console
$ ocp-what-merged -branch release-4.8 -release 4.8.9-x86_64 -since 8d
  URL (5)                                                              MESSAGE                                                                  WHEN        
 -------------------------------------------------------------------- ------------------------------------------------------------------------ ------------ 
  https://github.com/openshift/machine-config-operator/commit/db21bb   Activate the default interface explicitly in configure-ovs.sh            1 week ago  
                                                                       In some SDN migration case, the default interface cannot be activated                
                                                                       automatically by 'nmcli con reload'. We activate it explicitly.                      
                                                                       Also delete bridge br-ex and br-int created if exist, when running CNI               
                                                                       is openshift-sdn.                                                                    
                                                                       (cherry picked from commit 2e12d3e458e25375d111e654a387d1892294048a)                 
  https://github.com/openshift/insights-operator/commit/ce9605         [release-4.8] Bug TBD: Gather installed PSP names                        1 week ago  
                                                                       (#489) (#490)                                                                        
  https://github.com/openshift/kubernetes/commit/4c16c3                Merge tag 'v1.21.4' into HEAD                                            6 days ago  
                                                                       Kubernetes official release v1.21.4                                                  
  https://github.com/openshift/kubernetes/commit/872ead                UPSTREAM: <drop>: manually resolve conflicts                             6 days ago  
  https://github.com/openshift/kubernetes/commit/df3de0                UPSTREAM: <drop>: hack/update-vendor.sh, make update and                 6 days ago  
                                                                       update image    
```

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

[owners]: https://git.k8s.io/community/contributors/guide/owners.md
[Creative Commons 4.0]: https://git.k8s.io/website/LICENSE