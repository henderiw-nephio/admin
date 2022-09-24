# admin

## Description
admin controller

## Usage

### Fetch the package
`kpt pkg get REPO_URI[.git]/PKG_PATH[@VERSION] admin`
Details: https://kpt.dev/reference/cli/pkg/get/

### View package content
`kpt pkg tree admin`
Details: https://kpt.dev/reference/cli/pkg/tree/

### Apply the package
```
kpt live init admin
kpt live apply admin --reconcile-timeout=2m --output=table
```
Details: https://kpt.dev/reference/cli/live/
