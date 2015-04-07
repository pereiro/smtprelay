#!/bin/sh
CONFPATH=/usr/local/etc/smtprelay
BINPATH=/usr/sbin
BINNAME=smtprelay
LOGPATH=/var/log
LOFILE=relay.log
RCDIR=/etc/rc.d
RCCONF=/etc/rc.conf


echo "Installing REDIS"
pkg install redis >> log.txt
echo "Redis installed"
echo "Creating folders and copying files"
mkdir -p $CONFPATH
mkdir -p $CONFPATH/dkim_keys
touch $LOGPATH/$LOGFILE
cp -i conf/config.json $CONFPATH/config.json
cp -i conf/logconfig.xml $CONFPATH/logconfig.xml
cp -i bin/$BINNAME $BINPATH
echo "Adding to rc.d"
cp rc/relayd $RCPATH
if [ $(grep -c "smtprelay_enable" $RCCONF) -eq 0 ]; then
    echo "smtprelay_enable=YES" >> $RCCONF
fi
echo "Done. Use service start|stop to control smtprelay. Config files are at /usr/local/etc/relay"


