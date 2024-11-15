package main

import (
	"os"

	"github.com/willemm/cluster-api-provider-scvmm/internal/controllers"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	fh, err := os.Create("test.vhdx")
	defer fh.Close()
	check(err)
	handler, _ := controllers.FilesystemHandlers["vfat"]
	devHandler, _ := controllers.DeviceHandlers["scsi"]
	files := []controllers.CloudInitFile{
		{
			Filename: "test.txt",
			Contents: []byte("Contents of a small text file\n"),
		},
		{
			Filename: "test2.txt",
			Contents: []byte("Contents of another small text file\n"),
		},
	}
	err = devHandler(fh, handler, files)
	check(err)
}
