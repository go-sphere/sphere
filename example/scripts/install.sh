#!/usr/bin/env sh

set -u

type curl > /dev/null || { echo "curl: not found"; exit 1; }

set -e


bin="sphere"
repo="TBXark/sphere"
dest_dir="/usr/local/bin"

get_latest_release() {
  local repo="$1"
	curl -sSL "https://api.github.com/repos/${repo}/releases/latest" | \
    awk 'BEGIN{FS=": |,|\""}; /tag_name/{print $5}'
}

version="$(get_latest_release "${repo}")"  # v1.2.0

# if args has version override it and not eq "latest"
if test $# -eq 1; then
  if test "$1" != "latest"; then
    version="$1"

    echo "Install ${version}"
  fi
fi

platform="$(uname | tr "[A-Z]" "[a-z]")"  # Linux => linux
arch="$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')"  # x86_64 => amd64, aarch64 => arm64
package="${bin}_${platform}_${arch}.tar.gz"
package_url="https://github.com/${repo}/releases/download/${version}/${package}"
bin_path="${dest_dir}/${bin}"
tmp_dir="$(mktemp -d)"

trap "rm -r ${tmp_dir}" EXIT

if test -e "${bin_path}"; then
  current_version="v$("${bin_path}" -v | awk '{print $NF}')"
  if test "${current_version}" = "${version}"; then
    echo "${bin} is already updated, no need to upgrade."
    exit 0
  else
    echo "There is a new version of ${bin}, starting to upgrade from ${current_version} to ${version}."
  fi
fi
cd "${tmp_dir}"
curl -sSL "${package_url}" | tar xzf -

if test $(id -u) -eq 0; then
  mv "${bin}" "${dest_dir}"
else
  sudo mv "${bin}" "${dest_dir}"
fi

mkdir -p ~/.${bin}

echo "${bin} ${version} has been installed."
