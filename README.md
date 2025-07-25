# winfs

Windows filesystem library with golang, currently supports only Windows.

Licensed under the [BSD-1-Clause](https://opensource.org/license/bsd-1-clause) license, which does not require attribution for binary distribution.

## Components
The directories in this repository include the following components.

- [ads](ads): [Alternate Data Stream](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/e2b19412-a925-4360-b009-86e3b8a020c8) wrapper
- [backup](backup): [MS-BKUP](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-bkup/f67950c8-d583-469a-83dd-c4ff4cedf533) wrapper
- [ea](ea): [Extended Attributes](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fsa/be0bb27a-4954-4786-80a6-947df0e82a11) wrapper
- [efs](efs): [Encrypted File System](https://learn.microsoft.com/en-us/windows/win32/fileio/file-encryption) wrapper
- [file](file): file operation utilities for copying or moving file with callbacks, etc
