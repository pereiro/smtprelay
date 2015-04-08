#!/bin/sh
APPNAME=smtprelay
TMP=tmp
CWD=$(pwd)
GOROOT=/usr/local/go
BUILDLOG=buildlog.txt
OLDGOPATH=$GOPATH
TMPGOPATH=$CWD/$TMP
SYSTEM='unknown'
PM="unknown"

unamestr=`uname`

if [[ "$unamestr" == 'Linux' ]]; then
   SYSTEM='linux'
elif [[ "$unamestr" == 'FreeBSD' ]]; then
   SYSTEM='freebsd'
fi

if [[ $SYSTEM == "freebsd" ]]; then 

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
echo "Installing golang DKIM package"
go get "github.com/eaigner/dkim"
echo "Installing golang UUID package"
go get "github.com/twinj/uuid"
echo "Installing golang LOG4GO package"
go get "code.google.com/p/log4go"
echo "Installing golang REDIS package"
go get "gopkg.in/redis.v2"
mkdir $CWD/build/$SYSTEM/bin
go build -o $CWD/build/$SYSTEM/bin/$APPNAME -i
export GOPATH=$OLDGOPATH
rm -rf $TMPGOPATH
cd $CWD/build/$SYSTEM
./install.sh

elif [[ $SYSTEM == "linux" ]]; then

declare -A osInfo;
osInfo[/etc/redhat-release]=yum install 
osInfo[/etc/gentoo-release]=emerge
osInfo[/etc/SuSE-release]=zypper install 
osInfo[/etc/debian_version]=apt-get install

for f in ${!osInfo[@]}
do
    if [[ -f $f ]];then
         PM=${osInfo[$f]}
    fi
done

echo "Installing GOLANG"
$PM go
echo "Patching GOLANG"
cp build/patches/dnsclient.go $GOROOT/src/net/
cd $GOROOT/src/
./make.bash >> $CWD/$BUILDLOG
cd $CWD
export GOPATH=$TMPGOPATH
$PM mercurial
rm -rf $TMPGOPATH
mkdir $TMPGOPATH
mkdir $TMPGOPATH/src
mkdir $TMPGOPATH/src/$APPNAME
cp -r redismq $TMPGOPATH/src/$APPNAME/
cp -r smtp $TMPGOPATH/src/$APPNAME/
cp -r smtpd $TMPGOPATH/src/$APPNAME/
echo "Installing golang DKIM package"
go get "github.com/eaigner/dkim"
echo "Installing golang UUID package"
go get "github.com/twinj/uuid"
echo "Installing golang LOG4GO package"
go get "code.google.com/p/log4go"
echo "Installing golang REDIS package"
go get "gopkg.in/redis.v2"
#mkdir $CWD/build/$SYSTEM/bin
go build -o $CWD/build/$SYSTEM/bin/$APPNAME -i
export GOPATH=$OLDGOPATH
rm -rf $TMPGOPATH
cd $CWD/build/$SYSTEM
#./install.sh

fi