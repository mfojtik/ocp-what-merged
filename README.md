# ocp-what-merged

The purpose of this tool is to quickly list all changes that were pushed to OpenShift repositories which makes to the payload.
The reason this is useful is that this allows to spot offending changes that broke the playload or made it more flaky.

### Installation

`go install github.com/mfojtik/ocp-what-merged`

### Usage

Before you use this tool, make sure you provide valid Github token via `GITHUB_TOKEN` variable.

* `ocp-what-merged` - gives you list of changes that were merged to payload in last 24h
* `ocp-what-merged -since 48h` - same, but for last 2 days
* `ocp-what-merged -branch release-4.6` - changes for last 24h but in OpenShift 4.6 branch (z-stream)
* `ocp-what-merged -payload quay.io/openshift-release-dev/ocp-release:custom` - if you for any reason need custom payload (because new repository was added?)

### Example

```bash
$ ocp-what-merged -since 40h
2021/08/24 14:15:37 Processing 121 repositories for commits in master branch, since 40h0m0s ...
  URL (12)                                                                                                    MESSAGE                                                                                WHEN          
 ----------------------------------------------------------------------------------------------------------- -------------------------------------------------------------------------------------- -------------- 
  https://github.com/openshift/console/commit/276e9d485897af3d9ad28236635e94324e03336e                        fetch kamelets form both current namespace and global                                  1 day ago     
                                                                                                              ns where operator isinstal ...                                                                       
  https://github.com/openshift/console/commit/1469b054a1ad1a967121bb8aab720ee28646860b                        show only route resource id sidepanel if route                                         1 day ago     
                                                                                                              existis and show external url if ...                                                               
...
...
...                                                                                                              
```

### License

Apache License 2.0