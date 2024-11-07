#!/bin/bash
set -e 

OLD_MODULE=$(go list -m)
if [ "${OLD_MODULE}" != "github.com/tbxark/sphere" ]; then
  echo "Current module is not github.com/tbxark/sphere, please run this script in the root of Sphere"
  exit 1
fi

read -r -p "Enter new module name: " NEW_MODULE
if [ -z "${NEW_MODULE}" ]; then
  echo "New module name is required"
  exit 1
fi

make init

rm -rf ./pkg
rm -rf ./contrib

echo "Renaming module from ${OLD_MODULE} to ${NEW_MODULE}"
find . -type f -name "*.go" -exec sed -i '' "s,${OLD_MODULE}/cmd,${NEW_MODULE}/cmd,g" {} \;
find . -type f -name "*.go" -exec sed -i '' "s,${OLD_MODULE}/internal,${NEW_MODULE}/internal,g" {} \;
go mod edit -module "${NEW_MODULE}"

echo "Update buf.gen.yaml"
sed -i '' "s,${OLD_MODULE},${NEW_MODULE},g" buf.gen.yaml

go get github.com/tbxark/sphere@latest
go mod tidy

make build
make generate