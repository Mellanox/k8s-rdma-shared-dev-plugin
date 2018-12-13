package sriovnet

import (
	"fmt"
	"os"
	"path/filepath"
	"log"
)

const (
	NetSysDir        = "/sys/class/net"
	pcidevPrefix     = "device"
	netdevDriverDir  = "device/driver"
	netdevUnbindFile = "unbind"
	netdevBindFile   = "bind"

	netDevMaxVfCountFile     = "sriov_totalvfs"
	netDevCurrentVfCountFile = "sriov_numvfs"
	netDevVfDevicePrefix     = "virtfn"
)

type VfObject struct {
	NetdevName string
	PCIDevName string
}

func netDevDeviceDir(netDevName string) string {
	devDirName := NetSysDir + "/" + netDevName + "/" + pcidevPrefix
	return devDirName
}

func getMaxVfCount(pfNetdevName string) (int, error) {
	devDirName := netDevDeviceDir(pfNetdevName)

	maxDevFile := fileObject{
		Path: devDirName + "/" + netDevMaxVfCountFile,
	}

	maxVfs, err := maxDevFile.ReadInt()
	if err != nil {
		return 0, err
	} else {
		log.Println("max_vfs = ", maxVfs)
		return maxVfs, nil
	}
}

func setMaxVfCount(pfNetdevName string, maxVfs int) error {
	devDirName := netDevDeviceDir(pfNetdevName)

	maxDevFile := fileObject{
		Path: devDirName + "/" + netDevCurrentVfCountFile,
	}

	return maxDevFile.WriteInt(maxVfs)
}

func getCurrentVfCount(pfNetdevName string) (int, error) {
	devDirName := netDevDeviceDir(pfNetdevName)

	maxDevFile := fileObject{
		Path: devDirName + "/" + netDevCurrentVfCountFile,
	}

	curVfs, err := maxDevFile.ReadInt()
	if err != nil {
		return 0, err
	} else {
		log.Println("cur_vfs = ", curVfs)
		return curVfs, nil
	}
}

func vfNetdevNameFromParent(pfNetdevName string, vfIndex int) string {

	devDirName := netDevDeviceDir(pfNetdevName)
	vfNetdev, _ := lsFilesWithPrefix(fmt.Sprintf("%s/%s%v/net", devDirName,
		netDevVfDevicePrefix, vfIndex), "", false)
	if len(vfNetdev) <= 0 {
		return ""
	} else {
		return vfNetdev[0]
	}
}

func vfPCIDevNameFromVfIndex(pfNetdevName string, vfIndex int) (string, error) {
	link := filepath.Join(NetSysDir, pfNetdevName, pcidevPrefix, fmt.Sprintf("%s%v",
		netDevVfDevicePrefix, vfIndex))
	pciDevDir, err := os.Readlink(link)
	if len(pciDevDir) <= 3 {
		return "", fmt.Errorf("could not find PCI Address for VF %s%v of PF %s",
			netDevVfDevicePrefix, vfIndex, pfNetdevName)
	}

	return pciDevDir[3:len(pciDevDir)], err
}

func GetVfPciDevList(pfNetdevName string) ([]string, error) {
	var vfDirList []string
	var i int
	devDirName := netDevDeviceDir(pfNetdevName)

	virtFnDirs, err := lsFilesWithPrefix(devDirName, netDevVfDevicePrefix, true)

	if err != nil {
		return nil, err
	}

	i = 0
	for _, vfDir := range virtFnDirs {
		vfDirList = append(vfDirList, vfDir)
		i++
	}
	return vfDirList, nil
}

func findVfDirForNetdev(pfNetdevName string, vfNetdevName string) (string, error) {

	virtFnDirs, err := GetVfPciDevList(pfNetdevName)
	if err != nil {
		return "", err
	}

	ndevSearchName := vfNetdevName + "__"

	for _, vfDir := range virtFnDirs {

		vfNetdevPath := filepath.Join(NetSysDir, pfNetdevName,
			pcidevPrefix, vfDir, "net")
		vfNetdevList, err := lsDirs(vfNetdevPath)
		if err != nil {
			return "", err
		}
		for _, vfName := range vfNetdevList {
			vfNamePrefixed := vfName + "__"
			if ndevSearchName == vfNamePrefixed {
				return vfDir, nil
			}
		}
	}
	return "", fmt.Errorf("device %s not found", vfNetdevName)
}
