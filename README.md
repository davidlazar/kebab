# Kebab

Kebab is a backup tool that prioritizes confidentiality, integrity, and
availability above all else.  Kebab creates full backups of your files that
are compressed and encrypted and then stored on Amazon S3 or in another
directory.  To reduce the likelihood of bugs, Kebab is a small amount of
code, forgoing features like deduplication and incremental backups.

Kebab serves as an alternative to Tarsnap, offering the following advantages:

* **Tiny codebase.**
Kebab is less than 2000 lines of code.  This makes Kebab easier to audit
which reduces the likelihood of bugs.

* **Modern cryptography.**
Kebab uses NaCl's [secretbox](http://nacl.cr.yp.to/secretbox.html)
primitive to encrypt and authenticate your backups.  It also uses
scrypt to hash your keyfile passphrase.

* **Key availability.**
The availability of your backups is limited by the availability of your
decryption key.  Kebab keys are encrypted with a passphrase and easy to
write down, making it painless to create physical backups of your key.

* **Fast restoration.**
Kebab creates full backups of your files. Compared to other backup
strategies, full backups are slow to create but fast and simple to restore.

* **Bucket control.**
Kebab stores data on Amazon S3 using your own account, so you have full
control of the underlying bucket.  This allows you to store your backups
on Amazon Glacier to reduce storage costs.  Kebab also supports storing
backups in a directory.

* **Free software.**
Kebab is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

### Limitations

Kebab is not space efficient since every backup is a full backup.
To reduce complexity and code size, Kebab does not provide features like
deduplication or incremental backups.  However, Kebab does compress
your data using tar/gzip.

Kebab is still useful if you do not have a lot of data, or you have a lot
of bandwidth, or you organize your data so that it is easy to backup only
what changes (for example, a maildir with a separate directory for each
month of mail).

Another limitation of Kebab is that it calls out to tar and gzip, which are
relatively large pieces of software.  In the future, Kebab may switch to
using the `archive/tar` and `compress/gzip` packages instead.

## Getting Started

1. Create an Amazon S3 bucket to store your backups and a JSON file that
identifies this bucket:

    $ cat s3bucket.json
    {
        "Service": {
            "Region":"us-east-1",
            "AccessKey":"XYZ...",
            "AccessKeyId":"ABC..."
        },
        "Bucket":"kebab_482731..."
    }

2. Install Kebab:

    $ export GOPATH=...
    $ go install github.com/davidlazar/kebab

3. Run tests:

    $ go test -v -s3 s3bucket.json github.com/davidlazar/kebab/...
    $ go test -v github.com/davidlazar/go-crypto/...

4. Create a key file with a strong passphrase that you can memorize:

    $ kebab -keygen -key kebab.key
    Passphrase: ...

5. With a pen and paper, make physical copies of your key file and store
them somewhere safe:

    $ cat kebab.key
    wab77 b8fxk waqkz
    q0j9e 8jxqx vcc94
    64bb5 egb1d rpggb
    dbg4v 86ygw f4fzg

6. Create some backups:

    $ kebab -bucket s3bucket.json -key kebab.key -put email-$(date "+%Y-%m-%d") email \
    -putfrom docs-$(date "+%Y-%m-%d") ~/sensitive docs

7. Restore backups:

    $ kebab -bucket s3bucket.json -key kebab.key -get email-2015-03-14 -get ...


#### Copyright
Kebab Copyright (C) 2015 David Lazar
