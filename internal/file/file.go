package file

import (
	"io"
	"os"
)

// Exists returns a bool indicating if the specified file exists or not. It
// returns any errors that are encountered that are NOT an os.ErrNotExist error.
func Exists(filename string) (bool, error) {
	_, err := os.Stat(filename)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// CopyFile executes a simple and straightforward copy of the specified source
// FILE to the specified destination path. This does not account for various
// complexities. This will not copy directories. The destination must be a full
// path to the new file location. It cannot be just a directory path. There is
// no accounting for the possibility that a file already exists at the specified
// destination. There is no accounting for the possibility that the directories
// along the path to the destination don't already exist. In short, use this
// function only when your intention is very simply to copy a single file from
// one location on the file system to another and you're certain that the
// current state of the file system is favorable for that. Any error that is
// encountered will be returned.
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}
