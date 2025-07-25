# ea

Access NTFS(new technology file system) Extended Attributes(EA) with golang.

This package provides functions for writing and querying Extended Attributes for files in NTFS which can be shown by using "fsutil file queryea [file_path]" in cmd in Windows.

_The maximum size available for EA entries for each file is 65535(0xffff) bytes, trying to write larger data then maximum size would result in failing with STATUS_EA_TOO_LARGE or truncated data(https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fsa/be0bb27a-4954-4786-80a6-947df0e82a11)._

_example(cp949)_

```
C:\test>fsutil file queryea test.txt

Extended Attributes (EA) information for file C:\test\test.txt:

Total Ea Size: 0xbd

Ea Buffer Offset: 0
Ea Name: TEST
Ea Value Length: 16
0000:  74 68 69 73 20 69 73 20  61 6e 20 45 41 20 63 6f  this is an EA co
0010:  6e 74 65 6e 74 2e                                 ntent.

Ea Buffer Offset: 24
Ea Name: テスト
Ea Value Length: 21
0000:  45 41 20 77 72 69 74 65  20 74 65 73 74 2e 20 e3  EA write test. .
0010:  83 86 e3 82 b9 e3 83 88  20 6a 61 70 61 6e 65 73  ........ japanes
0020:  65                                                e

Ea Buffer Offset: 54
Ea Name: 테스트
Ea Value Length: 1f
0000:  45 41 20 77 72 69 74 65  20 74 65 73 74 2e 20 ed  EA write test. .
0010:  85 8c ec 8a a4 ed 8a b8  20 6b 6f 72 65 61 6e     ........ korean

Ea Buffer Offset: 84
Ea Name: 試驗
Ea Value Length: 2c
0000:  45 41 20 77 72 69 74 65  20 74 65 73 74 2e 20 e8  EA write test. .
0010:  a9 a6 e9 a9 97 20 43 4a  4b 20 55 6e 69 66 69 65  ..... CJK Unifie
0020:  64 20 49 64 65 6f 67 72  61 70 68 73              d Ideographs
```

## Writing EA

For writing EA into a file, EaWriteFile or WriteEaWithFile can be used to add EA.

_write EA with byte slice_

```go
import (
	"github.com/go-sw/ntfs/ea"
)

func main() {
	eaVal := "this is a value for ea"
	targetPath := "C:\\test\\test.txt"

	fileEa := ea.EaInfo{
		Flags: 0,
		EaName: "eaFromBytes",
		EaValues: []byte(eaVal),
	}

	err := ea.EaWriteFile(targetPath, false, fileEa)
	if err != nil {
		panic(err)
	}
}
```

_write EA with file content_

```go
import (
	"github.com/go-sw/ntfs/ea"
)

func main() {
	targetPath := "C:\\test\\test.txt"
	sourcePath := "C:\\test\\source.txt"
	flags := 0

	err := ea.WriteEaWithFile(targetPath, false, sourcePath, flags, "eaFromFile")
	if err != nil {
		panic(err)
	}
}
```

## Querying EA

For querying EA within the file. QueryEaFile can be used with target file path.

_querying ea from file and writing it to console_

```go
import (
	"encoding/hex"
	"fmt"

	"github.com/go-sw/ntfs/ea"
)

func main() {
	targetPath := "C:\\test\\test.txt"

	eaList, err := ea.QueryEaFile(targetPath, false)
	if err != nil {
		panic(err)
	}

	for _, e := range eaList {
		fmt.Printf("Flags: 0x%x\nEa Name: %s\nEa Value\n:%s\n", e.Flags, e.EaName, hex.Dump(e.EaValue))
	}
}
```
