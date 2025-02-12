#!/bin/bash
set -e 

OLD_MODULE=$(go list -m)
if [ "${OLD_MODULE}" != "github.com/TBXark/sphere" ]; then
  echo "Current module is not github.com/TBXark/sphere, please run this script in the root of Sphere"
  exit 1
fi

read -r -p "Enter new module name: " NEW_MODULE
if [ -z "${NEW_MODULE}" ]; then
  echo "New module name is required"
  exit 1
fi

make init

rm -rf ./api
rm -rf ./pkg
rm -rf ./contrib
rm -rf ./swagger

mkdir -p assets/dash/dashboard/apps/web-ele/dist
touch assets/dash/dashboard/apps/web-ele/dist/index.html

echo "Renaming module from ${OLD_MODULE} to ${NEW_MODULE}"
go install github.com/TBXark/sphere/contrib/import-renamer@latest
go mod edit -module "${NEW_MODULE}"
import-renamer -old "${OLD_MODULE}/cmd" -new "${NEW_MODULE}/cmd" -target .
import-renamer -old "${OLD_MODULE}/api" -new "${NEW_MODULE}/api" -target .
import-renamer -old "${OLD_MODULE}/swagger" -new "${NEW_MODULE}/swagger" -target .
import-renamer -old "${OLD_MODULE}/internal" -new "${NEW_MODULE}/internal" -target .

echo "Update buf.gen.yaml"
sed -i '' "s,${OLD_MODULE}/api,${NEW_MODULE}/api,g" buf.gen.yaml

go mod edit -dropreplace all
go get github.com/TBXark/sphere@latest
go mod tidy

make build
make generate