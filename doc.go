// Copyright 2023 uhppoted@twyst.co.za. All rights reserved.
// Use of this source code is governed by an MIT-style license
// that can be found in the LICENSE file.

/*
Package uhppoted-app-s3 integrates the uhppote-core API with access control lists stored as files on Amazon S3.

uhppoted-app-s3 can be used from the command line but is really intended to be run from a cron job to maintain
the cards and permissions on a set of access controllers from a unified access control list (ACL) file. Despite
the name, ACL files can also be read from the local disk or downloaded from an HTTP URL.

uhppoted-app-s3 supports the following commands:

  - load-acl, to download an ACL from a file to a set of access controllers
  - store-acl, to retrieve the ACL from a set of controllers and save it as a file
  - compare-acl, to compare an ACL from a file with the cards and permissons on a set of access controllers

*/
package s3
