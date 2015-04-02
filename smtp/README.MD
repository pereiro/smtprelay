Fork of golang net/smtp library. The only reason to use it instead of standard version is:
        //TLS INSECURE FIX
		config := &tls.Config{
            ServerName: c.serverName,
            InsecureSkipVerify:true}

It allows to communicate SMTP servers with self-signed TLS certificates.



