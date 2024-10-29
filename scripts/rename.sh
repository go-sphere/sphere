#!/bin/bash
set -e 

OLD_MODULE=$(go list -m)
read -r -p "Enter new module name: " NEW_MODULE
read -r -p "Do you want to keep pkg module name as ${OLD_MODULE} (y/n)? " KEEP_PKG_NAME

echo "Renaming module from ${OLD_MODULE} to ${NEW_MODULE}"

go mod edit -module "${NEW_MODULE}"
find . -type f -name '*.go' -exec sed -i -e "s,${OLD_MODULE},${NEW_MODULE},g" {} \;
find . -type f -name '*.go' -exec sed -i -e "s,${OLD_MODULE}/cmd,${NEW_MODULE}/cmd,g" {} \;
find . -type f -name '*.go' -exec sed -i -e "s,${OLD_MODULE}/internal,${NEW_MODULE}/internal,g" {} \;
if [ "${KEEP_PKG_NAME}" == "n" ]; then
  find . -type f -name '*.go' -exec sed -i -e "s,${OLD_MODULE}/pkg,${NEW_MODULE}/pkg,g" {} \;
else
  rm -rf pkg
fi

echo "Update buf.gem.yaml"
sed -i -e "s,${OLD_MODULE},${NEW_MODULE},g" buf.gem.yaml
