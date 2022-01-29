# End-To-End (E2E) Tests for Platform-Ref-AWS

This plugin is meant as a demo for writing conformance tests of crossplane packages.

## How to use this plugin

- Clone this repo
- Modify the build script to specify your registry/image/tag
- Write tests (using main_test.go as a jumping off point)
- Run ./build.sh to build the image and push it to your registry
- `sonobuoy run -p plugin.yaml` to run your own plugin
