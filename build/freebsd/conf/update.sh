#!/bin/sh
cd /usr/local/etc/smtprelay
rm -f smtprelay.tar.gz > /dev/null
rm -r -f tmp > /dev/null
wget https://github.com/Pereiro/smtprelay/releases/download/last/smtprelay.tar.gz --no-check-certificate
mkdir tmp
tar -xzf smtprelay.tar.gz -C tmp/
rm -f smtprelay.tar.gz > /dev/null
service smtprelay stop
cp -f tmp/freebsd/bin/smtprelay /usr/local/sbin/
rm -r -f tmp
sleep 1
service smtprelay start
service smtprelay status
