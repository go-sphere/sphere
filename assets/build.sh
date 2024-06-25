set -ex
set -o pipefail

dash_repo="git@github.com:pure-admin/pure-admin-thin.git"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
cd $script_dir

if [ ! -d "dashboard/.git" ]; then
  git clone $dash_repo dashboard
  cd dashboard
else
  cd dashboard
  git pull
fi

pnpm install
pnpm run build