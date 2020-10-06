package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

// FakeFilesystem allows to setup isolated fake files structure used for the tests.
type FakeFilesystem struct {
	RootDir  string
	Dirs     []string
	Files    map[string][]byte
	Symlinks map[string]string
}

// Use function creates entire files structure and returns a function to tear it down. Example usage: defer fs.Use()()
func (fs *FakeFilesystem) Use() func() {
	// create the new fake fs root dir in /tmp/sriov...
	tmpDir, err := ioutil.TempDir("", "k8s-rdma-shared-dev-plugin-")
	if err != nil {
		panic(fmt.Errorf("error creating fake root dir: %s", err.Error()))
	}
	fs.RootDir = tmpDir

	for _, dir := range fs.Dirs {
		osErr := os.MkdirAll(path.Join(fs.RootDir, dir), 0755)
		if osErr != nil {
			panic(fmt.Errorf("error creating fake directory: %s", osErr.Error()))
		}
	}
	for filename, body := range fs.Files {
		ioErr := ioutil.WriteFile(path.Join(fs.RootDir, filename), body, 0644)
		if ioErr != nil {
			panic(fmt.Errorf("error creating fake file: %s", ioErr.Error()))
		}
	}
	for link, target := range fs.Symlinks {
		osErr := os.Symlink(target, path.Join(fs.RootDir, link))
		if osErr != nil {
			panic(fmt.Errorf("error creating fake symlink: %s", osErr.Error()))
		}
	}

	sysNetDevices = path.Join(fs.RootDir, "/sys/class/net")

	return func() {
		// remove temporary fake fs
		err := os.RemoveAll(fs.RootDir)
		if err != nil {
			panic(fmt.Errorf("error tearing down fake filesystem: %s", err.Error()))
		}
	}
}
