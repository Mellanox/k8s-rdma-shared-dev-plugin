package resources

import "github.com/vishvananda/netlink"

type netlinkManager struct {
}

func (nLink *netlinkManager) LinkByName(ifName string) (netlink.Link, error) {
	return netlink.LinkByName(ifName)
}
