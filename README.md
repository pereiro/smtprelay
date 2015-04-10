# SMTPRELAY
*SMTPRELAY* is simple and fast smtp server for mass mailing purposes, written in GO
## Features
* **Using REDIS as MQ backend**
    REDIS is in-memory DB and it is very fast. On other side we have risk of data loss in case of power off 
    or redis crash. For minimizing risks, highly recommended to use Redis background saving feature.
* **Built-in DKIM signing**
    Most of mail providers require DKIM signature header in your messages. Smtprelay supports DKIM out of box.   
* **Direct and relay mode.** 
    In direct mode smtprelay resolves DNS MX records for each recipient and sends message to mailserver, 
    gained from MX. In relay mode it redirects all messages to relay server, specified in configuration file.
    
