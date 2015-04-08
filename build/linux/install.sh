#!/bin/sh
CONFPATH=/etc/smtprelay
BINPATH=/usr/bin
BINNAME=smtprelay
LOGPATH=/var/log
LOFILE=smtprelay.log
RCPATH=/etc/init.d

declare -A osInfo;
osInfo[/etc/redhat-release]="yum install redis"
osInfo[/etc/gentoo-release]="emerge dev-db/redis"
osInfo[/etc/SuSE-release]="zypp install redis"
osInfo[/etc/debian_version]="apt-get install redis"

for f in ${!osInfo[@]}
do
    if [[ -f $f ]];then
        PM=${osInfo[$f]}
    fi
done

echo "Installing REDIS"
$PM
echo "Redis installed"
echo "Creating folders and copying files"
mkdir -p $CONFPATH
mkdir -p $CONFPATH/dkim_keys
touch $LOGPATH/$LOGFILE
cp -i conf/config.json $CONFPATH/config.json
cp -i conf/logconfig.xml $CONFPATH/logconfig.xml
cp -i bin/$BINNAME $BINPATH/$BINNAME
echo "Adding to init.d"
cp -i init.d/$BINNAME $RCPATH/$BINNAME
#if [ $(grep -c "smtprelay_enable" $RCCONF) -eq 0 ]; then
#    echo "smtprelay_enable=YES" >> $RCCONF
#fi
echo "Done. Use service start|stop to control smtprelay. Config files are at $CONFPATH/$BINNAME"


