#!/bin/sh
set -e

VERSION=${1:-master}
BIN_NAME=ceph-docker-driver
DRIVER_URL="https://github.com/mvollman/ceph-docker-driver/releases/download/${VERSION}"
BIN_DIR="/usr/bin"

osid=$(awk -F'=' '/^ID=/{print $2}' /etc/os-release)
osver=$(awk -F'=' '/^VERSION_ID=/{print $2}' /etc/os-release)
if [[ "$osid" == 'ubuntu' ]] && [[ "$osver" == '"14.04"' ]]; then
        curl -sSL -o /etc/init/${BIN_NAME} ${DRIVER_URL}/${BIN_NAME}.upstart
        chmod 644 /etc/init/${BIN_NAME}
else
        curl -sSL -o /etc/systemd/system/${BIN_NAME}.service ${DRIVER_URL}/${BIN_NAME}.service
        if [[ "$osid" != 'ubuntu' ]] ; then
                sed -i 's%/etc/default/%/etc/sysconfig/%g' /etc/systemd/system/${BIN_NAME}.service
                curl -sSL -o /etc/sysconfig/${BIN_NAME} ${DRIVER_URL}/${BIN_NAME}.env
        else
                curl -sSL -o /etc/default/${BIN_NAME} ${DRIVER_URL}/${BIN_NAME}.env
        fi
        chmod 644 /etc/systemd/system/${BIN_NAME}.service
        systemctl daemon-reload
        systemctl enable $BIN_NAME
fi


