# uhppoted-acl-s3

```cron```'able command line utility to fetch access control lists stored on S3 and download to 
UHPPOTE UTO311-L0x access controller boards. 

Supported operating systems:
- Linux
- MacOS
- Windows

## Releases

- *In development*

## Installation

### Building from source

#### Dependencies

| *Dependency*                                                                 | *Description*                              |
| ---------------------------------------------------------------------------- | ------------------------------------------ |
| [com.github/uhppoted/uhppote-core](https://github.com/uhppoted/uhppote-core) | Device level API implementation            |
| [com.github/uhppoted/uhppoted-api](https://github.com/uhppoted/uhppoted-api) | common API for external applications       |
| golang.org/x/lint/golint                                                     | Additional *lint* check for release builds |

## uhppoted-acl-s3

Usage: ```uhppoted-acl-s3 command <options>```

Supported commands:

- `help`
- `version`
- `load-acl`
- `store-acl`
- `compare-acl`

### ACL file format

The only currently supported ACL file format is TSV (tab separated values) and is expected to be formatted as follows:

    Card Number	From	To	Workshop	Side Door	Front Door	Garage
    123465537	2020-01-01	2020-12-31	N	N	Y	N
    231465538	2020-01-01	2020-12-31	Y	N	Y	N
    635465539	2020-01-01	2020-12-31	N	N	N	N

| Field               | Description                                                                               |
|--------------------|----------------------------------------------------------------------------------|
| Card Number | Access card number                                                               |
| From              | Date from which card is valid (_valid from 00:00 on that date_) |
| To                   | Date until which card is valid (_valid until 23:59 on that date_)   |
| _Door_           | Door name matching controller configuration                         |
| _Door_           | Door name matching controller configuration                         |
| ...                    | Door name matching controller configuration                         |

The ACL file must include a column for each controller + door configured in the _devices_ section of the `uhppoted.conf` file used to configure the utility.

An [example ACL file](https://github.com/uhppoted/uhppoted/blob/master/runtime/simulation/405419896.acl) is included in the full `uhppoted` distribution, along with the matching [_conf_](https://github.com/uhppoted/uhppoted/blob/master/runtime/simulation/405419896.conf) file.

### `load-acl`

Fetches an ACL file from S3 (or other URL) and downloads it to the configured UHPPOTE controllers. Intended for use in a `cron` task that routinely updates the controllers from an authoritative source that exports the access control list as a TSV file. The ACL file is expected to be a `.tar.gz` or `.zip` archive and should include the following two files:
- `<file>.acl` 
- `signature`

The `signature` file is the RSA signature of the ACL file - it can be created using the `openssl` command:
```
openssl dgst -sha256 -sign <key file> <ACL file> signature
```
The user ID used to sign the ACL file should be included in the command to create the `.tar.gz` file:
```
tar cvzf acl.tar.gz --uname <user id> --gname <uhppoted> myacl.acl signature
```

and should match a public key file in the _keys_ directory for the `uhppoted-acl-s3` utility. For `.zip` archives, the user ID should be included as a comment to the ACL file entry:
```
zip -c myacl.zip myacl.acl signature
```

A sample [.tar.gz](https://github.com/uhppoted/uhppoted/blob/master/runtime/simulation/405419896.tar.gz) file is included in the full `uhppoted` distribution.

Short form:
```uhppoted-acl-s3 load-acl --url <url>```

Full command line:
```uhppoted-acl-s3 load-acl [--debug]  [--no-log] [--no-report] [--no-verify] [--config <file>] [--workdir <dir>] [--keys <dir>] [--credentials <file>] [--region <region>] --url <url>```

```
  --url         URL from which to fetch the ACL files. A URL starting with s3:// specifies 
                that the file should be fetched from an AWS S3 bucket using S3 operations
                and AWS credentials (files stored in AWS S3 buckets can also be retrieved
                using pre-signed https:// URL's). URL's with the file:// protocol can be used to specify local files. The file is expected to be a .tar.gz or .zip archive containing an ACL and signature file (defaults to .tar.gz unless the URL ends with .zip)

  --credentials AWS credentials file (described below) for fetching files from s3:// URL's
  --region      AWS S3 region (e.g. us-east-1) for use with the AWS credentials
  --keys        Directory containing the public keys for RSA keys used to sign the ACL's
  --config      Sets the uhppoted.conf file to use for controller configurations
  --workdir     Sets the working directory for generated report files
  --no-log      Writes log messages to the console rather than the rotating log file
  --no-report   Prints the load-acl operational report to the console rather than creating a report file
  --no-verify   Disables verification of the ACL file signature
  --debug       Displays verbose debugging information, in particular the communications with the UHPPOTE controllers
```

### `store-acl`

Fetches the cards stored in the configured UHPPOTE controllers, creates a matching ACL file from the UHPPOTED controller configuration and uploads it to an AWS S3 bucket (or other URL). Intended for use in a `cron` task that routinely audits the cards stored on the controllers against an authoritative source. The ACL file is a `.tar.gz` or `.zip' archive and contains the following two files:
- `uhppoted.acl` 
- `signature`

The `signature` file is the RSA signature of the ACL file - it can be verified using the `openssl` command:
```
openssl dgst -sha256 -verify <uhppoted public key file> -signature signature <ACL file> 
```

A sample [.tar.gz](https://github.com/uhppoted/uhppoted/blob/master/runtime/simulation/405419896.tar.gz) file is included in the full `uhppoted` distribution.

Short form:
```uhppoted-acl-s3 store-acl --url <url>```

Full command line:
```uhppoted-acl-s3 store-acl [--debug]  [--no-log] [--config <file>] [--key <RSA signing key>] [--credentials <file>] [--region <region>] --url <url>```

```
  --url         URL to which to store the ACL file. A URL starting with s3:// specifies 
                that the file should be stored in an AWS S3 bucket using S3 operations
                and AWS credentials (files stored in AWS S3 buckets can also be uploaded
                using a pre-signed https:// URL). URL's with the file:// protocol can be                 used to specify local files. The created file is a .tar.gz (or .zip)
                archive containing an ACL and signature file (defaults to .tar.gz unless
                the URL ends with .zip)
  
  --credentials AWS credentials file (described below) for fetching files from s3:// URL's
  --region      AWS S3 region (e.g. us-east-1) for use with the AWS credentials
  --key         File containing the private RSA key used to sign the ACL
  --config      Sets the uhppoted.conf file to use for controller configurations
  --no-log      Writes log messages to the console rather than the rotating log file
  --debug       Displays verbose debugging information, in particular the communications with the UHPPOTE controllers
```

### `compare-acl`

Fetches an ACL file from S3 (or other URL) and compares it to the cards stored in the configured UHPPOTE controllers. Intended for use in a `cron` task that routinely audits the controllers against an authoritative source that exports the access control list as a TSV file. The ACL file follows the structure outlined above for the `load-acl` command i.e. should be a `.tar.gz` or `.zip` archive and should include the following two files:
- `<file>.acl` 
- `signature`

The `signature` file is the RSA signature of the ACL file - it can be created using the `openssl` command:
```
openssl dgst -sha256 -sign <key file> <ACL file> signature
```
The user ID used to sign the ACL file should be included in the command to create the `.tar.gz` file:
```
tar cvzf acl.tar.gz --uname <user id> --gname <uhppoted> myacl.acl signature
```

and should match a public key file in the _keys_ directory for the `uhppoted-acl-s3` utility. For `.zip` archives, the user ID should be included as a comment to the ACL file entry:
```
zip -c myacl.zip myacl.acl signature
```

Short form:
```uhppoted-acl-s3 compare-acl --acl <url> --report <url>```

Full command line:
```uhppoted-acl-s3 compare-acl [--debug]  [--no-log] [--no-verify] [--config <file>] [--keys <dir>] [--key <file>] [--credentials <file>] [--region <region>] --acl <url> --report <url>```

```
  --acl         URL from which to fetch the ACL files. A URL starting with s3:// specifies 
                that the file should be fetched from an AWS S3 bucket using S3 operations
                and AWS credentials (files stored in AWS S3 buckets can also be retrieved
                using pre-signed https:// URL's). URL's with the file:// protocol can be used to specify local files. The file is expected to be a .tar.gz or .zip archive containing an ACL and signature file (defaults to .tar.gz unless the URL ends with .zip)
  
  --report      URL to which to store the compare report file. A URL starting with s3:// specifies 
                that the file should be stored in an AWS S3 bucket using S3 operations
                and AWS credentials (files stored in AWS S3 buckets can also be uploaded
                using a pre-signed https:// URL). URL's with the file:// protocol can be used to specify local files. The created file is a .tar.gz (or .zip) archive containing an ACL and signature file (defaults to .tar.gz unless the URL ends with .zip)
  
  --credentials AWS credentials file (described below) for fetching files from s3:// URL's
  --region      AWS S3 region (e.g. us-east-1) for use with the AWS credentials
  --keys        Directory containing the public keys for RSA keys used to sign the ACL's
  --key         File containing the private RSA key used to sign the report
  --config      Sets the uhppoted.conf file to use for controller configurations
  --no-verify   Disables verification of the ACL file signature
  --no-log      Writes log messages to the console rather than the rotating log file
  --debug       Displays verbose debugging information, in particular the communications with the UHPPOTE controllers
```
