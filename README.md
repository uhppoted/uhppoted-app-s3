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
| From              | Date from which card is valid (valid from 00:00 on that date) |
| To                   | Date until which card is valid (valid until 23:59 on that date)   |
| _Door_           | Door name matching controller configuration                         |
| _Door_           | Door name matching controller configuration                         |
| ...                    | Door name matching controller configuration                         |

The ACL file must include a column for each controller + door configured in the _devices_ section of the `uhppoted.conf` file used to configure the utility.

An [example ACL file](https://github.com/uhppoted/uhppoted/blob/master/runtime/simulation/405419896.acl) is included in the full `uhppoted` distribution, along with the matching [_conf_](https://github.com/uhppoted/uhppoted/blob/master/runtime/simulation/405419896.conf) file.

### `load-acl`

Fetches an ACL file from S3 (or other URL) and downloads it to the configured UHPPOTE controllers. Intended for use in a `cron` task that routinely updates the controllers from an authoritative source that exports the access control list as a TSV file. The ACL file is expected to be compressed as a `.tar.gz` file and should include the following two files:
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

and should match a public key file in the _keys_ directory for the `uhppoted-acl-s3` utility.


A sample [.tar.gz](https://github.com/uhppoted/uhppoted/blob/master/runtime/simulation/405419896.tar.gz) file is included in the full `uhppoted` distribution.

Short form:
```uhppoted-acl-s3 load-acl --url <url>```

Full command line:
```uhppoted-acl-s3 load-acl [--debug] [--no-report] [--no-verify] [--workdir <dir>] [--keys <dir>] [--credentials <file>] [--config <file>] --url <url>```

### `store-acl`

### `compare-acl`
