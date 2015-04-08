#!/bin/sh
TMP=tmp
APPNAME=smtprelay
CWD=$(pwd)
GOROOT=/usr/local/go
BUILDLOG=buildlog.txt
OLDGOPATH=$GOPATH
TMPGOPATH=$CWD/$TMP
SYSTEM=freebsd


echo "Installing GOLANG"
pkg install go
echo "Patching GOLANG"
cp build/patches/dnsclient.go $GOROOT/src/net/
cd $GOROOT/src/
./make.bash >> $CWD/$BUILDLOG
cd $CWD
export GOPATH=$TMPGOPATH
pkg install mercurial
rm -rf $TMPGOPATH
mkdir $TMPGOPATH
mkdir $TMPGOPATH/src
mkdir $TMPGOPATH/src/$APPNAME
cp -r redismq $TMPGOPATH/src/$APPNAME/
cp -r smtp $TMPGOPATH/src/$APPNAME/
cp -r smtpd $TMPGOPATH/src/$APPNAME/
go get "github.com/eaigner/dkim"
go get "github.com/twinj/uuid"
go get "code.google.com/p/log4go"
go get "gopkg.in/redis.v2"
mkdir $CWD/build/$SYSTEM/bin
go build -o $CWD/build/$SYSTEM/bin/$APPNAME -i
export GOPATH=$OLDGOPATH
rm -rf $TMPGOPATH
cd $CWD/build/$SYSTEM
./install.sh