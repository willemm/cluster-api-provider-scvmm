package controllers

import (
	"io"
)

func writePlain(fh io.WriterAt, handler CloudInitFilesystemHandler, files []CloudInitFile) error {
	_, err := handler(fh, files, 0)
	return err
}

func init() {
	DeviceHandlers[""] = writePlain
	DeviceHandlers["dvd"] = DeviceHandlers[""]
	DeviceHandlers["floppy"] = DeviceHandlers[""]
}
