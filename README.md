![build](https://github.com/uhppoted/uhppoted-app-s3/workflows/build/badge.svg)

# uhppoted-app-s3

```cron```'able command line utility to download access control lists stored on S3 to UHPPOTE UTO311-L0x 
access controller boards. 

Supported operating systems:
- Linux
- MacOS
- Windows
- ARM7

## Releases

| *Version* | *Description*                                                                             |
| --------- | ----------------------------------------------------------------------------------------- |
| v0.8.0    | Maintenance release to update dependencies on `uhppote-core` and `uhppoted-lib`           |
| v0.7.3    | Maintenance release to update dependencies on `uhppote-core` and `uhppoted-lib`           |
| v0.7.2    | Maintenance release to update dependencies on `uhppote-core` and `uhppoted-lib`           |
| v0.7.1    | Maintenance release to update dependencies on `uhppote-core` and `uhppoted-lib`           |
| v0.7.0    | Added support for time profiles from the extended API                                     |
| v0.6.12   | Maintenance release to update dependencies on `uhppote-core` and `uhppoted-api`           |
| v0.6.10   | Maintenance release for version compatibility with `uhppoted-app-wild-apricot`            |
| v0.6.8    | Maintenance release for version compatibility with `uhppote-core` `v.0.6.8`               |
| v0.6.7    | Maintenance release for version compatibility with `uhppoted-api` `v.0.6.7`               |
| v0.6.5    | Maintenance release for version compatibility with `node-red-contrib-uhppoted`            |
| v0.6.4    | Maintenance release for version compatibility with `uhppoted-app-sheets`                  |
| v0.6.3    | Maintenance release to update module dependencies                                         |
| v0.6.2    | Maintenance release to update module dependencies                                         |
| v0.6.1    | Maintenance release to update module dependencies                                         |
| v0.6.0    | Initial release                                                                           |

## Installation

Executables for all the supported operating systems are packaged in the [releases](https://github.com/uhppoted/uhppoted-app-s3/releases). The provided archives contain the executables for all the operating systems - OS specific tarballs can be found in the [uhppoted](https://github.com/uhppoted/uhppoted/releases) releases.

Installation is straightforward - download the archive and extract it to a directory of your choice and then place the executable in a directory in your PATH. The `uhppoted-app-s3` utility requires the following additional 
files:

- `uhppoted.conf`
- `aws.credentials`
- `keys` directory containg the public keys for all user ID's permitted to sign an ACL
- `uhppoted` key file used to (optionally) sign uploaded files

### `uhppoted.conf`

`uhppoted.conf` is the communal configuration file shared by all the `uhppoted` project modules and is (or will 
eventually be) documented in [uhppoted](https://github.com/uhppoted/uhppoted). `uhppoted-app-s3` requires the 
_devices_ section to resolve non-local controller IP addresses and door to controller door identities.

A sample [uhppoted.conf](https://github.com/uhppoted/uhppoted/blob/master/runtime/simulation/405419896.conf) file is included in the `uhppoted` distribution.

### `aws.credentials`

The credentials required to directly access files in AWS S3 buckets are retrieved from an AWS credentials file. The 
file follows the current AWS credentials convention:

```
[default]
aws_access_key_id = AK...
aws_secret_access_key = FR...
```

`uhppoted-app-s3` uses the `[default]` credentials and defaults to the `.aws/credentials` file - use the `-credentials` option to specify an alternative credentials file. Future releases may add a command line option to select alternative credential sets from within the file.

**NOTE:** 

*It is **highly** recommended that a dedicated set of IAM credentials be created for use with `uhppoted-app-s3`,
with a policy that restricts access to only the required S3 buckets and keys.*

### _keys_ directory

The _keys_ directory should contain the RSA public keys of the users that are authorised to provide ACL files. The
public key files should be named:

    <userID>.pub

where `userID` is the user ID included as the `uname` attribute of the ACL file in the tar.gz archive (or corresponding `comment` in a ZIP file). The default _keys_ directory is _<conf dir>/acl/keys_. An alternative directory can be specified
with the `--keys` command line option for the `load` and `compare` commands.

### _key file_

The _key file_ is the RSA private key used by `uhppoted-app-s3` to sign uploaded files (derived ACL's and reports). The default key file is _<conf dir>/acl/keys/uhppoted_. An alternative _key file_ can be specified with the `--keys` command line option for the `store` and `compare` commands.


### Building from source

Assuming you have `Go` and `make` installed:

```
git clone https://github.com/uhppoted/uhppoted-app-s3.git
cd uhppoted-app-s3
make build
```

If you prefer not to use `make`:
```
git clone https://github.com/uhppoted/uhppoted-app-s3.git
cd uhppoted-app-s3
mkdir bin
go build -trimpath -o bin ./...
```

The above commands build the `'uhppoted-app-s3` executable to the `bin` directory.

#### Dependencies

| *Dependency*                                                                 | *Description*                              |
| ---------------------------------------------------------------------------- | ------------------------------------------ |
| [com.github/uhppoted/uhppote-core](https://github.com/uhppoted/uhppote-core) | Device level API implementation            |
| [com.github/uhppoted-lib](https://github.com/uhppoted/uhppoted-lib)          | Shared application library                 |
| [com.github/aws/aws-sdk-go](https://github.com/aws/aw-sdk-go)                | AWS API Go library                         |
[ golang.org/x/sys                                                             | AWS API library dependency                 |
| golang.org/x/lint/golint                                                     | Additional *lint* check for release builds |

## uhppoted-app-s3

Usage: ```uhppoted-app-s3 <command> <options>```

Supported commands:

- `help`
- `version`
- `load-acl`
- `store-acl`
- `compare-acl`

### ACL file format

The only currently supported ACL file format is TSV (tab separated values) and is expected to be formatted as follows:

    Card Number	From	To	Workshop	Side Door	Front Door	Garage	Upstairs	Downstairs	Tower	Cellar
    123465537	2020-01-01	2020-12-31	N	N	Y	N	Y	N	Y	Y
    231465538	2020-01-01	2020-12-31	Y	N	Y	N	N	Y	29	N
    635465539	2020-01-01	2020-12-31	N	N	N	N	Y	N	Y	Y

| Field         | Description                                                                |
|---------------|----------------------------------------------------------------------------|
| `Card Number` | Access card number                                                         |
| `From`        | Date from which card is valid (_valid from 00:00 on that date_)            |
| `To`          | Date until which card is valid (_valid until 23:59 on that date_)          |
| `<door>`      | Door name matching controller configuration (_case and space insensitive_) |
| `<door>`      | ...                                                                        |
| ...           |                                                                            |

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

and should match a public key file in the _keys_ directory for the `uhppoted-app-s3` utility. For `.zip` archives, the user ID should be included as a comment to the ACL file entry:
```
zip -c myacl.zip myacl.acl signature
```

A sample [tar.gz](https://github.com/uhppoted/uhppoted/blob/master/runtime/simulation/405419896.tar.gz) file is included in the full `uhppoted` distribution.

Command line:

```uhppoted-app-s3 load-acl --url <url>```

```uhppoted-app-s3 load-acl [--debug]  [--no-log] [--no-report] [--no-verify] [--config <file>] [--workdir <dir>] [--keys <dir>] [--credentials <file>] [--region <region>] --url <url>```

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

Fetches the cards stored in the configured UHPPOTE controllers, creates a matching ACL file from the UHPPOTED controller configuration and uploads it to an AWS S3 bucket (or other URL). Intended for use in a `cron` task that routinely audits the cards stored on the controllers against an authoritative source. The ACL file is a `.tar.gz` or `.zip` archive and contains the following two files:
- `uhppoted.acl` 
- `signature`

The `signature` file is the RSA signature of the ACL file - it can be verified using the `openssl` command:
```
openssl dgst -sha256 -verify <uhppoted public key file> -signature signature <ACL file> 
```

Command line:

```uhppoted-app-s3 store-acl --url <url>```

```uhppoted-app-s3 store-acl [--debug]  [--no-log] [--no-sign] [--config <file>] [--key <RSA signing key>] [--credentials <file>] [--region <region>] --url <url>```

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
  --no-sign     Does not sign the generated ACL file with the uhppoted RSA signing key
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

and should match a public key file in the _keys_ directory for the `uhppoted-app-s3` utility. For `.zip` archives, the user ID should be included as a comment to the ACL file entry:
```
zip -c myacl.zip myacl.acl signature
```

Command line:

```uhppoted-app-s3 compare-acl --acl <url> --report <url>```

```uhppoted-app-s3 compare-acl [--debug]  [--no-log] [--no-verify] [--config <file>] [--keys <dir>] [--key <file>] [--credentials <file>] [--region <region>] --acl <url> --report <url>```

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
