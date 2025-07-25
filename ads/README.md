# ads
Access NTFS(New Technology File System) ADS(Alternate Data Stream) using golang.

This package provides access data streams in NTFS for files and directories with names(a.k.a Alternate Data Stream) which can be accessed by appending ":\[stream name\]" after file name.
This also appiles to directories and reparse points, which is normally not available with cmd with commonly known methods. Also, extracting data from alternative stream is a bit complicated with cmd.

## Query ADS from file
_Query name and size of ADS from file_
```go
import (
	"fmt"

	"github.com/go-sw/ntfs/ads"
)

func main() {
	targetPath := "test.txt"

	fileAds, err := ads.GetFileADS(targetPath)
	if err != nil {
		panic(err)
	}

	for name, size := range fileAds.StreamInfoMap {
		fmt.Printf("name: %s, size: %d\n", name, size)
	}
}
```

## Write, remove, rename ADS from file
```go
import (
	"fmt"
	"os"

	"github.com/go-sw/ntfs/ads"
)

func main() {
	targetPath := "test.txt"

	ads1, err := ads.OpenFileADS(targetPath, "ads1", os.O_CREATE|os_O_WRONLY)
	ads1.Write([]byte("test ads 1"))
	ads1.Close()
	ads2, err := ads.OpenFileADS(targetPath, "ads2", os.O_CREATE|os_O_WRONLY)
	ads2.Write([]byte("test ads 2"))
	ads2.Close()
	ads3, err := ads.OpenFileADS(targetPath, "ads3", os.O_CREATE|os_O_WRONLY)
	ads3.Write([]byte("test ads 3"))
	ads3.Close()
	ads4, err := ads.OpenFileADS(targetPath, "ads4", os.O_CREATE|os_O_WRONLY)
	ads4.Write([]byte("test ads 4"))
	ads4.Close()

	// create ADS handler for file
	fileAds, err := ads.GetFileADS(targetPath)
	if err != nil {
		panic(err)
	}

	// rename ADS "ads1" to "renamed1"
	err = fileAds.RenameADS("ads1", "renamed1", true)
	if err != nil {
		panic(err)
	}

	// remove ADS "ads2"
	err = fileAds.RemoveADS("ads2")
	if err != nil {
		panic(err)
	}

	// remove all ADS from "test.txt"
	err = fileAds.RemoveAllADS()
	if err != nil {
		panic(err)
	}
}
```
