#!/bin/sh
TMP=tmp
CWD=$(pwd)
GOROOT=/usr/local/go
BUILDLOG=buildlog.txt
OLDGOPATH=$GOPATH
TMPGOPATH=$CWD/$TMP
SYSTEM=freebsd


echo "Installing GOLANG"
pkg install go-1.4.2,1 >> $BUILDLOG
echo "Patching GOLANG"
cp build/patches/dnsclient.go $GOROOT/src/net/
cd $GOROOT/src/
./make.bash >> $CWD/$BUILDLOG
cd $CWD
export GOPATH=$TMPGOPATH
go build -o $CWD/build/$SYSTEM/bin -i
export GOPATH=$OLDGOPATH
rm -rf $TMPGOPATH
cd $CWD/build/$SYSTEM
./install.sh