# ceph-docker-driver
Simple Docker Volume Driver for Ceph

# Build
```
docker run --rm -v "$PWD":/usr/src/$(basename $PWD) -w /usr/src/$(basename $PWD) -e GOPATH=/usr/src/$(basename $PWD) golang:1.9 go get
docker run --rm -v "$PWD":/usr/src/$(basename $PWD) -w /usr/src/$(basename $PWD) -e GOPATH=/usr/src/$(basename $PWD) golang:1.9 go build
```
# Install

curl -sSl https://raw.githubusercontent.com/mvollman/ceph-docker-driver/master/install.sh | sudo bash -x -s <releasever>

# Run

