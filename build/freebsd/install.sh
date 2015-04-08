#!/bin/sh
CONFPATH=/usr/local/etc/smtprelay
BINPATH=/usr/local/sbin
BINNAME=smtprelay
LOGPATH=/var/log
LOFILE=smtprelay.log
RCDIR=/etc/rc.d
RCCONF=/etc/rc.conf


echo "Installing REDIS"
pkg install redis > installlog.txt
echo "Redis installed"
echo "Creating folders and copying files"
mkdir -p $CONFPATH
mkdir -p $CONFPATH/dkim_keys
touch $LOGPATH/$LOGFILE
cp -i conf/config.json $CONFPATH/config.json
cp -i conf/logconfig.xml $CONFPATH/logconfig.xml
cp -i bin/$BINNAME $BINPATH/$BINNAME
echo "Adding to rc.d"
cp rc/$BINNAME $RCPATH/$BINNAME
if [ $(grep -c "smtprelay_enable" $RCCONF) -eq 0 ]; then
    echo "smtprelay_enable=YES" >> $RCCONF
fi
echo "Done. Use service start|stop to control smtprelay. Config files are at /usr/local/etc/smtprelay"


