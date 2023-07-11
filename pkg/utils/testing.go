package utils

import (
	"fmt"
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
//
//nolint:gomnd
func (fs *FakeFilesystem) Use() func() {
	// create the new fake fs root dir in /tmp/sriov...
	tmpDir, err := os.MkdirTemp("", "k8s-rdma-shared-dev-plugin-")
	if err != nil {
		panic(fmt.Errorf("error creating fake root dir: %s", err.Error()))
	}
	fs.RootDir = tmpDir

	for _, dir := range fs.Dirs {
		osErr := os.MkdirAll(path.Join(fs.RootDir, dir), 0700)
		if osErr != nil {
			panic(fmt.Errorf("error creating fake directory: %s", osErr.Error()))
		}
	}
	for filename, body := range fs.Files {
		ioErr := os.WriteFile(path.Join(fs.RootDir, filename), body, 0600)
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
	err = os.MkdirAll(path.Join(fs.RootDir, "usr/share/hwdata"), 0700)
	if err != nil {
		panic(fmt.Errorf("error creating fake directory: %s", err.Error()))
	}

	// TODO: Remove writing pci.ids file once ghw is mocked
	// This is to fix the CI failure where ghw lib fails to
	// unzip pci.ids file downloaded from internet.
	pciData, err := os.ReadFile("/usr/share/hwdata/pci.ids")
	if err != nil {
		panic(fmt.Errorf("error reading file: %s", err.Error()))
	}
	err = os.WriteFile(path.Join(fs.RootDir, "usr/share/hwdata/pci.ids"), pciData, 0600)
	if err != nil {
		panic(fmt.Errorf("error creating fake file: %s", err.Error()))
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
