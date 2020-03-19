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
| [com.github/uhppoted/uhppote-core][https://github.com/uhppoted/uhppote-core] | Device level API implementation            |
| [com.github/uhppoted/uhppoted-api][https://github.com/uhppoted/uhppoted-api] | common API for external applications       |
| golang.org/x/lint/golint                                                     | Additional *lint* check for release builds |

## uhppoted-acl-s3

Usage: ```uhppoted-acl-s3``` [--debug] \<command\> \<arguments\>```

Supported commands:

- help
- version
- load-acl
- store-acl

