set -ex

BIN_NAME=backend
BACKEND_BIN=./build/linux_x86/$BIN_NAME
SERVICE_NAME=backend
HOST="root@127.0.0.1"

if [ ! -f "$BACKEND_BIN" ]; then
  echo "Backend binary not found: $BACKEND_BIN"
  exit 1
fi

scp "$BACKEND_BIN" $HOST:~/"$BIN_NAME"
ssh $HOST "
  set -e && \
  systemctl stop $SERVICE_NAME.service && \
  mv ~/$BIN_NAME /usr/local/bin/$BIN_NAME && \
  systemctl start $SERVICE_NAME.service && \
  /usr/local/bin/$BIN_NAME version && \
  systemctl status $SERVICE_NAME.service
"